package services_test

import (
	"context"
	"slices"
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
