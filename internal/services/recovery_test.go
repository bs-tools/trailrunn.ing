package services_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	plansv1 "github.com/bs-tools/trailrunn.ing/internal/gen/plans/v1"
	stravav1 "github.com/bs-tools/trailrunn.ing/internal/gen/strava/v1"
	"github.com/bs-tools/trailrunn.ing/internal/strava"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

func TestRecoveryWorkouts(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	plan, err := e.plans.CreateTrainingPlan(ctx, plansv1.CreateTrainingPlanRequest_builder{
		Plan: plansv1.TrainingPlan_builder{Name: "Rehab", StartDate: "2026-11-02", EndDate: "2026-12-31"}.Build(),
	}.Build())
	if err != nil {
		t.Fatal(err)
	}
	planID := plan.GetPlan().GetId()
	create := func(items ...*plansv1.RecoveryItem) (*plansv1.Workout, error) {
		resp, err := e.plans.CreateWorkouts(ctx, plansv1.CreateWorkoutsRequest_builder{
			PlanId: planID,
			Workouts: []*plansv1.Workout{plansv1.Workout_builder{
				Date: "2026-11-10", Title: "PowerDot",
				Recovery: plansv1.RecoveryWorkout_builder{Items: items}.Build(),
			}.Build()},
		}.Build())
		if err != nil {
			return nil, err
		}
		return resp.GetWorkouts()[0], nil
	}
	ems := func(program plansv1.StimulationProgram, muscles ...plansv1.Muscle) *plansv1.RecoveryItem {
		return plansv1.RecoveryItem_builder{
			Modality:    plansv1.RecoveryModality_RECOVERY_MODALITY_ELECTRICAL_STIMULATION,
			Muscles:     muscles,
			Stimulation: plansv1.ElectricalStimulation_builder{Program: program}.Build(),
		}.Build()
	}

	// A physio's session: active recovery on both calves with two pods, then
	// TENS pain relief on the left glute medius at a prescribed intensity,
	// then a sauna.
	workout, err := create(
		plansv1.RecoveryItem_builder{
			Modality:        plansv1.RecoveryModality_RECOVERY_MODALITY_ELECTRICAL_STIMULATION,
			DurationSeconds: 25 * 60,
			Muscles:         []plansv1.Muscle{plansv1.Muscle_MUSCLE_CALVES},
			Stimulation: plansv1.ElectricalStimulation_builder{
				Program: plansv1.StimulationProgram_STIMULATION_PROGRAM_ACTIVE_RECOVERY,
				Pods:    2,
			}.Build(),
		}.Build(),
		plansv1.RecoveryItem_builder{
			Modality:        plansv1.RecoveryModality_RECOVERY_MODALITY_ELECTRICAL_STIMULATION,
			DurationSeconds: 30 * 60,
			Muscles:         []plansv1.Muscle{plansv1.Muscle_MUSCLE_HIP_ABDUCTORS, plansv1.Muscle_MUSCLE_HIP_ABDUCTORS},
			Side:            plansv1.RecoverySide_RECOVERY_SIDE_LEFT,
			Notes:           "  Pads above and behind the hip bone  ",
			Stimulation: plansv1.ElectricalStimulation_builder{
				Program:     plansv1.StimulationProgram_STIMULATION_PROGRAM_PAIN_RELIEF,
				Pods:        1,
				Intensity:   35,
				PainLevel:   proto.Int32(0),
				TensMassage: plansv1.TensMassage_TENS_MASSAGE_LOW_FREQUENCY,
			}.Build(),
		}.Build(),
		plansv1.RecoveryItem_builder{Modality: plansv1.RecoveryModality_RECOVERY_MODALITY_HEAT, DurationSeconds: 15 * 60}.Build(),
	)
	if err != nil {
		t.Fatal(err)
	}
	if workout.GetType() != plansv1.WorkoutType_WORKOUT_TYPE_RECOVERY {
		t.Errorf("type = %v, want RECOVERY", workout.GetType())
	}
	items := workout.GetRecovery().GetItems()
	if len(items) != 3 {
		t.Fatalf("got %d items, want 3", len(items))
	}
	tens := items[1]
	if s := tens.GetStimulation(); s.GetProgram() != plansv1.StimulationProgram_STIMULATION_PROGRAM_PAIN_RELIEF ||
		s.GetIntensity() != 35 || !s.HasPainLevel() || s.GetPainLevel() != 0 ||
		s.GetTensMassage() != plansv1.TensMassage_TENS_MASSAGE_LOW_FREQUENCY ||
		len(tens.GetMuscles()) != 1 || tens.GetSide() != plansv1.RecoverySide_RECOVERY_SIDE_LEFT ||
		tens.GetNotes() != "Pads above and behind the hip bone" {
		t.Errorf("unexpected TENS item %v", tens)
	}
	if items[0].GetStimulation().HasPainLevel() || items[2].HasStimulation() || items[2].GetDurationSeconds() != 15*60 {
		t.Errorf("unexpected items %v", items)
	}

	// Mistakes are explained.
	for _, tc := range []struct {
		name  string
		items []*plansv1.RecoveryItem
		want  string
	}{
		{"no items", nil, "at least one item"},
		{"no modality", []*plansv1.RecoveryItem{plansv1.RecoveryItem_builder{}.Build()}, "modality is required"},
		{"no pads", []*plansv1.RecoveryItem{ems(plansv1.StimulationProgram_STIMULATION_PROGRAM_MASSAGE)}, "where the pads go"},
		{"no program", []*plansv1.RecoveryItem{ems(plansv1.StimulationProgram_STIMULATION_PROGRAM_UNSPECIFIED, plansv1.Muscle_MUSCLE_QUADS)}, "program is required"},
		{"stimulation on a massage gun", []*plansv1.RecoveryItem{plansv1.RecoveryItem_builder{
			Modality:    plansv1.RecoveryModality_RECOVERY_MODALITY_PERCUSSION,
			Stimulation: plansv1.ElectricalStimulation_builder{Program: plansv1.StimulationProgram_STIMULATION_PROGRAM_MASSAGE}.Build(),
		}.Build()}, "only for ELECTRICAL_STIMULATION"},
		{"pain level on EMS", []*plansv1.RecoveryItem{plansv1.RecoveryItem_builder{
			Modality: plansv1.RecoveryModality_RECOVERY_MODALITY_ELECTRICAL_STIMULATION,
			Muscles:  []plansv1.Muscle{plansv1.Muscle_MUSCLE_QUADS},
			Stimulation: plansv1.ElectricalStimulation_builder{
				Program: plansv1.StimulationProgram_STIMULATION_PROGRAM_STRENGTH, PainLevel: proto.Int32(4),
			}.Build(),
		}.Build()}, "pain_level is only for"},
		{"massage style on period pain", []*plansv1.RecoveryItem{plansv1.RecoveryItem_builder{
			Modality: plansv1.RecoveryModality_RECOVERY_MODALITY_ELECTRICAL_STIMULATION,
			Muscles:  []plansv1.Muscle{plansv1.Muscle_MUSCLE_ABS},
			Stimulation: plansv1.ElectricalStimulation_builder{
				Program:     plansv1.StimulationProgram_STIMULATION_PROGRAM_PERIOD_PAIN_RELIEF,
				TensMassage: plansv1.TensMassage_TENS_MASSAGE_REFRESHING,
			}.Build(),
		}.Build()}, "tens_massage is only for the PAIN_RELIEF"},
		{"three pods", []*plansv1.RecoveryItem{plansv1.RecoveryItem_builder{
			Modality: plansv1.RecoveryModality_RECOVERY_MODALITY_ELECTRICAL_STIMULATION,
			Muscles:  []plansv1.Muscle{plansv1.Muscle_MUSCLE_QUADS},
			Stimulation: plansv1.ElectricalStimulation_builder{
				Program: plansv1.StimulationProgram_STIMULATION_PROGRAM_ACTIVE_RECOVERY, Pods: 3,
			}.Build(),
		}.Build()}, "pods must be"},
		{"other without notes", []*plansv1.RecoveryItem{plansv1.RecoveryItem_builder{
			Modality: plansv1.RecoveryModality_RECOVERY_MODALITY_OTHER,
		}.Build()}, "notes are required"},
	} {
		_, err := create(tc.items...)
		wantCode(t, err, connect.CodeInvalidArgument)
		if !strings.Contains(err.Error(), tc.want) {
			t.Errorf("%s: error %q should contain %q", tc.name, err, tc.want)
		}
	}

	// She marks it done, which keeps its items.
	done, err := e.plans.UpdateWorkout(ctx, plansv1.UpdateWorkoutRequest_builder{
		Workout: plansv1.Workout_builder{
			Id:       workout.GetId(),
			Recovery: plansv1.RecoveryWorkout_builder{Done: true}.Build(),
		}.Build(),
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"recovery.done"}},
	}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if !done.GetWorkout().GetRecovery().GetDone() || len(done.GetWorkout().GetRecovery().GetItems()) != 3 {
		t.Errorf("unexpected done workout %v", done.GetWorkout())
	}

	// Syncing activities doesn't link one to recovery work, though any
	// activity that isn't a run can complete a cross-training workout.
	yoga := &strava.Activity{
		ID: 2000, Name: "Yoga", SportType: "Yoga",
		StartDate: time.Date(2026, 11, 10, 18, 0, 0, 0, time.UTC), StartDateLocal: "2026-11-10T18:00:00Z",
		Timezone: "(GMT+00:00) Europe/London", MovingTime: 1800, ElapsedTime: 1800,
	}
	e.strava.upsertActivity(yoga)
	e.connectStrava()
	if _, err := e.stravaAPI.SyncStrava(ctx, stravav1.SyncStravaRequest_builder{}.Build()); err != nil {
		t.Fatalf("SyncStrava: %v", err)
	}
	e.waitForSync()
	got, err := e.plans.GetWorkout(ctx, plansv1.GetWorkoutRequest_builder{Id: workout.GetId()}.Build())
	if err != nil {
		t.Fatal(err)
	}
	if got.GetWorkout().GetCompletedActivityId() != "" {
		t.Errorf("recovery work was linked to activity %s", got.GetWorkout().GetCompletedActivityId())
	}
}
