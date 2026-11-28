package garmin

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bs-tools/trailrunn.ing/internal/fit"
	"github.com/bs-tools/trailrunn.ing/internal/plans"
)

func TestStepDescription(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name     string
		exercise *plans.PlannedExercise
		withName bool
		want     string
	}{
		{"nothing", &plans.PlannedExercise{ExerciseName: "Push-up"}, false, ""},
		{
			"everything",
			&plans.PlannedExercise{ExerciseName: "Split squat", Tempo: "3 s down", TargetEffort: 6, Notes: "Front foot flat."},
			true,
			"Split squat. Tempo: 3 s down. Effort 6/10. Front foot flat",
		},
		{"catalog exercise", &plans.PlannedExercise{ExerciseName: "Push-up", Tempo: "2 s down", Notes: "Elbows in"}, false, "Tempo: 2 s down. Elbows in"},
	} {
		if got := stepDescription(tc.exercise, tc.withName, 0); got != tc.want {
			t.Errorf("%s: got %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestRunSteps(t *testing.T) {
	t.Parallel()
	w := &plans.Workout{Type: plans.WorkoutTypeIntensity, Title: "Hill repeats", RunBlocks: []plans.RunBlock{
		{Repeats: 1, Steps: []plans.RunStep{{Kind: plans.RunStepWarmUp, DurationSeconds: 900, HeartRate: &plans.HeartRateTarget{Reference: plans.HeartRateAerobicThreshold, LowBPM: -20, HighBPM: 0}}}},
		{Repeats: 6, Steps: []plans.RunStep{
			{Kind: plans.RunStepRun, DurationSeconds: 180, HeartRate: &plans.HeartRateTarget{Reference: plans.HeartRateAnaerobicThreshold, LowBPM: -5, HighBPM: 3}, Notes: "Uphill"},
			{Kind: plans.RunStepRecovery, DistanceMeters: 400, Pace: &plans.PaceTarget{FastSecondsPerKilometer: 300, SlowSecondsPerKilometer: 400}},
		}},
		{Repeats: 1, Steps: []plans.RunStep{{Kind: plans.RunStepCoolDown}}},
	}}
	gw, ok := trainingWorkout(w, thresholds{aerobic: 150}, "")
	if !ok {
		t.Fatal("not a Garmin workout")
	}
	steps := gw.Segments[0].Steps
	if len(steps) != 3 || gw.CountSteps() != 5 {
		t.Fatalf("got %d top-level steps and %d in all, want 3 and 5: %+v", len(steps), gw.CountSteps(), steps)
	}
	warmUp, repeat, coolDown := steps[0], steps[1], steps[2]
	if warmUp.Intensity != "WARMUP" || warmUp.DurationType != "TIME" || warmUp.DurationValue != 900 ||
		warmUp.TargetType != "HEART_RATE" || warmUp.TargetValueLow != 130 || warmUp.TargetValueHigh != 150 {
		t.Errorf("unexpected warm-up %+v", warmUp)
	}
	if repeat.Type != "WorkoutRepeatStep" || repeat.RepeatValue != 6 || len(repeat.Steps) != 2 {
		t.Fatalf("unexpected repeat %+v", repeat)
	}
	// Without an anaerobic threshold, its target is left open and described.
	if hard := repeat.Steps[0]; hard.TargetType != "OPEN" || hard.Intensity != "ACTIVE" ||
		!strings.Contains(hard.Description, "Uphill. Heart rate -5 to +3 bpm from your anaerobic threshold") {
		t.Errorf("unexpected hard step %+v", hard)
	}
	// Paces are speeds, the slow pace the low one.
	if easy := repeat.Steps[1]; easy.DurationType != "DISTANCE" || easy.DurationValue != 400 || easy.TargetType != "PACE" ||
		easy.TargetValueLow != 2.5 || easy.TargetValueHigh != 3.333 {
		t.Errorf("unexpected recovery %+v", easy)
	}
	if coolDown.Intensity != "COOLDOWN" || coolDown.DurationType != "OPEN" {
		t.Errorf("unexpected cool-down %+v", coolDown)
	}
	// Step orders count every step, repeats included.
	if warmUp.StepOrder != 1 || repeat.StepOrder != 2 || repeat.Steps[0].StepOrder != 3 || coolDown.StepOrder != 5 {
		t.Errorf("unexpected step orders %d %d %d %d", warmUp.StepOrder, repeat.StepOrder, repeat.Steps[0].StepOrder, coolDown.StepOrder)
	}
}

func TestStrengthWarmUpSteps(t *testing.T) {
	t.Parallel()
	w := &plans.Workout{Type: plans.WorkoutTypeStrength, Title: "Legs", Exercises: []*plans.PlannedExercise{
		{ExerciseName: "Leg swings", Group: "Warm-up", GroupKind: plans.GroupWarmUp, Sets: 1},
		{ExerciseName: "Split squat", Group: "Main", Sets: 1, Reps: 8},
		{ExerciseName: "Hip flexor stretch", Group: "Cool-down", GroupKind: plans.GroupCoolDown, Sets: 1, HoldSeconds: 30},
	}}
	gw, ok := trainingWorkout(w, thresholds{}, "")
	if !ok {
		t.Fatal("not a Garmin workout")
	}
	steps := gw.Segments[0].Steps
	// The stretch's hold waits for the lap button.
	if len(steps) != 4 {
		t.Fatalf("got %d steps, want 4: %+v", len(steps), steps)
	}
	for i, want := range []string{"WARMUP", "ACTIVE", "REST", "COOLDOWN"} {
		if steps[i].Intensity != want {
			t.Errorf("step %d has intensity %q, want %q", i, steps[i].Intensity, want)
		}
	}
	if steps[0].DurationType != "OPEN" {
		t.Errorf("the warm-up until done is %q, want OPEN", steps[0].DurationType)
	}
}

func TestStrengthHoldSteps(t *testing.T) {
	t.Parallel()
	w := &plans.Workout{Type: plans.WorkoutTypeStrength, Title: "Core", Exercises: []*plans.PlannedExercise{
		{ExerciseName: "Side plank", ExerciseEachSide: true, Sets: 1, HoldSeconds: 30, RestSeconds: 45},
		{ExerciseName: "Dead bug", ExerciseEachSide: true, Sets: 1, Reps: 8},
	}}
	gw, ok := trainingWorkout(w, thresholds{}, "")
	if !ok {
		t.Fatal("not a Garmin workout")
	}
	steps := gw.Segments[0].Steps
	type step struct {
		intensity, duration string
		value               float64
	}
	var got []step
	for _, s := range steps {
		got = append(got, step{s.Intensity, s.DurationType, s.DurationValue})
	}
	// Each side's hold waits for the lap button, then the rest, then the
	// dead bugs with reps per side.
	want := []step{
		{"REST", "OPEN", 0},
		{"ACTIVE", "TIME", 30},
		{"REST", "OPEN", 0},
		{"ACTIVE", "TIME", 30},
		{"REST", "TIME", 45},
		{"ACTIVE", "REPS", 8},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got steps %v, want %v", got, want)
	}
	if !strings.Contains(steps[3].Description, "Side 2 of 2") || !strings.Contains(steps[5].Description, "Each side") {
		t.Errorf("got descriptions %q and %q", steps[3].Description, steps[5].Description)
	}
	// The steps the watch records sets of are the same as those sent.
	if layout := plans.StrengthStepExercises(w.Exercises); len(layout) != len(steps) || layout[1] != 0 || layout[3] != plans.StepSecondSide || layout[5] != 1 {
		t.Errorf("got step exercises %v for %d steps", layout, len(steps))
	}

	// A watch records a set for each side: they're one set.
	log := strengthLogFromSets(w, &fit.Activity{Sets: []fit.Set{
		{Active: true, HasWorkoutStep: true, WorkoutStep: 1, Duration: 30 * time.Second},
		{Active: true, HasWorkoutStep: true, WorkoutStep: 3, Duration: 31 * time.Second},
		{Active: true, HasWorkoutStep: true, WorkoutStep: 5, Reps: 8},
	}})
	if log == nil || len(log.Exercises) != 2 || len(log.Exercises[0].Sets) != 1 || log.Exercises[0].Sets[0].HoldSeconds != 30 || len(log.Exercises[1].Sets) != 1 {
		t.Errorf("got log %+v, want one side plank set and one set of dead bugs", log)
	}
}

func TestStrengthUnevenSteps(t *testing.T) {
	t.Parallel()
	w := &plans.Workout{Type: plans.WorkoutTypeStrength, Title: "Rehab", Exercises: []*plans.PlannedExercise{
		// More on the weaker left leg, and a longer hold on the left side.
		{ExerciseName: "Single-leg squat", ExerciseEachSide: true, Sets: 1, Reps: 8, LeftReps: 10},
		{ExerciseName: "Side plank", ExerciseEachSide: true, Sets: 1, HoldSeconds: 30, LeftHoldSeconds: 45},
	}}
	gw, ok := trainingWorkout(w, thresholds{}, "")
	if !ok {
		t.Fatal("not a Garmin workout")
	}
	steps := gw.Segments[0].Steps
	type step struct {
		intensity, duration string
		value               float64
		side                string
	}
	var got []step
	for _, s := range steps {
		side := ""
		for _, name := range []string{"Left side", "Right side"} {
			if strings.Contains(s.Description, name) {
				side = name
			}
		}
		got = append(got, step{s.Intensity, s.DurationType, s.DurationValue, side})
	}
	// Each side is its own step, the left first, with its own reps or hold.
	want := []step{
		{"ACTIVE", "REPS", 10, "Left side"},
		{"ACTIVE", "REPS", 8, "Right side"},
		{"REST", "OPEN", 0, "Left side"},
		{"ACTIVE", "TIME", 45, "Left side"},
		{"REST", "OPEN", 0, "Right side"},
		{"ACTIVE", "TIME", 30, "Right side"},
	}
	if !slices.Equal(got, want) {
		t.Errorf("got steps %v, want %v", got, want)
	}
	if layout := plans.StrengthStepExercises(w.Exercises); !slices.Equal(layout, []int{0, plans.StepSecondSide, plans.StepNoExercise, 1, plans.StepNoExercise, plans.StepSecondSide}) {
		t.Errorf("got step exercises %v", layout)
	}

	// The watch's sets of each side are one set, with the left side's reps
	// and hold where they differ from the right's.
	log := strengthLogFromSets(w, &fit.Activity{Sets: []fit.Set{
		{Active: true, HasWorkoutStep: true, WorkoutStep: 0, Reps: 10},
		{Active: true, HasWorkoutStep: true, WorkoutStep: 1, Reps: 7},
		{Active: true, HasWorkoutStep: true, WorkoutStep: 3, Duration: 45 * time.Second},
		{Active: true, HasWorkoutStep: true, WorkoutStep: 5, Duration: 45 * time.Second},
	}})
	if log == nil || len(log.Exercises) != 2 {
		t.Fatalf("got log %+v, want both exercises", log)
	}
	if sets := log.Exercises[0].Sets; len(sets) != 1 || sets[0].Reps != 7 || sets[0].LeftReps != 10 {
		t.Errorf("got squat sets %+v, want one of 7 reps, 10 on the left", sets)
	}
	// Both sides held for as long, so the set has no left side's hold.
	if sets := log.Exercises[1].Sets; len(sets) != 1 || sets[0].HoldSeconds != 45 || sets[0].LeftHoldSeconds != 0 {
		t.Errorf("got plank sets %+v, want one of 45 s on both sides", sets)
	}
}

func TestPlainRunHeartRate(t *testing.T) {
	t.Parallel()
	both := thresholds{aerobic: 149, anaerobic: 170}
	for _, tc := range []struct {
		name              string
		workout           plans.Workout
		thresholds        thresholds
		wantLow, wantHigh float64
		// What the step's description has, empty for no description.
		wantNote string
	}{
		// A base run is in zones 1 and 2 by default: from 80% of the
		// aerobic threshold's heart rate up to it.
		{name: "base run", workout: plans.Workout{Type: plans.WorkoutTypeBase, TargetDurationSeconds: 3600}, thresholds: both, wantLow: 119, wantHigh: 149, wantNote: "Zones 1 to 2: 119 to 149 bpm"},
		{name: "without an anaerobic threshold", workout: plans.Workout{Type: plans.WorkoutTypeBase}, thresholds: thresholds{aerobic: 110}, wantLow: 88, wantHigh: 110, wantNote: "Zones 1 to 2"},
		// Without an aerobic threshold there are no zones, and a hard run
		// without structure has no one zone.
		{name: "no threshold", workout: plans.Workout{Type: plans.WorkoutTypeBase}},
		{name: "intensity run", workout: plans.Workout{Type: plans.WorkoutTypeIntensity}, thresholds: both},
		// The run's own range replaces the default: zones, also of a hard
		// run, or beats per minute.
		{name: "recovery run", workout: plans.Workout{Type: plans.WorkoutTypeBase, RunHeartRate: &plans.HeartRateTarget{LowZone: 1, HighZone: 1}}, thresholds: both, wantLow: 119, wantHigh: 134, wantNote: "Zone 1: 119 to 134 bpm"},
		{name: "tempo run", workout: plans.Workout{Type: plans.WorkoutTypeIntensity, RunHeartRate: &plans.HeartRateTarget{LowZone: 3, HighZone: 3}}, thresholds: both, wantLow: 149, wantHigh: 170, wantNote: "Zone 3: 149 to 170 bpm"},
		{name: "exact range", workout: plans.Workout{Type: plans.WorkoutTypeBase, RunHeartRate: &plans.HeartRateTarget{LowBPM: 125, HighBPM: 140}}, thresholds: both, wantLow: 125, wantHigh: 140},
		{name: "exact range without thresholds", workout: plans.Workout{Type: plans.WorkoutTypeBase, RunHeartRate: &plans.HeartRateTarget{LowBPM: 125, HighBPM: 140}}, wantLow: 125, wantHigh: 140},
		// Zone 3 ends at the anaerobic threshold: without it the run is
		// open, and says what's missing.
		{name: "zone 3 without an anaerobic threshold", workout: plans.Workout{Type: plans.WorkoutTypeIntensity, RunHeartRate: &plans.HeartRateTarget{LowZone: 2, HighZone: 3}}, thresholds: thresholds{aerobic: 149}, wantNote: "Zones 2 to 3; do a lactate test that finds your anaerobic threshold"},
		{name: "zones without thresholds", workout: plans.Workout{Type: plans.WorkoutTypeBase, RunHeartRate: &plans.HeartRateTarget{LowZone: 1, HighZone: 2}}, wantNote: "Zones 1 to 2; do a lactate test that finds your aerobic threshold"},
	} {
		gw, ok := trainingWorkout(&tc.workout, tc.thresholds, "")
		if !ok || len(gw.Segments[0].Steps) != 1 {
			t.Fatalf("%s: got %+v, want a workout of one step", tc.name, gw)
		}
		step := gw.Segments[0].Steps[0]
		wantType := "OPEN"
		if tc.wantHigh > 0 {
			wantType = "HEART_RATE"
		}
		if step.TargetType != wantType || step.TargetValueLow != tc.wantLow || step.TargetValueHigh != tc.wantHigh {
			t.Errorf("%s: got target %s %v-%v, want %s %v-%v", tc.name, step.TargetType, step.TargetValueLow, step.TargetValueHigh, wantType, tc.wantLow, tc.wantHigh)
		}
		if !strings.HasPrefix(step.Description, tc.wantNote) || (tc.wantNote == "") != (step.Description == "") {
			t.Errorf("%s: got description %q, want %q", tc.name, step.Description, tc.wantNote)
		}
	}
	// The duration is kept with the target.
	timed, _ := trainingWorkout(&plans.Workout{Type: plans.WorkoutTypeBase, TargetDurationSeconds: 3600}, thresholds{aerobic: 149}, "")
	if step := timed.Segments[0].Steps[0]; step.DurationType != "TIME" || step.DurationValue != 3600 {
		t.Errorf("got duration %s %v, want an hour", step.DurationType, step.DurationValue)
	}
	// A step of a structured run takes zones too, after its own notes.
	structured, _ := trainingWorkout(&plans.Workout{Type: plans.WorkoutTypeIntensity, RunBlocks: []plans.RunBlock{{Repeats: 1, Steps: []plans.RunStep{
		{Kind: plans.RunStepRun, DurationSeconds: 1200, Notes: "Steady", HeartRate: &plans.HeartRateTarget{LowZone: 3, HighZone: 4}},
	}}}}, both, "")
	if step := structured.Segments[0].Steps[0]; step.TargetValueLow != 149 || step.TargetValueHigh != 220 || step.Description != "Steady. Zones 3 to 4: 149 to 220 bpm" {
		t.Errorf("got step %+v, want zones 3 to 4 after its notes", step)
	}
}

func TestLactateTestSteps(t *testing.T) {
	t.Parallel()
	w := &plans.Workout{Type: plans.WorkoutTypeBase, Title: "Lactate test", LactateTest: true}
	for _, tc := range []struct {
		name              string
		start, threshold  int32
		firstBPM, lastBPM float64
		wantStages        int
	}{
		{name: "first test", firstBPM: 100, lastBPM: 180, wantStages: 9},
		// A later test starts where the last did, with enough stages to get
		// past the threshold.
		{name: "from the last test", start: 150, firstBPM: 150, lastBPM: 190, wantStages: 5},
		// With a threshold from the last test, it starts 30 bpm below it:
		// 115, 125, 135, 140, 145, 150, 155, 165 and 175.
		{name: "around the last threshold", start: 100, threshold: 143, firstBPM: 115, lastBPM: 175, wantStages: 9},
	} {
		gw, ok := trainingWorkout(w, thresholds{lactateStart: tc.start, lactateThreshold: tc.threshold}, "")
		if !ok {
			t.Fatalf("%s: not a Garmin workout", tc.name)
		}
		steps := gw.Segments[0].Steps
		if len(steps) != 2*tc.wantStages+2 {
			t.Fatalf("%s: got %d steps, want a warm-up, %d stages with a sample each, and a cool-down: %+v", tc.name, len(steps), tc.wantStages, steps)
		}
		if warmUp := steps[0]; warmUp.Intensity != "WARMUP" || warmUp.DurationType != "TIME" || warmUp.DurationValue != 900 {
			t.Errorf("%s: unexpected warm-up %+v", tc.name, warmUp)
		}
		first, sample, last := steps[1], steps[2], steps[len(steps)-3]
		if first.Intensity != "ACTIVE" || first.DurationType != "TIME" || first.DurationValue != 240 ||
			first.TargetType != "HEART_RATE" || first.TargetValueLow != tc.firstBPM-5 || first.TargetValueHigh != tc.firstBPM+5 {
			t.Errorf("%s: unexpected first stage %+v", tc.name, first)
		}
		if sample.Intensity != "REST" || sample.DurationType != "OPEN" || !strings.Contains(sample.Description, "lactate") {
			t.Errorf("%s: unexpected sample step %+v", tc.name, sample)
		}
		if last.TargetValueLow != tc.lastBPM-5 || last.TargetValueHigh != tc.lastBPM+5 {
			t.Errorf("%s: the last stage is %v-%v bpm, want around %v", tc.name, last.TargetValueLow, last.TargetValueHigh, tc.lastBPM)
		}
		if coolDown := steps[len(steps)-1]; coolDown.Intensity != "COOLDOWN" || coolDown.DurationType != "OPEN" {
			t.Errorf("%s: unexpected cool-down %+v", tc.name, coolDown)
		}
	}
}

func TestTrainingWorkoutCourse(t *testing.T) {
	w := &plans.Workout{
		Type: plans.WorkoutTypeBase, Title: "VK", Description: "Hike the steep climbs.",
		TargetDurationSeconds: 3600,
		RunFueling:            &plans.RunFueling{CarbsGramsPerHour: 60},
	}
	// The course comes first, then the fueling and the description.
	gw, ok := trainingWorkout(w, thresholds{}, "VK XXIV - Rule Britannia")
	if !ok {
		t.Fatal("not a Garmin workout")
	}
	want := "Course on your watch: VK XXIV - Rule Britannia\n\nFuel: 60 g carbs\n\nHike the steep climbs."
	if gw.Description != want {
		t.Errorf("got description %q, want %q", gw.Description, want)
	}
	// Without a course or fueling, it's just the description.
	w.RunFueling = nil
	if gw, _ := trainingWorkout(w, thresholds{}, ""); gw.Description != "Hike the steep climbs." {
		t.Errorf("got description %q, want only the workout's", gw.Description)
	}
}
