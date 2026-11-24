package routing_test

import (
	"context"
	"slices"
	"testing"

	"github.com/bs-tools/trailrunn.ing/internal/geo"
)

func TestTrackTerrains(t *testing.T) {
	t.Parallel()
	router := newRouter(t)
	tracks := [][]geo.Point{
		// Along the road, about 4 m east of it as GPS tracks are.
		{{Lat: 0.001, Lng: 0.00004}, {Lat: 0.005, Lng: 0.00004}, {Lat: 0.009, Lng: 0.00004}},
		// Along the east trail, from near its start where it meets the
		// road, then along the spur path back to the road.
		{{Lat: 0.0003, Lng: 0.00024}, {Lat: 0.0049, Lng: 0.0039}, {Lat: 0.005, Lng: 0.0019}, {Lat: 0.005, Lng: 0.0002}},
		// Off the map.
		{{Lat: 5, Lng: 5}, {Lat: 5.001, Lng: 5}},
		// A single point, and no points.
		{{Lat: 0.001, Lng: 0}},
		nil,
	}
	got, err := router.TrackTerrains(context.Background(), tracks)
	if err != nil {
		t.Fatal(err)
	}
	want := [][]geo.Terrain{
		{geo.TerrainPaved, geo.TerrainPaved, geo.TerrainUnknown},
		// The segment from the east trail to the spur path crosses open
		// ground at first, but is mostly near one or the other.
		{geo.TerrainTrail, geo.TerrainTrail, geo.TerrainTrail, geo.TerrainUnknown},
		{geo.TerrainUnknown, geo.TerrainUnknown},
		{geo.TerrainUnknown},
		{},
	}
	if len(got) != len(want) {
		t.Fatalf("got %d tracks, want %d", len(got), len(want))
	}
	for i := range want {
		if !slices.Equal(got[i], want[i]) {
			t.Errorf("track %d: got terrains %q, want %q", i, got[i], want[i])
		}
	}
}
