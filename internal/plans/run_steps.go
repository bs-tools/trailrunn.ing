package plans

import (
	"encoding/json"
	"fmt"
)

// RunStepKind is what a step of a structured run is.
type RunStepKind string

// Run step kinds.
const (
	RunStepWarmUp   RunStepKind = "warm_up"
	RunStepRun      RunStepKind = "run"
	RunStepRecovery RunStepKind = "recovery"
	RunStepRest     RunStepKind = "rest"
	RunStepCoolDown RunStepKind = "cool_down"
)

// HeartRateReference is what a heart rate target is measured from.
type HeartRateReference string

// Heart rate references.
const (
	HeartRateBPM                HeartRateReference = ""
	HeartRateAerobicThreshold   HeartRateReference = "aerobic_threshold"
	HeartRateAnaerobicThreshold HeartRateReference = "anaerobic_threshold"
)

// RunBlock is a block of a structured run: its steps, done Repeats times.
// It's stored as JSON with the workout, so its field names mustn't change.
type RunBlock struct {
	Repeats int       `json:"repeats"`
	Steps   []RunStep `json:"steps"`
}

// RunStep is a step of a structured run. It lasts DurationSeconds or
// DistanceMeters, or until the lap button if neither is set.
type RunStep struct {
	Kind            RunStepKind      `json:"kind"`
	DurationSeconds int32            `json:"duration_seconds,omitempty"`
	DistanceMeters  float64          `json:"distance_meters,omitempty"`
	HeartRate       *HeartRateTarget `json:"heart_rate,omitempty"`
	Pace            *PaceTarget      `json:"pace,omitempty"`
	Notes           string           `json:"notes,omitempty"`
}

// HeartRateTarget is a heart rate range: in beats per minute, relative to
// a threshold, or from the bottom of LowZone to the top of HighZone, zones
// of Uphill Athlete's system, see HeartRateZones. It's stored as JSON, so
// its field names mustn't change.
type HeartRateTarget struct {
	Reference HeartRateReference `json:"reference,omitempty"`
	LowBPM    int32              `json:"low_bpm,omitempty"`
	HighBPM   int32              `json:"high_bpm,omitempty"`
	// From 1 to 4, and 0 if the range isn't of zones.
	LowZone  int32 `json:"low_zone,omitempty"`
	HighZone int32 `json:"high_zone,omitempty"`
}

// PaceTarget is a pace range, in seconds per kilometer.
type PaceTarget struct {
	FastSecondsPerKilometer int32 `json:"fast_seconds_per_kilometer"`
	SlowSecondsPerKilometer int32 `json:"slow_seconds_per_kilometer"`
}

// Resolve returns a heart rate target's range in beats per minute, given
// the user's thresholds, 0 if unknown. It reports false if it's relative to
// an unknown threshold, or of zones that need one.
func (t *HeartRateTarget) Resolve(aerobicBPM, anaerobicBPM int32) (low, high int32, ok bool) {
	if t.LowZone > 0 {
		zones, ok := UphillAthleteZones(aerobicBPM, anaerobicBPM)
		if !ok {
			return 0, 0, false
		}
		return zones.Range(t.LowZone, t.HighZone)
	}
	var base int32
	switch t.Reference {
	case HeartRateBPM:
	case HeartRateAerobicThreshold:
		base = aerobicBPM
	case HeartRateAnaerobicThreshold:
		base = anaerobicBPM
	}
	if t.Reference != HeartRateBPM && base == 0 {
		return 0, 0, false
	}
	return base + t.LowBPM, base + t.HighBPM, true
}

// heartRateToDB returns a heart rate target as stored, empty for none.
func heartRateToDB(t *HeartRateTarget) string {
	if t == nil {
		return ""
	}
	data, err := json.Marshal(t)
	if err != nil {
		// The target is plain values, which always marshal.
		panic(fmt.Sprintf("marshaling a heart rate target: %v", err))
	}
	return string(data)
}

// heartRateFromDB returns a heart rate target from the database.
func heartRateFromDB(data []byte) *HeartRateTarget {
	if len(data) == 0 {
		return nil
	}
	var t HeartRateTarget
	if err := json.Unmarshal(data, &t); err != nil {
		return nil
	}
	return &t
}

// runBlocksToDB returns a run's blocks as stored, empty for none.
func runBlocksToDB(blocks []RunBlock) string {
	if len(blocks) == 0 {
		return ""
	}
	data, err := json.Marshal(blocks)
	if err != nil {
		// The blocks are plain values, which always marshal.
		panic(fmt.Sprintf("marshaling run blocks: %v", err))
	}
	return string(data)
}

// runBlocksFromDB returns a run's blocks from the database.
func runBlocksFromDB(data []byte) []RunBlock {
	if len(data) == 0 {
		return nil
	}
	var blocks []RunBlock
	if err := json.Unmarshal(data, &blocks); err != nil {
		return nil
	}
	return blocks
}
