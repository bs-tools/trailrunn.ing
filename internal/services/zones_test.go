package services_test

import (
	"context"
	"slices"
	"testing"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/bs-tools/trailrunn.ing/internal/activities"
	lactatev1 "github.com/bs-tools/trailrunn.ing/internal/gen/lactate/v1"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"
)

// zoneRanges returns the zones' heart rates, as [low, high] pairs.
func zoneRanges(zones []*lactatev1.HeartRateZone) [][2]int32 {
	ranges := make([][2]int32, len(zones))
	for i, z := range zones {
		ranges[i] = [2]int32{z.GetLowBpm(), z.GetHighBpm()}
	}
	return ranges
}

func TestHeartRateZones(t *testing.T) {
	t.Parallel()
	e := newEnv(t)
	ctx := context.Background()
	get := func() *lactatev1.LactateProfile {
		t.Helper()
		got, err := e.lactate.GetLactateProfile(ctx, &lactatev1.GetLactateProfileRequest{})
		if err != nil {
			t.Fatalf("GetLactateProfile: %v", err)
		}
		return got.GetProfile()
	}
	if p := get(); len(p.GetZones()) != 0 || p.GetEstimatedMaxHeartRateBpm() != 0 {
		t.Errorf("got profile %v without a test or activities, want no zones", p)
	}

	// A test that finds both thresholds: 2 mmol/L at 150 bpm, and 4 at 170.
	if _, err := e.lactate.CreateLactateTest(ctx, lactatev1.CreateLactateTestRequest_builder{
		Test: lactatev1.LactateTest_builder{Date: time.Now().Format(time.DateOnly), Stages: stages(
			[]int32{130, 140, 150, 160, 170}, []float64{1.0, 1.5, 2.0, 3.0, 4.0},
		)}.Build(),
	}.Build()); err != nil {
		t.Fatalf("CreateLactateTest: %v", err)
	}
	// Without a maximum heart rate, zone 4 goes up to 220 bpm.
	p := get()
	if got, want := zoneRanges(p.GetZones()), [][2]int32{{120, 135}, {135, 150}, {150, 170}, {170, 220}}; !slices.Equal(got, want) {
		t.Errorf("got zones %v, want %v", got, want)
	}
	if p.GetAnaerobicThresholdHeartRateBpm() != 170 || p.GetZones()[0].GetDescription() == "" {
		t.Errorf("got profile %v, want the anaerobic threshold and described zones", p)
	}

	// The maximum is estimated from the past year's activities, without
	// sensor glitches.
	for i, bpm := range []float64{186, 241} {
		start := time.Now().AddDate(0, 0, -10-i)
		if _, _, err := e.activities.UpsertActivity(ctx, e.userID, &activities.ActivitySummary{
			StravaID: int64(500 + i), Name: "Run", SportType: "Run", StartTime: start,
			StartDateLocal: start.Truncate(24 * time.Hour), Timezone: "UTC", MaxHeartrateBpm: bpm, RawJSON: []byte("{}"),
		}); err != nil {
			t.Fatal(err)
		}
	}
	if p := get(); p.GetEstimatedMaxHeartRateBpm() != 186 || p.GetZones()[3].GetHighBpm() != 186 || p.HasMaxHeartRateBpm() {
		t.Errorf("got profile %v, want zone 4 up to the estimated 186 bpm", p)
	}

	// The user's own maximum replaces it, and setting it alone keeps the
	// aerobic threshold.
	set, err := e.lactate.SetLactateProfile(ctx, lactatev1.SetLactateProfileRequest_builder{
		Profile:    lactatev1.LactateProfile_builder{MaxHeartRateBpm: proto.Int32(192)}.Build(),
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"max_heart_rate_bpm"}},
	}.Build())
	if err != nil {
		t.Fatalf("SetLactateProfile: %v", err)
	}
	if p := set.GetProfile(); p.GetMaxHeartRateBpm() != 192 || p.GetAerobicThresholdHeartRateBpm() != 150 || p.GetZones()[3].GetHighBpm() != 192 {
		t.Errorf("got profile %v, want 192 bpm with the threshold kept", p)
	}
	// Setting the threshold alone keeps the maximum, and moves the zones.
	set, err = e.lactate.SetLactateProfile(ctx, lactatev1.SetLactateProfileRequest_builder{
		Profile:    lactatev1.LactateProfile_builder{AerobicThresholdHeartRateBpm: proto.Int32(140)}.Build(),
		UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"aerobicThresholdHeartRateBpm"}},
	}.Build())
	if err != nil {
		t.Fatalf("SetLactateProfile: %v", err)
	}
	if p := set.GetProfile(); p.GetMaxHeartRateBpm() != 192 || p.GetZones()[0].GetLowBpm() != 112 || p.GetZones()[2].GetLowBpm() != 140 {
		t.Errorf("got profile %v, want the zones from 140 bpm with the maximum kept", p)
	}
	for _, req := range []*lactatev1.SetLactateProfileRequest{
		lactatev1.SetLactateProfileRequest_builder{Profile: lactatev1.LactateProfile_builder{MaxHeartRateBpm: proto.Int32(90)}.Build()}.Build(),
		lactatev1.SetLactateProfileRequest_builder{UpdateMask: &fieldmaskpb.FieldMask{Paths: []string{"zones"}}}.Build(),
	} {
		_, err := e.lactate.SetLactateProfile(ctx, req)
		wantCode(t, err, connect.CodeInvalidArgument)
	}
}
