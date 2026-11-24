package strava

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync"
	"testing"
	"time"

	"github.com/bs-tools/trailrunn.ing/internal/activities"
	"github.com/bs-tools/trailrunn.ing/internal/blob"
	"github.com/bs-tools/trailrunn.ing/internal/errreport"
	"github.com/bs-tools/trailrunn.ing/internal/store"
	"github.com/bs-tools/trailrunn.ing/internal/strava"
	"github.com/jackc/pgx/v5/pgtype"
)

func TestParseSegmentID(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int64
	}{
		{"40535472", 40535472},
		{" 40535472 ", 40535472},
		{"https://www.strava.com/segments/40535472", 40535472},
		{"https://www.strava.com/segments/40535472?filter=overall", 40535472},
		{"https://www.strava.com/segments/40535472/", 40535472},
		{"strava.com/segments/40535472#leaderboard", 40535472},
		{"http://strava.com/segments/1", 1},
	} {
		got, err := ParseSegmentID(tc.in)
		if err != nil || got != tc.want {
			t.Errorf("ParseSegmentID(%q) = %d, %v; want %d", tc.in, got, err, tc.want)
		}
	}
	for _, in := range []string{
		"",
		"hello",
		"0",
		"https://www.strava.com/activities/40535472",
		"https://evil.example/segments/40535472",
		"https://www.strava.com/segments/abc",
		"99999999999999999999",
	} {
		if _, err := ParseSegmentID(in); !errors.Is(err, ErrInvalidSegment) {
			t.Errorf("ParseSegmentID(%q) = %v, want ErrInvalidSegment", in, err)
		}
	}
}

// fakeSegmentConns has one user's connection, whose token expires at
// expiry, and counts the refreshes.
type fakeSegmentConns struct {
	mu        sync.Mutex
	userID    pgtype.UUID
	expiry    time.Time
	token     string
	refreshes int
}

func (f *fakeSegmentConns) GetStravaConnection(_ context.Context, userID pgtype.UUID) (*activities.StravaConnection, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if userID != f.userID {
		return nil, store.ErrNotFound
	}
	return &activities.StravaConnection{UserID: userID, AccessToken: f.token, RefreshToken: "refresh", Expiry: f.expiry}, nil
}

func (f *fakeSegmentConns) RefreshStravaTokens(ctx context.Context, userID pgtype.UUID, refresh func(stored *activities.StravaConnection) (*activities.StravaTokens, error)) (*activities.StravaConnection, error) {
	conn, err := f.GetStravaConnection(ctx, userID)
	if err != nil {
		return nil, err
	}
	tokens, err := refresh(conn)
	if err != nil || tokens == nil {
		return conn, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.refreshes++
	f.token, f.expiry = tokens.AccessToken, tokens.Expiry
	return &activities.StravaConnection{UserID: userID, AccessToken: f.token, Expiry: f.expiry}, nil
}

// fakeSegmentClient serves segments, and records the token of each request
// for a segment, which comes before the one for its path.
type fakeSegmentClient struct {
	mu       sync.Mutex
	segments map[int64]*strava.Segment
	tokens   []string
}

func (f *fakeSegmentClient) Refresh(context.Context, string) (*strava.Token, error) {
	return &strava.Token{AccessToken: "fresh", RefreshToken: "refresh", Expiry: time.Now().Add(6 * time.Hour)}, nil
}

func (f *fakeSegmentClient) GetSegment(_ context.Context, token string, id int64) (*strava.Segment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.tokens = append(f.tokens, token)
	seg := f.segments[id]
	if seg == nil {
		return nil, strava.ErrNotFound
	}
	copied := *seg
	return &copied, nil
}

func (f *fakeSegmentClient) GetSegmentStreams(_ context.Context, _ string, id int64) (*strava.SegmentStreams, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.segments[id] == nil {
		return nil, strava.ErrNotFound
	}
	return &strava.SegmentStreams{
		LatLng:   [][2]float64{{49.63, -123.2}, {49.64, -123.21}, {49.65, -123.22}},
		Distance: []float64{0, 1300, 2600},
		Altitude: []float64{69, 300, 460},
	}, nil
}

func (f *fakeSegmentClient) requests() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.tokens)
}

func TestSegmentsCache(t *testing.T) {
	ctx := context.Background()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	userID, err := store.StringToUUID("8f6c1f54-4b1e-4f4e-9a0e-2f0b0a8f6c1f")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 24, 12, 0, 0, 0, time.UTC)
	conns := &fakeSegmentConns{userID: userID, token: "stale", expiry: now.Add(time.Minute)}
	client := &fakeSegmentClient{segments: map[int64]*strava.Segment{
		40535472: {ID: 40535472, Name: " VK XXIV - Rule Britannia ", Distance: 4538, TotalElevationGain: 550, City: "Britannia Beach", State: "BC", Country: "Canada"},
		7:        {ID: 7, Name: "Secret climb", Private: true},
	}}
	dir, err := blob.NewDir(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	segments := NewSegments(log, errreport.NewLocal(log), conns, client, dir, strava.WebURL)
	segments.nowFn = func() time.Time { return now }

	// The first fetch refreshes the token that expires soon, and asks
	// Strava for the segment and its path.
	seg, err := segments.Get(ctx, userID, 40535472)
	if err != nil {
		t.Fatal(err)
	}
	if seg.Name != "VK XXIV - Rule Britannia" || seg.Location() != "Britannia Beach, BC, Canada" || len(seg.Path()) != 3 || seg.Path()[2].Ele != 460 {
		t.Errorf("segment = %+v", seg)
	}
	if conns.refreshes != 1 || client.tokens[0] != "fresh" {
		t.Errorf("refreshes = %d, tokens = %v; want the refreshed token", conns.refreshes, client.tokens)
	}

	// Later fetches come from memory, then from the store.
	if _, err := segments.Get(ctx, userID, 40535472); err != nil || client.requests() != 1 {
		t.Errorf("second fetch: %v, %d requests to Strava", err, client.requests())
	}
	segments.recent = map[int64]*Segment{}
	now = now.Add(SegmentMaxAge - time.Minute)
	if _, err := segments.Get(ctx, userID, 40535472); err != nil || client.requests() != 1 {
		t.Errorf("fetch from the store: %v, %d requests to Strava", err, client.requests())
	}

	// Once SegmentMaxAge old, the segment is fetched again.
	now = now.Add(2 * time.Minute)
	if _, err := segments.Get(ctx, userID, 40535472); err != nil || client.requests() != 2 {
		t.Errorf("stale fetch: %v, %d requests to Strava", err, client.requests())
	}

	// A segment Strava no longer has is removed from the cache.
	client.mu.Lock()
	delete(client.segments, 40535472)
	client.mu.Unlock()
	now = now.Add(SegmentMaxAge)
	if _, err := segments.Get(ctx, userID, 40535472); !errors.Is(err, ErrSegmentNotFound) {
		t.Errorf("deleted segment: %v, want ErrSegmentNotFound", err)
	}
	if _, err := dir.Get(ctx, segmentKey(40535472)); !errors.Is(err, blob.ErrNotFound) {
		t.Errorf("deleted segment is still cached: %v", err)
	}

	// Private segments are never cached, so they're fetched every time.
	for range 2 {
		if seg, err := segments.Get(ctx, userID, 7); err != nil || !seg.Private {
			t.Fatalf("private segment: %+v, %v", seg, err)
		}
	}
	if client.requests() != 5 {
		t.Errorf("%d requests to Strava, want the private segment fetched twice", client.requests())
	}
	if _, err := dir.Get(ctx, segmentKey(7)); !errors.Is(err, blob.ErrNotFound) {
		t.Errorf("private segment was cached: %v", err)
	}

	// Users who haven't connected Strava get no segments, not even cached
	// ones.
	other, err := store.StringToUUID("1f6c1f54-4b1e-4f4e-9a0e-2f0b0a8f6c1f")
	if err != nil {
		t.Fatal(err)
	}
	client.mu.Lock()
	client.segments[40535472] = &strava.Segment{ID: 40535472, Name: "Back"}
	client.mu.Unlock()
	if _, err := segments.Get(ctx, userID, 40535472); err != nil {
		t.Fatal(err)
	}
	if _, err := segments.Get(ctx, other, 40535472); !errors.Is(err, ErrNotConnected) {
		t.Errorf("unconnected user: %v, want ErrNotConnected", err)
	}
}
