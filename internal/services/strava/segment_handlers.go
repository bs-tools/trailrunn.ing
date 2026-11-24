package strava

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/bs-tools/trailrunn.ing/internal/authn"
	geov1 "github.com/bs-tools/trailrunn.ing/internal/gen/geo/v1"
	stravav1 "github.com/bs-tools/trailrunn.ing/internal/gen/strava/v1"
	"github.com/bs-tools/trailrunn.ing/internal/geo"
	"github.com/bs-tools/trailrunn.ing/internal/strava"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// SegmentDownloadPath is where segments are downloaded as GPX files, at
// /strava/segments/{id}.gpx.
const SegmentDownloadPath = "/strava/segments/"

// SetSegments sets where the service gets segments from. Without it,
// GetStravaSegment fails, as on a server without Strava.
func (s *Service) SetSegments(segments *Segments) {
	s.segments = segments
}

func (s *Service) GetStravaSegment(ctx context.Context, req *stravav1.GetStravaSegmentRequest) (*stravav1.GetStravaSegmentResponse, error) {
	userID, ok := authn.GetUserID(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, "")
	}
	if s.segments == nil {
		return nil, errStravaNotConfigured
	}
	id, err := ParseSegmentID(req.GetSegment())
	if err != nil {
		return nil, s.segments.connectError(ctx, err, req.GetSegment())
	}
	seg, err := s.segments.Get(ctx, userID, id)
	if err != nil {
		return nil, s.segments.connectError(ctx, err, req.GetSegment())
	}
	b := stravav1.StravaSegment_builder{
		Id:                  strconv.FormatInt(seg.ID, 10),
		Name:                seg.Name,
		Url:                 s.segments.URL(seg.ID),
		GpxUrl:              fmt.Sprintf("%s%d.gpx", SegmentDownloadPath, seg.ID),
		ActivityType:        seg.ActivityType,
		DistanceMeters:      seg.DistanceMeters,
		ElevationGainMeters: seg.ElevationGainMeters,
		AverageGradePercent: seg.AverageGradePercent,
		Location:            seg.Location(),
		Private:             seg.Private,
		Hazardous:           seg.Hazardous,
		FetchTime:           timestamppb.New(seg.FetchTime),
	}
	if req.GetIncludePoints() {
		b.Points = make([]*geov1.LatLng, len(seg.LatLng))
		for i, ll := range seg.LatLng {
			b.Points[i] = geov1.LatLng_builder{Lat: ll[0], Lng: ll[1]}.Build()
		}
	}
	return stravav1.GetStravaSegmentResponse_builder{Segment: b.Build()}.Build(), nil
}

var errStravaNotConfigured = connect.NewError(connect.CodeFailedPrecondition,
	"Strava isn't set up on this server, so segments can't be fetched")

// URL returns the link to a segment on Strava's website.
func (s *Segments) URL(id int64) string {
	return fmt.Sprintf("%s/segments/%d", s.webURL, id)
}

// Track returns a segment's name, link and path, to make a route of it.
// Its errors are Connect errors for users. segment is a link to the
// segment or its ID.
func (s *Segments) Track(ctx context.Context, userID pgtype.UUID, segment string) (name, link string, points []geo.Point, err error) {
	id, err := ParseSegmentID(segment)
	if err != nil {
		return "", "", nil, s.connectError(ctx, err, segment)
	}
	seg, err := s.Get(ctx, userID, id)
	if err != nil {
		return "", "", nil, s.connectError(ctx, err, segment)
	}
	return seg.Name, s.URL(id), seg.Path(), nil
}

// connectError returns the Connect error for a failure to get a segment,
// saying what to do.
func (s *Segments) connectError(ctx context.Context, err error, segment string) error {
	var rateErr *strava.RateLimitError
	switch {
	case errors.Is(err, ErrInvalidSegment):
		return connect.Errorf(connect.CodeInvalidArgument,
			"%q isn't a Strava segment: give a link like https://www.strava.com/segments/40535472, or the segment's ID", segment)
	case errors.Is(err, ErrNotConnected):
		return connect.NewError(connect.CodeFailedPrecondition,
			"connect Strava in the settings first: Strava's API only gives segments to the athletes connected to the app")
	case errors.Is(err, strava.ErrUnauthorized):
		return connect.NewError(connect.CodeFailedPrecondition, err.Error())
	case errors.Is(err, ErrSegmentNotFound):
		return connect.NewError(connect.CodeNotFound,
			"Strava has no segment with that ID that you can see: check the link, or whether the segment is private or was deleted")
	case errors.As(err, &rateErr):
		return connect.Errorf(connect.CodeResourceExhausted,
			"the app has used up its Strava requests for now; try again after %s", rateErr.ResetAt.UTC().Format("15:04 MST"))
	default:
		s.report.Report(ctx, "Failed to get a Strava segment", err)
		return connect.NewError(connect.CodeUnavailable, "failed to get the segment from Strava; try again later").WithCause(err)
	}
}

// unsafeFilenameChars matches the runs of characters replaced with a dash
// in download file names. Dashes are among them, so "VK XXIV - Rule
// Britannia" becomes "VK-XXIV-Rule-Britannia".
var unsafeFilenameChars = regexp.MustCompile(`[^A-Za-z0-9._]+`)

// SegmentDownloadHandler serves segments as GPX files at
// /strava/segments/{id}.gpx, named after the segment. It must be mounted at
// SegmentDownloadPath behind the authn middleware.
func (s *Service) SegmentDownloadHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		name, isGPX := strings.CutSuffix(strings.TrimPrefix(r.URL.Path, SegmentDownloadPath), ".gpx")
		if !isGPX || r.Method != http.MethodGet {
			http.NotFound(w, r)
			return
		}
		userID, ok := authn.GetUserID(r.Context())
		if !ok {
			http.Error(w, "sign in to download segments", http.StatusUnauthorized)
			return
		}
		if s.segments == nil {
			http.Error(w, "strava isn't set up on this server", http.StatusServiceUnavailable)
			return
		}
		id, err := ParseSegmentID(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		seg, err := s.segments.Get(r.Context(), userID, id)
		if err != nil {
			var connectErr *connect.Error
			if errors.As(s.segments.connectError(r.Context(), err, name), &connectErr) {
				http.Error(w, connectErr.Message(), httpStatus(connectErr.Code()))
				return
			}
			http.Error(w, "failed to get the segment", http.StatusInternalServerError)
			return
		}
		filename := strings.Trim(unsafeFilenameChars.ReplaceAllString(seg.Name, "-"), "-")
		if filename == "" {
			filename = fmt.Sprintf("segment-%d", seg.ID)
		}
		w.Header().Set("Content-Type", "application/gpx+xml")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s.gpx"`, filename))
		// The browser may keep the file until it goes stale in the cache.
		maxAge := max(0, SegmentMaxAge-time.Since(seg.FetchTime))
		w.Header().Set("Cache-Control", fmt.Sprintf("private, max-age=%d", int(maxAge.Seconds())))
		if err := geo.WriteGPX(w, seg.Name, seg.Path()); err != nil {
			s.report.Report(r.Context(), "Failed to write a segment's GPX file", err)
		}
	})
}

// httpStatus returns the HTTP status for a Connect error code.
func httpStatus(code connect.Code) int {
	switch code {
	case connect.CodeInvalidArgument:
		return http.StatusBadRequest
	case connect.CodeNotFound:
		return http.StatusNotFound
	case connect.CodeFailedPrecondition:
		return http.StatusConflict
	case connect.CodeResourceExhausted:
		return http.StatusTooManyRequests
	default:
		return http.StatusServiceUnavailable
	}
}
