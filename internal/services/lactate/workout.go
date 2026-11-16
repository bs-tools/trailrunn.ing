package lactate

import (
	"bytes"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/bs-tools/trailrunn.ing/internal/fit"
	"github.com/bs-tools/trailrunn.ing/internal/plans"
)

// WorkoutPath is the path of a lactate test as a Garmin workout.
const WorkoutPath = "/lactate-tests/workout.fit"

// workoutWarmUp is how long the workout's warm-up lasts.
const workoutWarmUp = plans.LactateTestWarmUpMinutes * time.Minute

// workoutParam is a query parameter of the workout, with its default and
// bounds.
type workoutParam struct {
	name               string
	fallback, min, max int
}

// The workout's query parameters, matching the guided test's settings, and
// by default the stages of planned tests on the Garmin calendar.
var (
	startParam   = workoutParam{"start", int(plans.DefaultLactateTestStages.StartBPM), 60, 200}
	stepParam    = workoutParam{"step", int(plans.DefaultLactateTestStages.StepBPM), 2, 30}
	endParam     = workoutParam{"end", int(plans.DefaultLactateTestStages.EndBPM), 60, 220}
	minutesParam = workoutParam{"minutes", int(plans.DefaultLactateTestStages.StageMinutes), 1, 20}
	rangeParam   = workoutParam{"range", int(plans.DefaultLactateTestStages.RangeBPM), 1, 15}
	// The expected aerobic threshold, around which the stages are closer
	// together. 0, the default, is none.
	thresholdParam = workoutParam{"threshold", 0, 0, 220}
)

// get returns the parameter's value in the query, or its default if unset.
func (p workoutParam) get(r *http.Request) (int, error) {
	text := r.URL.Query().Get(p.name)
	if text == "" {
		return p.fallback, nil
	}
	v, err := strconv.Atoi(text)
	if err != nil || v < p.min || v > p.max {
		return 0, fmt.Errorf("%s must be a whole number from %d to %d", p.name, p.min, p.max)
	}
	return v, nil
}

// WorkoutHandler serves a lactate test as a FIT workout file, which Garmin
// watches guide stage by stage, alerting when the heart rate leaves each
// stage's range, and waiting for the lap button while the user samples. The
// query sets the stages like the guided test: the start heart rate, the step
// between stages, the end heart rate, the minutes of each stage, the range,
// how far the heart rate may stray from each stage's, and the expected
// aerobic threshold, around which the stages are closer together.
func (s *Service) WorkoutHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		values := make([]int32, 6)
		for i, p := range []workoutParam{startParam, stepParam, endParam, minutesParam, rangeParam, thresholdParam} {
			v, err := p.get(r)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			values[i] = int32(v)
		}
		stages := plans.LactateTestStages{
			StartBPM: values[0], StepBPM: values[1], EndBPM: values[2],
			StageMinutes: values[3], RangeBPM: values[4], ThresholdBPM: values[5],
		}
		if stages.EndBPM < stages.StartBPM {
			http.Error(w, "end must be at least start", http.StatusBadRequest)
			return
		}
		workout := testWorkout(stages)
		if len(workout.Steps) > fit.MaxWorkoutSteps {
			http.Error(w, fmt.Sprintf("too many stages for a watch: at most %d; raise the step or narrow the heart rates", (fit.MaxWorkoutSteps-2)/2), http.StatusBadRequest)
			return
		}
		var buf bytes.Buffer
		if err := fit.WriteWorkout(&buf, workout, time.Now()); err != nil {
			s.report.Report(r.Context(), "Failed to write a lactate test workout", err)
			http.Error(w, "failed to write the workout", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/vnd.ant.fit")
		w.Header().Set("Content-Disposition", `attachment; filename="lactate-test.fit"`)
		_, _ = w.Write(buf.Bytes())
	})
}

// testWorkout returns the steps of a lactate test: a warm-up, then each
// stage, held in its heart rate range, followed by an open step to sample
// lactate in, ending at the lap button, and a cool-down.
func testWorkout(stages plans.LactateTestStages) *fit.Workout {
	steps := []fit.WorkoutStep{{
		Name:      "Warm up",
		Notes:     fmt.Sprintf("Very easy, below %d bpm", stages.StartBPM),
		Duration:  workoutWarmUp,
		Intensity: fit.IntensityWarmUp,
	}}
	for _, stage := range stages.Stages() {
		low, high := int(stage.BPM-stage.RangeBPM), int(stage.BPM+stage.RangeBPM)
		steps = append(steps,
			fit.WorkoutStep{
				Name:            fmt.Sprintf("%d bpm", stage.BPM),
				Notes:           fmt.Sprintf("Hold %d-%d bpm, then take a sample", low, high),
				Duration:        time.Duration(stages.StageMinutes) * time.Minute,
				MinHeartRateBPM: low,
				MaxHeartRateBPM: high,
				Intensity:       fit.IntensityActive,
			},
			fit.WorkoutStep{
				Name:      "Sample",
				Notes:     "Measure your lactate, then press lap",
				Intensity: fit.IntensityRest,
			},
		)
	}
	steps = append(steps, fit.WorkoutStep{
		Name:      "Cool down",
		Notes:     "Easy, until you press lap",
		Intensity: fit.IntensityCoolDown,
	})
	return &fit.Workout{Name: "Lactate test", Sport: fit.SportRunning, Steps: steps}
}
