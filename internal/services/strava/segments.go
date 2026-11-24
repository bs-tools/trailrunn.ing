package strava

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/bs-tools/trailrunn.ing/internal/activities"
	"github.com/bs-tools/trailrunn.ing/internal/blob"
	"github.com/bs-tools/trailrunn.ing/internal/errreport"
	"github.com/bs-tools/trailrunn.ing/internal/geo"
	"github.com/bs-tools/trailrunn.ing/internal/store"
	"github.com/bs-tools/trailrunn.ing/internal/strava"
	"github.com/jackc/pgx/v5/pgtype"
	"golang.org/x/sync/singleflight"
)

const (
	// SegmentMaxAge is how long a segment is cached. Strava's API policy
	// (section 6.2) allows caching its data for at most seven days; the
	// bucket's lifecycle rule deletes segments after six, as Cloud Storage
	// applies it up to a day late, and older ones are fetched again.
	SegmentMaxAge = 6 * 24 * time.Hour
	// recentSegments is how many segments are kept in memory, so ones
	// looked at again, e.g. downloaded after being previewed, aren't read
	// from the bucket. A segment's path is tens of kilobytes.
	recentSegments = 16
)

var (
	// ErrSegmentNotFound is returned when Strava has no segment with an ID
	// that the user can see.
	ErrSegmentNotFound = errors.New("strava segment not found")
	// ErrNotConnected is returned when the user hasn't connected Strava,
	// whose API needs a user's token for every request.
	ErrNotConnected = errors.New("strava is not connected")
	// ErrInvalidSegment is returned for a segment that is neither a link to
	// a segment nor an ID.
	ErrInvalidSegment = errors.New("not a Strava segment link or ID")
)

// SegmentClient gets segments from Strava.
type SegmentClient interface {
	Refresh(ctx context.Context, refreshToken string) (*strava.Token, error)
	GetSegment(ctx context.Context, accessToken string, id int64) (*strava.Segment, error)
	GetSegmentStreams(ctx context.Context, accessToken string, id int64) (*strava.SegmentStreams, error)
}

// SegmentConnections gets and refreshes users' Strava tokens.
type SegmentConnections interface {
	GetStravaConnection(ctx context.Context, userID pgtype.UUID) (*activities.StravaConnection, error)
	RefreshStravaTokens(ctx context.Context, userID pgtype.UUID, refresh func(stored *activities.StravaConnection) (*activities.StravaTokens, error)) (*activities.StravaConnection, error)
}

// Segment is a Strava segment, as cached.
type Segment struct {
	ID                  int64   `json:"id"`
	Name                string  `json:"name"`
	ActivityType        string  `json:"activity_type"`
	DistanceMeters      float64 `json:"distance_meters"`
	ElevationGainMeters float64 `json:"elevation_gain_meters"`
	AverageGradePercent float64 `json:"average_grade_percent"`
	City                string  `json:"city"`
	State               string  `json:"state"`
	Country             string  `json:"country"`
	Private             bool    `json:"private"`
	Hazardous           bool    `json:"hazardous"`
	// When the segment was fetched from Strava.
	FetchTime time.Time `json:"fetch_time"`
	// The path's coordinates, as latitude and longitude.
	LatLng [][2]float64 `json:"latlng"`
	// The elevation at each coordinate in meters, or nil if Strava has none.
	Altitude []float64 `json:"altitude,omitempty"`
}

// Path returns the segment's path, with elevations if it has them.
func (s *Segment) Path() []geo.Point {
	points := make([]geo.Point, len(s.LatLng))
	for i, ll := range s.LatLng {
		ele := math.NaN()
		if i < len(s.Altitude) {
			ele = s.Altitude[i]
		}
		points[i] = geo.Point{Lat: ll[0], Lng: ll[1], Ele: ele}
	}
	return points
}

// Location returns where the segment is, e.g. "Squamish, BC, Canada".
func (s *Segment) Location() string {
	var parts []string
	for _, part := range []string{s.City, s.State, s.Country} {
		if part = strings.TrimSpace(part); part != "" {
			parts = append(parts, part)
		}
	}
	return strings.Join(parts, ", ")
}

// Segments gets Strava segments with the Strava token of the user asking,
// and caches them: a segment's path is the same for everyone, and costs two
// of the app's Strava requests to fetch, which are shared with syncing
// activities. Private segments are only fetched, never cached, so they're
// never shown to anyone but their owner.
type Segments struct {
	log    *slog.Logger
	report *errreport.Reporter
	conns  SegmentConnections
	client SegmentClient
	cache  blob.Store
	// The URL of Strava's website, for links to segments.
	webURL string
	nowFn  func() time.Time

	// Fetches of the same segment for the same user at the same time
	// share one request to Strava.
	group singleflight.Group

	mu     sync.Mutex
	recent map[int64]*Segment
}

// NewSegments creates Segments that caches segments in the store. webURL
// is the URL of Strava's website, usually strava.WebURL, for links to
// segments.
func NewSegments(log *slog.Logger, report *errreport.Reporter, conns SegmentConnections, client SegmentClient, cache blob.Store, webURL string) *Segments {
	return &Segments{
		log:    log,
		report: report,
		conns:  conns,
		client: client,
		cache:  cache,
		webURL: webURL,
		nowFn:  time.Now,
		recent: map[int64]*Segment{},
	}
}

// segmentIDRe matches the ID in a link to a segment, e.g.
// https://www.strava.com/segments/40535472, or a bare ID.
var segmentIDRe = regexp.MustCompile(`^(?:(?:https?://)?(?:www\.)?strava\.com/segments/)?(\d{1,19})(?:[/?#].*)?$`)

// ParseSegmentID returns the ID of a segment from a link to it or its ID.
func ParseSegmentID(segment string) (int64, error) {
	m := segmentIDRe.FindStringSubmatch(strings.TrimSpace(segment))
	if m == nil {
		return 0, ErrInvalidSegment
	}
	id, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil || id <= 0 {
		return 0, ErrInvalidSegment
	}
	return id, nil
}

// Get returns a segment, from the cache if it was fetched less than
// SegmentMaxAge ago, or else from Strava with the user's token. Only users
// who connected Strava get segments, even cached ones, as Strava's API
// terms only let the app show its data to the athletes connected to it.
func (s *Segments) Get(ctx context.Context, userID pgtype.UUID, id int64) (*Segment, error) {
	conn, err := s.conns.GetStravaConnection(ctx, userID)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrNotConnected
	}
	if err != nil {
		return nil, err
	}
	if seg := s.fresh(s.recentSegment(id)); seg != nil {
		return seg, nil
	}
	key := fmt.Sprintf("%x/%d", userID.Bytes, id)
	v, err, _ := s.group.Do(key, func() (any, error) {
		if seg := s.fresh(s.cached(ctx, id)); seg != nil {
			s.remember(seg)
			return seg, nil
		}
		return s.fetch(ctx, conn, id)
	})
	if err != nil {
		return nil, err
	}
	return v.(*Segment), nil
}

// fresh returns the segment if it's younger than SegmentMaxAge, else nil.
func (s *Segments) fresh(seg *Segment) *Segment {
	if seg == nil || s.nowFn().Sub(seg.FetchTime) >= SegmentMaxAge {
		return nil
	}
	return seg
}

func (s *Segments) recentSegment(id int64) *Segment {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.recent[id]
}

// remember keeps a segment in memory, forgetting the one fetched longest
// ago if there are too many.
func (s *Segments) remember(seg *Segment) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.recent[seg.ID]; !ok && len(s.recent) >= recentSegments {
		var oldest *Segment
		for _, other := range s.recent {
			if oldest == nil || other.FetchTime.Before(oldest.FetchTime) {
				oldest = other
			}
		}
		delete(s.recent, oldest.ID)
	}
	s.recent[seg.ID] = seg
}

// forget removes a segment from memory and the cache, as Strava's API
// policy requires once Strava no longer offers it.
func (s *Segments) forget(ctx context.Context, id int64) {
	s.mu.Lock()
	delete(s.recent, id)
	s.mu.Unlock()
	if err := s.cache.Delete(ctx, segmentKey(id)); err != nil {
		s.report.Report(ctx, "Failed to delete a cached Strava segment", err)
	}
}

func segmentKey(id int64) string {
	return fmt.Sprintf("segments/%d.json.gz", id)
}

// cached returns the cached segment, or nil if it isn't cached or can't be
// read, in which case it's fetched again.
func (s *Segments) cached(ctx context.Context, id int64) *Segment {
	data, err := s.cache.Get(ctx, segmentKey(id))
	if errors.Is(err, blob.ErrNotFound) {
		return nil
	}
	if err != nil {
		s.report.Report(ctx, "Failed to read a cached Strava segment", err)
		return nil
	}
	seg, err := decodeSegment(data)
	if err != nil {
		s.report.Report(ctx, "Failed to decode a cached Strava segment", err)
		return nil
	}
	return seg
}

// fetch gets a segment from Strava with the user's token, and caches it
// unless it's private.
func (s *Segments) fetch(ctx context.Context, conn *activities.StravaConnection, id int64) (*Segment, error) {
	// The user is waiting for the segment, so it may use the requests kept
	// in reserve from syncing.
	ctx = strava.WithPriority(ctx, strava.PriorityInteractive)
	token, err := s.token(ctx, conn)
	if err != nil {
		return nil, err
	}
	detail, err := s.client.GetSegment(ctx, token, id)
	if err != nil {
		return nil, s.fetchError(ctx, id, err)
	}
	streams, err := s.client.GetSegmentStreams(ctx, token, id)
	if err != nil {
		return nil, s.fetchError(ctx, id, err)
	}
	if len(streams.LatLng) < 2 {
		return nil, fmt.Errorf("%w: Strava has no path for segment %d", ErrSegmentNotFound, id)
	}
	seg := &Segment{
		ID:                  id,
		Name:                strings.TrimSpace(detail.Name),
		ActivityType:        detail.ActivityType,
		DistanceMeters:      detail.Distance,
		ElevationGainMeters: detail.TotalElevationGain,
		AverageGradePercent: detail.AverageGrade,
		City:                detail.City,
		State:               detail.State,
		Country:             detail.Country,
		Private:             detail.Private,
		Hazardous:           detail.Hazardous,
		FetchTime:           s.nowFn().UTC(),
		LatLng:              streams.LatLng,
	}
	if len(streams.Altitude) == len(streams.LatLng) {
		seg.Altitude = streams.Altitude
	}
	if seg.Private {
		return seg, nil
	}
	data, err := encodeSegment(seg)
	if err != nil {
		return nil, err
	}
	// A segment that couldn't be cached is still returned, and fetched
	// again next time.
	if err := s.cache.Put(ctx, segmentKey(id), data, "application/gzip"); err != nil {
		s.report.Report(ctx, "Failed to cache a Strava segment", err)
	}
	s.remember(seg)
	return seg, nil
}

// fetchError returns the error for a failed fetch of a segment, forgetting
// the segment if Strava no longer has it.
func (s *Segments) fetchError(ctx context.Context, id int64, err error) error {
	if errors.Is(err, strava.ErrNotFound) {
		s.forget(ctx, id)
		return fmt.Errorf("%w: %d", ErrSegmentNotFound, id)
	}
	return err
}

// token returns a valid access token of the user's Strava connection,
// refreshing it if it expires soon.
func (s *Segments) token(ctx context.Context, conn *activities.StravaConnection) (string, error) {
	if s.nowFn().Add(5 * time.Minute).Before(conn.Expiry) {
		return conn.AccessToken, nil
	}
	// The refresh is serialized with the syncer's, which locks the
	// connection the same way, and reads the tokens again once locked, so
	// a token refreshed meanwhile is used rather than refreshed again.
	current, err := s.conns.RefreshStravaTokens(ctx, conn.UserID, func(stored *activities.StravaConnection) (*activities.StravaTokens, error) {
		if s.nowFn().Add(5 * time.Minute).Before(stored.Expiry) {
			return nil, nil
		}
		token, err := s.client.Refresh(ctx, stored.RefreshToken)
		if err != nil {
			return nil, err
		}
		return &activities.StravaTokens{AccessToken: token.AccessToken, RefreshToken: token.RefreshToken, Expiry: token.Expiry}, nil
	})
	if errors.Is(err, store.ErrNotFound) {
		// Strava was disconnected meanwhile.
		return "", ErrNotConnected
	}
	if err != nil {
		return "", err
	}
	return current.AccessToken, nil
}

func encodeSegment(seg *Segment) ([]byte, error) {
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	if err := json.NewEncoder(zw).Encode(seg); err != nil {
		return nil, fmt.Errorf("failed to encode segment: %w", err)
	}
	if err := zw.Close(); err != nil {
		return nil, fmt.Errorf("failed to compress segment: %w", err)
	}
	return buf.Bytes(), nil
}

func decodeSegment(data []byte) (*Segment, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	var seg Segment
	if err := json.NewDecoder(io.LimitReader(zr, 64<<20)).Decode(&seg); err != nil {
		return nil, err
	}
	return &seg, nil
}
