package strava

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetSegment(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/segments/40535472":
			// Abridged from Strava's response, with a 64-bit ID.
			fmt.Fprint(w, `{"id": 40535472, "resource_state": 3, "name": "VK XXIV - Rule Britannia", "activity_type": "Run",
				"distance": 4538.2, "average_grade": 8.7, "maximum_grade": 40.1, "elevation_high": 460, "elevation_low": 64,
				"total_elevation_gain": 550.4, "city": "Britannia Beach", "state": "BC", "country": "Canada",
				"private": false, "hazardous": false, "starred": false, "map": {"polyline": "abc"}}`)
		case "/segments/40535472/streams":
			if r.URL.Query().Get("keys") != "latlng,distance,altitude" || r.URL.Query().Get("key_by_type") != "true" {
				http.Error(w, "unexpected query "+r.URL.RawQuery, http.StatusBadRequest)
				return
			}
			fmt.Fprint(w, `{"latlng": {"data": [[49.630022, -123.203187], [49.63, -123.2]]}, "distance": {"data": [0, 10.5]},
				"altitude": {"data": [69, 70.2]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)
	client := New(Config{BaseURL: srv.URL, HTTPClient: srv.Client()})
	ctx := context.Background()
	seg, err := client.GetSegment(ctx, "token", 40535472)
	if err != nil {
		t.Fatal(err)
	}
	if seg.ID != 40535472 || seg.Name != "VK XXIV - Rule Britannia" || seg.Distance != 4538.2 ||
		seg.TotalElevationGain != 550.4 || seg.City != "Britannia Beach" || seg.Private {
		t.Errorf("unexpected segment %+v", seg)
	}
	streams, err := client.GetSegmentStreams(ctx, "token", 40535472)
	if err != nil {
		t.Fatal(err)
	}
	if len(streams.LatLng) != 2 || streams.LatLng[0][0] != 49.630022 || streams.Distance[1] != 10.5 || streams.Altitude[1] != 70.2 {
		t.Errorf("unexpected streams %+v", streams)
	}
	if _, err := client.GetSegment(ctx, "token", 2); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
	if _, err := client.GetSegmentStreams(ctx, "token", 2); !errors.Is(err, ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
}
