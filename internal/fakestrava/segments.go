package fakestrava

import (
	"fmt"
	"net/http"
	"sort"
)

// segmentMeters is how long the fake segments are.
const segmentMeters = 3000

// A fake segment is the start of an activity with a track, up to
// segmentMeters long, with the activity's ID: there are no real segments to
// copy, and this gives one for every recorded activity, along trails.
type segment struct {
	id       int64
	name     string
	distance []float64
	latlng   [][2]float64
	altitude []float64
}

// segment returns the segment of a request, or writes a not found error.
func (s *Server) segment(w http.ResponseWriter, r *http.Request) *segment {
	a := s.activity(w, r)
	if a == nil {
		return nil
	}
	if a.streams == nil || len(a.streams.latlng) < 2 {
		writeJSON(w, http.StatusNotFound, map[string]any{"message": "Record Not Found"})
		return nil
	}
	st := a.streams
	end := sort.SearchFloat64s(st.distance, segmentMeters)
	end = max(2, min(end+1, len(st.latlng)))
	return &segment{
		id:       a.id,
		name:     fmt.Sprintf("The start of %s", a.name),
		distance: st.distance[:end],
		latlng:   st.latlng[:end],
		altitude: st.altitude[:end],
	}
}

func (s *Server) getSegment(w http.ResponseWriter, r *http.Request) {
	seg := s.segment(w, r)
	if seg == nil {
		return
	}
	gain := 0.0
	for i := 1; i < len(seg.altitude); i++ {
		gain += max(0, seg.altitude[i]-seg.altitude[i-1])
	}
	distance := seg.distance[len(seg.distance)-1] - seg.distance[0]
	grade := 0.0
	if distance > 0 {
		grade = (seg.altitude[len(seg.altitude)-1] - seg.altitude[0]) / distance * 100
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":                   seg.id,
		"resource_state":       3,
		"name":                 seg.name,
		"activity_type":        "Run",
		"distance":             distance,
		"average_grade":        grade,
		"total_elevation_gain": gain,
		"city":                 "Squamish",
		"state":                "BC",
		"country":              "Canada",
		"private":              false,
		"hazardous":            false,
	})
}

func (s *Server) getSegmentStreams(w http.ResponseWriter, r *http.Request) {
	seg := s.segment(w, r)
	if seg == nil {
		return
	}
	stream := func(data any) map[string]any {
		return map[string]any{"data": data, "series_type": "distance", "original_size": len(seg.latlng), "resolution": "high"}
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"latlng":   stream(seg.latlng),
		"distance": stream(seg.distance),
		"altitude": stream(seg.altitude),
	})
}
