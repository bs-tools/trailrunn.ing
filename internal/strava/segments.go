package strava

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
)

// Segment is a Strava segment: a stretch of road or trail that athletes
// compare their times on.
type Segment struct {
	ID int64 `json:"id"`
	// The segment's name, as its creator named it.
	Name string `json:"name"`
	// The sport, e.g. "Run" or "Ride".
	ActivityType string `json:"activity_type"`
	// Distance in meters.
	Distance float64 `json:"distance"`
	// Average grade in percent.
	AverageGrade float64 `json:"average_grade"`
	// Total elevation gain in meters.
	TotalElevationGain float64 `json:"total_elevation_gain"`
	City               string  `json:"city"`
	State              string  `json:"state"`
	Country            string  `json:"country"`
	// Private segments are only visible to the athlete who created them.
	Private bool `json:"private"`
	// Hazardous segments were flagged as dangerous by athletes, e.g. a
	// descent on a busy road.
	Hazardous bool `json:"hazardous"`
}

// GetSegment gets a segment. It returns ErrNotFound if there's no segment
// with the ID the athlete can see.
func (c *Client) GetSegment(ctx context.Context, accessToken string, id int64) (*Segment, error) {
	body, err := c.get(ctx, accessToken, "/segments/"+strconv.FormatInt(id, 10), nil)
	if err != nil {
		return nil, err
	}
	var s Segment
	if err := json.Unmarshal(body, &s); err != nil {
		return nil, fmt.Errorf("failed to decode segment: %w", err)
	}
	return &s, nil
}

// SegmentStreams are a segment's path: its coordinates, with the distance
// from the start and the elevation at each. Streams Strava doesn't have are
// nil.
type SegmentStreams struct {
	LatLng   [][2]float64
	Distance []float64
	Altitude []float64
}

// GetSegmentStreams gets a segment's path at full resolution. It returns
// ErrNotFound if there's no segment with the ID the athlete can see.
func (c *Client) GetSegmentStreams(ctx context.Context, accessToken string, id int64) (*SegmentStreams, error) {
	q := url.Values{}
	q.Set("keys", "latlng,distance,altitude")
	q.Set("key_by_type", "true")
	body, err := c.get(ctx, accessToken, "/segments/"+strconv.FormatInt(id, 10)+"/streams", q)
	if err != nil {
		return nil, err
	}
	var resp streamsResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("failed to decode segment streams: %w", err)
	}
	streams := &SegmentStreams{}
	if resp.LatLng != nil {
		streams.LatLng = resp.LatLng.Data
	}
	if resp.Distance != nil {
		streams.Distance = resp.Distance.Data
	}
	if resp.Altitude != nil {
		streams.Altitude = resp.Altitude.Data
	}
	return streams, nil
}
