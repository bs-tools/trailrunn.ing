package services_test

import (
	"context"
	"slices"
	"strings"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	geov1 "github.com/bs-tools/trailrunn.ing/internal/gen/geo/v1"
	plansv1 "github.com/bs-tools/trailrunn.ing/internal/gen/plans/v1"
	"google.golang.org/protobuf/proto"
)

// gearNames returns the gear of items.
func gearNames(items []*plansv1.GearItem) []plansv1.Gear {
	gear := make([]plansv1.Gear, len(items))
	for i, item := range items {
		gear[i] = item.GetGear()
	}
	return gear
}

func TestGearAdvice(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	today := time.Now().UTC().Truncate(24 * time.Hour)
	date := today.AddDate(0, 0, 2).Format(time.DateOnly)
	plan, err := e.plans.CreateTrainingPlan(ctx, plansv1.CreateTrainingPlanRequest_builder{
		Plan: plansv1.TrainingPlan_builder{Name: "Build", StartDate: today.Format(time.DateOnly), EndDate: today.AddDate(0, 0, 30).Format(time.DateOnly)}.Build(),
	}.Build())
	if err != nil {
		t.Fatal(err)
	}
	// At UTC, from 05:00 for 90 minutes: it feels like 10 to 12 °C, on 25 cm
	// of snow, see fakeMapData.HourlyForecast.
	location := geov1.LatLng_builder{Lat: 0.3, Lng: 0.01}.Build()
	created, err := e.plans.CreateWorkouts(ctx, plansv1.CreateWorkoutsRequest_builder{
		PlanId: plan.GetPlan().GetId(),
		Workouts: []*plansv1.Workout{
			plansv1.Workout_builder{Date: date, StartTime: "05:00", Title: "Early", Location: location, Run: plansv1.RunWorkout_builder{
				Intensity: plansv1.RunIntensity_RUN_INTENSITY_BASE, TargetDurationSeconds: 5400,
			}.Build()}.Build(),
			plansv1.Workout_builder{Date: date, Title: "Gym", Strength: plansv1.StrengthWorkout_builder{}.Build()}.Build(),
		},
	}.Build())
	if err != nil {
		t.Fatalf("CreateWorkouts: %v", err)
	}
	advice := func() *plansv1.GearAdvice {
		t.Helper()
		got, err := e.plans.GetWorkoutWeather(ctx, plansv1.GetWorkoutWeatherRequest_builder{WorkoutId: created.GetWorkouts()[0].GetId()}.Build())
		if err != nil {
			t.Fatalf("GetWorkoutWeather: %v", err)
		}
		return got.GetWeather().GetGear()
	}

	// Without a gear profile, any gear is suggested: shorts from 10 °C, and
	// waterproof shoes for the snow.
	got := advice()
	if wear := gearNames(got.GetWear()); !slices.Contains(wear, plansv1.Gear_GEAR_SHORTS) || !slices.Contains(wear, plansv1.Gear_GEAR_WATERPROOF_SHOES) {
		t.Errorf("got wear %v, want shorts and waterproof shoes", wear)
	}
	if got.GetBasis() != "Feels like 10 to 12 °C, snow on the ground" {
		t.Errorf("got basis %q", got.GetBasis())
	}

	// With the user's gear and their temperature for shorts.
	profile, err := e.plans.SetGearProfile(ctx, plansv1.SetGearProfileRequest_builder{Profile: plansv1.GearProfile_builder{
		Gear: []plansv1.Gear{
			plansv1.Gear_GEAR_TIGHTS, plansv1.Gear_GEAR_TIGHTS, plansv1.Gear_GEAR_T_SHIRT, plansv1.Gear_GEAR_TRAIL_SHOES, plansv1.Gear_GEAR_RAIN_JACKET,
		},
		ShortsMinApparentCelsius: proto.Float64(15),
	}.Build()}.Build())
	if err != nil {
		t.Fatalf("SetGearProfile: %v", err)
	}
	if p := profile.GetProfile(); len(p.GetGear()) != 4 || p.GetShortsMinApparentCelsius() != 15 || !p.HasUpdateTime() {
		t.Errorf("got profile %v, want the gear once each", p)
	}
	got = advice()
	wear := gearNames(got.GetWear())
	if !slices.Equal(wear, []plansv1.Gear{plansv1.Gear_GEAR_TIGHTS, plansv1.Gear_GEAR_T_SHIRT, plansv1.Gear_GEAR_TRAIL_SHOES, plansv1.Gear_GEAR_RUNNING_VEST}) {
		t.Errorf("got wear %v, want tights, a t-shirt, trail shoes for the snow, and a vest", wear)
	}
	if vest := got.GetWear()[3]; !vest.GetMissing() {
		t.Errorf("got vest %v, want it missing", vest)
	}
	if stored, err := e.plans.GetGearProfile(ctx, &plansv1.GetGearProfileRequest{}); err != nil || len(stored.GetProfile().GetGear()) != 4 {
		t.Errorf("GetGearProfile: got %v, %v", stored.GetProfile(), err)
	}

	// Out of range and unknown values are refused.
	for _, p := range []*plansv1.GearProfile{
		plansv1.GearProfile_builder{ShortsMinApparentCelsius: proto.Float64(40)}.Build(),
		plansv1.GearProfile_builder{Gear: []plansv1.Gear{plansv1.Gear_GEAR_UNSPECIFIED}}.Build(),
		plansv1.GearProfile_builder{Gear: []plansv1.Gear{plansv1.Gear(999)}}.Build(),
	} {
		_, err := e.plans.SetGearProfile(ctx, plansv1.SetGearProfileRequest_builder{Profile: p}.Build())
		wantCode(t, err, connect.CodeInvalidArgument)
	}

	// Strength workouts have no weather, and so no gear.
	if got, err := e.plans.GetWorkoutWeather(ctx, plansv1.GetWorkoutWeatherRequest_builder{WorkoutId: created.GetWorkouts()[1].GetId()}.Build()); err != nil || got.GetWeather().HasGear() {
		t.Errorf("got gear %v, %v for a strength workout", got.GetWeather().GetGear(), err)
	}
}

func TestGearFeedback(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	today := time.Now().UTC().Truncate(24 * time.Hour)
	plan, err := e.plans.CreateTrainingPlan(ctx, plansv1.CreateTrainingPlanRequest_builder{
		Plan: plansv1.TrainingPlan_builder{Name: "Build", StartDate: today.AddDate(0, 0, -10).Format(time.DateOnly), EndDate: today.AddDate(0, 0, 30).Format(time.DateOnly)}.Build(),
	}.Build())
	if err != nil {
		t.Fatal(err)
	}
	// Two past runs to rate, and one to come, from 05:00 for 90 minutes:
	// it feels like 10 to 12 °C, see fakeMapData.HourlyForecast.
	location := geov1.LatLng_builder{Lat: 0.3, Lng: 0.01}.Build()
	run := func(days int) *plansv1.Workout {
		return plansv1.Workout_builder{Date: today.AddDate(0, 0, days).Format(time.DateOnly), StartTime: "05:00", Title: "Run", Location: location, Run: plansv1.RunWorkout_builder{
			Intensity: plansv1.RunIntensity_RUN_INTENSITY_BASE, TargetDurationSeconds: 5400,
		}.Build()}.Build()
	}
	created, err := e.plans.CreateWorkouts(ctx, plansv1.CreateWorkoutsRequest_builder{
		PlanId: plan.GetPlan().GetId(), Workouts: []*plansv1.Workout{run(-3), run(-2), run(2)},
	}.Build())
	if err != nil {
		t.Fatalf("CreateWorkouts: %v", err)
	}
	ws := created.GetWorkouts()
	feel := func(area plansv1.GearArea, f plansv1.GearFeel) *plansv1.GearAreaFeel {
		return plansv1.GearAreaFeel_builder{Area: area, Feel: f}.Build()
	}
	set := func(workout *plansv1.Workout, notes string, feels ...*plansv1.GearAreaFeel) (*plansv1.SetGearFeedbackResponse, error) {
		return e.plans.SetGearFeedback(ctx, plansv1.SetGearFeedbackRequest_builder{
			WorkoutId: workout.GetId(),
			Feedback:  plansv1.GearFeedback_builder{Feels: feels, Notes: notes}.Build(),
		}.Build())
	}
	advice := func() *plansv1.GearAdvice {
		t.Helper()
		got, err := e.plans.GetWorkoutWeather(ctx, plansv1.GetWorkoutWeatherRequest_builder{WorkoutId: ws[2].GetId()}.Build())
		if err != nil {
			t.Fatalf("GetWorkoutWeather: %v", err)
		}
		return got.GetWeather().GetGear()
	}
	if wear := gearNames(advice().GetWear()); !slices.Contains(wear, plansv1.Gear_GEAR_SHORTS) || slices.Contains(wear, plansv1.Gear_GEAR_LINER_GLOVES) {
		t.Fatalf("got wear %v before feedback, want shorts and no gloves", wear)
	}

	// Legs and hands were too cold on both runs: they're now dressed 3 °C
	// colder, so for 7 °C: tights and liner gloves.
	if _, err := set(ws[0], "", feel(plansv1.GearArea_GEAR_AREA_LEGS, plansv1.GearFeel_GEAR_FEEL_TOO_COLD), feel(plansv1.GearArea_GEAR_AREA_HANDS, plansv1.GearFeel_GEAR_FEEL_TOO_COLD)); err != nil {
		t.Fatalf("SetGearFeedback: %v", err)
	}
	resp, err := set(ws[1], " Cold hands at the start ",
		feel(plansv1.GearArea_GEAR_AREA_HANDS, plansv1.GearFeel_GEAR_FEEL_TOO_COLD),
		feel(plansv1.GearArea_GEAR_AREA_LEGS, plansv1.GearFeel_GEAR_FEEL_TOO_COLD),
		feel(plansv1.GearArea_GEAR_AREA_BODY, plansv1.GearFeel_GEAR_FEEL_JUST_RIGHT))
	if err != nil {
		t.Fatalf("SetGearFeedback: %v", err)
	}
	if f := resp.GetFeedback(); f.GetNotes() != "Cold hands at the start" || len(f.GetFeels()) != 3 || f.GetFeels()[0].GetArea() != plansv1.GearArea_GEAR_AREA_LEGS {
		t.Errorf("got feedback %v, want three areas in order, and trimmed notes", f)
	}
	comfort := resp.GetComfort()
	if len(comfort) != 3 || comfort[0].GetOffsetCelsius() != 3 || comfort[1].GetOffsetCelsius() != 0 || comfort[2].GetRatings() != 2 {
		t.Errorf("got comfort %v, want legs and hands 3 °C colder and the body as it is", comfort)
	}
	got := advice()
	if wear := gearNames(got.GetWear()); !slices.Contains(wear, plansv1.Gear_GEAR_TIGHTS) || !slices.Contains(wear, plansv1.Gear_GEAR_LINER_GLOVES) {
		t.Errorf("got wear %v after feedback, want tights and liner gloves", wear)
	}
	if want := "Feels like 10 to 12 °C, snow on the ground; dressed 3 °C colder for your legs, 3 °C colder for your hands, from your feedback"; got.GetBasis() != want {
		t.Errorf("got basis %q, want %q", got.GetBasis(), want)
	}
	if p, err := e.plans.GetGearProfile(ctx, &plansv1.GetGearProfileRequest{}); err != nil || len(p.GetProfile().GetComfort()) != 3 {
		t.Errorf("GetGearProfile: got %v, %v, want what feedback taught", p.GetProfile(), err)
	}
	stored, err := e.plans.GetGearFeedback(ctx, plansv1.GetGearFeedbackRequest_builder{WorkoutId: ws[1].GetId()}.Build())
	if err != nil || len(stored.GetFeedback().GetFeels()) != 3 {
		t.Errorf("GetGearFeedback: got %v, %v", stored.GetFeedback(), err)
	}

	// Feedback without ratings or notes is deleted, and taught nothing.
	resp, err = set(ws[1], "")
	if err != nil || resp.HasFeedback() || resp.GetComfort()[0].GetOffsetCelsius() != 1.5 {
		t.Errorf("got %v, %v after deleting, want the first run's feedback only", resp, err)
	}
	if stored, err := e.plans.GetGearFeedback(ctx, plansv1.GetGearFeedbackRequest_builder{WorkoutId: ws[1].GetId()}.Build()); err != nil || stored.HasFeedback() {
		t.Errorf("GetGearFeedback after deleting: got %v, %v", stored.GetFeedback(), err)
	}

	// A wet run's ratings are learned for wet runs only: the next run is
	// dry, so its advice is as before.
	resp, err = e.plans.SetGearFeedback(ctx, plansv1.SetGearFeedbackRequest_builder{
		WorkoutId: ws[1].GetId(),
		Feedback: plansv1.GearFeedback_builder{
			Wet:   true,
			Feels: []*plansv1.GearAreaFeel{feel(plansv1.GearArea_GEAR_AREA_BODY, plansv1.GearFeel_GEAR_FEEL_TOO_COLD)},
		}.Build(),
	}.Build())
	if err != nil || !resp.GetFeedback().GetWet() {
		t.Fatalf("SetGearFeedback: got %v, %v, want it wet", resp.GetFeedback(), err)
	}
	if c := resp.GetComfort(); len(c) != 3 || !c[2].GetWet() || c[2].GetArea() != plansv1.GearArea_GEAR_AREA_BODY || c[2].GetOffsetCelsius() != 1.5 {
		t.Errorf("got comfort %v, want the dry legs and hands, and the wet body", c)
	}
	if got := advice(); strings.Contains(got.GetBasis(), "body") {
		t.Errorf("got basis %q on a dry run, want no change for the body", got.GetBasis())
	}

	// Ratings are of known parts of the body, once each.
	for name, feels := range map[string][]*plansv1.GearAreaFeel{
		"no area":     {feel(plansv1.GearArea_GEAR_AREA_UNSPECIFIED, plansv1.GearFeel_GEAR_FEEL_TOO_COLD)},
		"no feel":     {feel(plansv1.GearArea_GEAR_AREA_LEGS, plansv1.GearFeel_GEAR_FEEL_UNSPECIFIED)},
		"rated twice": {feel(plansv1.GearArea_GEAR_AREA_LEGS, plansv1.GearFeel_GEAR_FEEL_TOO_COLD), feel(plansv1.GearArea_GEAR_AREA_LEGS, plansv1.GearFeel_GEAR_FEEL_TOO_WARM)},
	} {
		if _, err := set(ws[0], "", feels...); connect.CodeOf(err) != connect.CodeInvalidArgument {
			t.Errorf("%s: got %v, want InvalidArgument", name, err)
		}
	}
	// Only the user's own workouts.
	_, err = e.plans.SetGearFeedback(ctx, plansv1.SetGearFeedbackRequest_builder{
		WorkoutId: "00000000-0000-0000-0000-000000000001",
		Feedback:  plansv1.GearFeedback_builder{Notes: "Warm"}.Build(),
	}.Build())
	wantCode(t, err, connect.CodeNotFound)
}
