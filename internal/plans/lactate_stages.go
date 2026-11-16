package plans

import "fmt"

// LactateTestStages is how a lactate test is run: stages of StageMinutes at
// heart rates from StartBPM up to EndBPM, StepBPM apart, each held within
// RangeBPM of its heart rate. Around ThresholdBPM, where the aerobic
// threshold is expected, the stages are closer together, so the test finds
// it more precisely.
type LactateTestStages struct {
	StartBPM     int32
	StepBPM      int32
	EndBPM       int32
	StageMinutes int32
	RangeBPM     int32
	// The expected aerobic threshold, like the last test's, or 0 if unknown.
	ThresholdBPM int32
}

// DefaultLactateTestStages are the stages of a test without a previous one,
// the same as the web app's guided test.
var DefaultLactateTestStages = LactateTestStages{
	StartBPM:     100,
	StepBPM:      10,
	EndBPM:       180,
	StageMinutes: 4,
	RangeBPM:     5,
}

// LactateTestWarmUpMinutes is how long a test's warm-up lasts.
const LactateTestWarmUpMinutes = 15

// How a test is run around the expected aerobic threshold. The web app's
// guided test has the same values, in lactate-utils.ts.
const (
	// lactateFineStepBPM is how far apart the stages near the expected
	// threshold are. Heart rates in fives are easy to remember and hold.
	lactateFineStepBPM = 5
	// lactateFineWindowBPM is how far either side of the expected threshold
	// the stages are that close: far enough to still bracket a threshold
	// that moved by a stage since the last test.
	lactateFineWindowBPM = 10
	// lactateFineRangeBPM is how far the heart rate may stray from a stage's
	// near the expected threshold. The usual range would make neighboring
	// stages the same effort.
	lactateFineRangeBPM = 3
	// lactateStartBelowThresholdBPM is how far below the expected threshold
	// a test starts: three stages of baseline before the close ones, without
	// the time and strips of stages far below it.
	lactateStartBelowThresholdBPM = 30
)

// minLactateTestStages is the fewest stages a test from a high start heart
// rate still gets, so it reaches past the aerobic threshold.
const minLactateTestStages = 5

// LactateTestStagesFrom returns the stages of a test for a user whose last
// test started at startBPM and found the aerobic threshold at thresholdBPM,
// each 0 if unknown. With a threshold, the test starts a few stages below it
// and has closer stages around it. Without one it starts where the last
// test did, so the two are comparable, or else has the defaults.
func LactateTestStagesFrom(startBPM, thresholdBPM int32) LactateTestStages {
	s := DefaultLactateTestStages
	if thresholdBPM > 0 {
		s.ThresholdBPM = thresholdBPM
		low, high, _ := s.fineZone()
		s.StartBPM = low + lactateFineWindowBPM - lactateStartBelowThresholdBPM
		// A stage past the close ones, in case the threshold rose.
		s.EndBPM = max(s.EndBPM, high+s.StepBPM)
		return s
	}
	if startBPM <= 0 {
		return s
	}
	s.StartBPM = startBPM
	if last := startBPM + (minLactateTestStages-1)*s.StepBPM; s.EndBPM < last {
		s.EndBPM = last
	}
	return s
}

// fineZone returns the heart rates between which the stages are closer
// together, around the expected threshold rounded to 5 bpm, if there is one.
func (s LactateTestStages) fineZone() (low, high int32, ok bool) {
	if s.ThresholdBPM <= 0 {
		return 0, 0, false
	}
	center := (s.ThresholdBPM + lactateFineStepBPM/2) / lactateFineStepBPM * lactateFineStepBPM
	return center - lactateFineWindowBPM, center + lactateFineWindowBPM, true
}

// nextBPM returns the heart rate of the stage after the one at bpm: a step
// higher, or a smaller step around the expected threshold. The last stage
// below the close ones stops where they start.
func (s LactateTestStages) nextBPM(bpm int32) int32 {
	low, high, ok := s.fineZone()
	switch {
	case !ok || bpm >= high:
		return bpm + s.StepBPM
	case bpm < low:
		return min(bpm+s.StepBPM, low)
	default:
		return bpm + min(s.StepBPM, lactateFineStepBPM)
	}
}

// LactateTestStage is a stage of a test: the heart rate to hold, and how far
// it may stray from it.
type LactateTestStage struct {
	BPM, RangeBPM int32
}

// Stages returns the test's stages, from the lowest heart rate.
func (s LactateTestStages) Stages() []LactateTestStage {
	low, high, ok := s.fineZone()
	var stages []LactateTestStage
	for bpm := s.StartBPM; bpm <= s.EndBPM; bpm = s.nextBPM(bpm) {
		stage := LactateTestStage{BPM: bpm, RangeBPM: s.RangeBPM}
		if ok && bpm >= low && bpm <= high {
			stage.RangeBPM = min(s.RangeBPM, lactateFineRangeBPM)
		}
		stages = append(stages, stage)
	}
	return stages
}

// Blocks returns the test as the blocks of a structured run, for watches: a
// warm-up, told to stay below the first stage, then each stage held in its heart rate
// range followed by an open step to sample lactate in, which ends at the lap
// button, and a cool-down.
func (s LactateTestStages) Blocks() []RunBlock {
	steps := []RunStep{{
		Kind:            RunStepWarmUp,
		DurationSeconds: LactateTestWarmUpMinutes * 60,
		Notes:           fmt.Sprintf("Very easy, below %d bpm", s.StartBPM),
	}}
	for _, stage := range s.Stages() {
		low, high := stage.BPM-stage.RangeBPM, stage.BPM+stage.RangeBPM
		steps = append(steps,
			RunStep{
				Kind:            RunStepRun,
				DurationSeconds: s.StageMinutes * 60,
				HeartRate:       &HeartRateTarget{LowBPM: low, HighBPM: high},
				Notes:           fmt.Sprintf("%d bpm: hold %d-%d bpm, then take a sample", stage.BPM, low, high),
			},
			RunStep{
				Kind:  RunStepRest,
				Notes: "Measure your lactate, then press lap",
			},
		)
	}
	steps = append(steps, RunStep{Kind: RunStepCoolDown, Notes: "Easy, until you press lap"})
	return []RunBlock{{Repeats: 1, Steps: steps}}
}
