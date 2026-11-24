package services_test

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"connectrpc.com/connect/v2"
	routesv1 "github.com/bs-tools/trailrunn.ing/internal/gen/routes/v1"
	stravav1 "github.com/bs-tools/trailrunn.ing/internal/gen/strava/v1"
	"github.com/bs-tools/trailrunn.ing/internal/strava"
	"google.golang.org/protobuf/proto"
)

// GetSegment returns a segment added with addSegment, or ErrNotFound.
func (f *fakeStrava) GetSegment(_ context.Context, _ string, id int64) (*strava.Segment, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.segmentCalls++
	seg := f.segments[id]
	if seg == nil {
		return nil, strava.ErrNotFound
	}
	copied := *seg
	return &copied, nil
}

// GetSegmentStreams returns a segment's path: a climb of 3 points.
func (f *fakeStrava) GetSegmentStreams(_ context.Context, _ string, id int64) (*strava.SegmentStreams, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.segments[id] == nil {
		return nil, strava.ErrNotFound
	}
	return &strava.SegmentStreams{
		LatLng:   [][2]float64{{49.630022, -123.203187}, {49.634, -123.199}, {49.638, -123.195}},
		Distance: []float64{0, 540, 1080},
		Altitude: []float64{69, 260, 460},
	}, nil
}

func (f *fakeStrava) addSegment(seg *strava.Segment) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.segments == nil {
		f.segments = map[int64]*strava.Segment{}
	}
	f.segments[seg.ID] = seg
}

func (f *fakeStrava) segmentFetches() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.segmentCalls
}

func TestStravaSegments(t *testing.T) {
	e := newEnv(t)
	ctx := context.Background()
	e.strava.addSegment(&strava.Segment{
		ID: 40535472, Name: "VK XXIV - Rule Britannia", ActivityType: "Run",
		Distance: 4538, TotalElevationGain: 550, AverageGrade: 8.7,
		City: "Britannia Beach", State: "BC", Country: "Canada",
	})
	get := func(segment string, points bool) (*stravav1.StravaSegment, error) {
		resp, err := e.stravaAPI.GetStravaSegment(ctx, stravav1.GetStravaSegmentRequest_builder{
			Segment: segment, IncludePoints: points,
		}.Build())
		return resp.GetSegment(), err
	}

	// Strava's API needs the user's connection to get segments.
	_, err := get("40535472", false)
	wantCode(t, err, connect.CodeFailedPrecondition)
	if !strings.Contains(err.Error(), "connect Strava") {
		t.Errorf("error %q should say to connect Strava", err)
	}
	e.connectStrava()

	// What isn't a segment link or ID is rejected, and segments Strava
	// doesn't have aren't found.
	_, err = get("https://www.strava.com/activities/123", false)
	wantCode(t, err, connect.CodeInvalidArgument)
	_, err = get("https://www.strava.com/segments/1", false)
	wantCode(t, err, connect.CodeNotFound)

	// A segment is found from its link, with its statistics, and its path
	// if asked for.
	seg, err := get("https://www.strava.com/segments/40535472?filter=overall", false)
	if err != nil {
		t.Fatal(err)
	}
	if seg.GetId() != "40535472" || seg.GetName() != "VK XXIV - Rule Britannia" ||
		seg.GetDistanceMeters() != 4538 || seg.GetElevationGainMeters() != 550 ||
		seg.GetLocation() != "Britannia Beach, BC, Canada" ||
		seg.GetUrl() != "https://www.strava.com/segments/40535472" ||
		seg.GetGpxUrl() != "/strava/segments/40535472.gpx" || len(seg.GetPoints()) != 0 {
		t.Errorf("unexpected segment %v", seg)
	}
	fetches := e.strava.segmentFetches()
	seg, err = get("40535472", true)
	if err != nil {
		t.Fatal(err)
	}
	if len(seg.GetPoints()) != 3 || seg.GetPoints()[0].GetLat() != 49.630022 {
		t.Errorf("unexpected points %v", seg.GetPoints())
	}
	// The second time, it came from the cache.
	if e.strava.segmentFetches() != fetches {
		t.Errorf("the cached segment was fetched again")
	}

	// It downloads as a GPX file named after it, while signed in and
	// connected to Strava.
	resp, gpx := e.get("/strava/segments/40535472.gpx", e.token)
	if resp.StatusCode != http.StatusOK || strings.Count(string(gpx), "<trkpt") != 3 ||
		!strings.Contains(string(gpx), "<name>VK XXIV - Rule Britannia</name>") ||
		!strings.Contains(resp.Header.Get("Content-Disposition"), `filename="VK-XXIV-Rule-Britannia.gpx"`) {
		t.Errorf("unexpected GPX response %d %s %v", resp.StatusCode, gpx, resp.Header)
	}
	if resp, _ := e.get("/strava/segments/40535472.gpx", ""); resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("anonymous download: %d, want 401", resp.StatusCode)
	}
	if resp, _ := e.get("/strava/segments/40535472.gpx", e.otherToken); resp.StatusCode != http.StatusConflict {
		t.Errorf("download without Strava: %d, want 409", resp.StatusCode)
	}
	if resp, _ := e.get("/strava/segments/1.gpx", e.token); resp.StatusCode != http.StatusNotFound {
		t.Errorf("download of a missing segment: %d, want 404", resp.StatusCode)
	}

	// A route made from the segment follows its path, named after it and
	// linking to it.
	created, err := e.routes.CreateRoute(ctx, routesv1.CreateRouteRequest_builder{
		StravaSegment: proto.String("https://www.strava.com/segments/40535472"),
		IncludePoints: true,
	}.Build())
	if err != nil {
		t.Fatal(err)
	}
	route := created.GetRoute()
	if route.GetName() != "VK XXIV - Rule Britannia" ||
		route.GetDescription() != "From the Strava segment https://www.strava.com/segments/40535472" ||
		len(route.GetPoints()) < 3 || route.GetPoints()[len(route.GetPoints())-1].GetElevationMeters() != 460 {
		t.Errorf("unexpected route %v", route)
	}
	// Its name and description can be given instead.
	created, err = e.routes.CreateRoute(ctx, routesv1.CreateRouteRequest_builder{
		Name:          "Britannia VK",
		Description:   "Sunday",
		StravaSegment: proto.String("40535472"),
	}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if created.GetRoute().GetName() != "Britannia VK" || created.GetRoute().GetDescription() != "Sunday" {
		t.Errorf("unexpected route %v", created.GetRoute())
	}
	_, err = e.routes.CreateRoute(ctx, routesv1.CreateRouteRequest_builder{StravaSegment: proto.String("nope")}.Build())
	wantCode(t, err, connect.CodeInvalidArgument)
}
