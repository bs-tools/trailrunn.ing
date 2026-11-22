package plan

import (
	"context"
	"math"
	"strings"

	"connectrpc.com/connect/v2"
	"github.com/bs-tools/trailrunn.ing/internal/authn"
	plansv1 "github.com/bs-tools/trailrunn.ing/internal/gen/plans/v1"
	"github.com/bs-tools/trailrunn.ing/internal/geo"
	"github.com/bs-tools/trailrunn.ing/internal/mapdata"
	"github.com/bs-tools/trailrunn.ing/internal/plans"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// The bounds of the apparent temperature from which the user runs in
// shorts.
const minShortsCelsius, maxShortsCelsius = -10, 25

func (s *Service) GetGearProfile(ctx context.Context, _ *plansv1.GetGearProfileRequest) (*plansv1.GetGearProfileResponse, error) {
	userID, ok := authn.GetUserID(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, "")
	}
	profile, err := s.plans.GetGearProfile(ctx, userID)
	if err != nil {
		return nil, s.toConnectError(ctx, err, "failed to get gear profile")
	}
	return plansv1.GetGearProfileResponse_builder{Profile: gearProfileToProto(profile)}.Build(), nil
}

func (s *Service) SetGearProfile(ctx context.Context, req *plansv1.SetGearProfileRequest) (*plansv1.SetGearProfileResponse, error) {
	userID, ok := authn.GetUserID(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, "")
	}
	p := req.GetProfile()
	profile := &plans.GearProfile{}
	for i, g := range p.GetGear() {
		gear, ok := gearFromProto(g)
		if !ok {
			return nil, invalidArgument("profile.gear[%d] is unknown", i)
		}
		if !containsGear(profile.Gear, gear) {
			profile.Gear = append(profile.Gear, gear)
		}
	}
	if p.HasShortsMinApparentCelsius() {
		v := p.GetShortsMinApparentCelsius()
		if v < minShortsCelsius || v > maxShortsCelsius || math.IsNaN(v) {
			return nil, invalidArgument("profile.shorts_min_apparent_celsius must be from %d to %d, or unset for %d", minShortsCelsius, maxShortsCelsius, plans.DefaultShortsMinApparentCelsius)
		}
		profile.ShortsMinApparentCelsius = &v
	}
	set, err := s.plans.SetGearProfile(ctx, userID, profile)
	if err != nil {
		return nil, s.toConnectError(ctx, err, "failed to set gear profile")
	}
	return plansv1.SetGearProfileResponse_builder{Profile: gearProfileToProto(set)}.Build(), nil
}

func containsGear(gear []plans.Gear, g plans.Gear) bool {
	for _, other := range gear {
		if other == g {
			return true
		}
	}
	return false
}

// gearFromProto returns the gear of an enum value, named alike: GEAR_SHORTS
// is "shorts".
func gearFromProto(g plansv1.Gear) (plans.Gear, bool) {
	name := plans.Gear(strings.ToLower(strings.TrimPrefix(g.String(), "GEAR_")))
	return name, g != plansv1.Gear_GEAR_UNSPECIFIED && containsGear(plans.AllGear, name)
}

func gearToProto(g plans.Gear) plansv1.Gear {
	return plansv1.Gear(plansv1.Gear_value["GEAR_"+strings.ToUpper(string(g))])
}

func gearProfileToProto(p *plans.GearProfile) *plansv1.GearProfile {
	b := plansv1.GearProfile_builder{ShortsMinApparentCelsius: p.ShortsMinApparentCelsius}
	for _, g := range p.Gear {
		b.Gear = append(b.Gear, gearToProto(g))
	}
	if !p.UpdateTime.IsZero() {
		b.UpdateTime = timestamppb.New(p.UpdateTime)
	}
	return b.Build()
}

func gearAdviceToProto(a plans.GearAdvice) *plansv1.GearAdvice {
	items := func(items []plans.GearItem) []*plansv1.GearItem {
		result := make([]*plansv1.GearItem, len(items))
		for i, item := range items {
			result[i] = plansv1.GearItem_builder{Gear: gearToProto(item.Gear), Reason: item.Reason, Missing: item.Missing}.Build()
		}
		return result
	}
	return plansv1.GearAdvice_builder{Basis: a.Basis, Wear: items(a.Wear), Carry: items(a.Carry)}.Build()
}

// pavedShare is the share of a route on paved roads and paths, from which
// it's run in road shoes.
const pavedShare = 0.6

// snowOnGroundMeters is the modeled snow depth from which there's snow to
// run on.
const snowOnGroundMeters = 0.03

// sunnyUVIndex is the UV index from which sunglasses and a cap help.
const sunnyUVIndex = 3

// gearAdvice returns what to wear and carry on a workout, from the hours
// of the forecast during it, or of the daytime if timeKnown is false, and
// whether its route is mostly paved. It reports false for workouts that
// get none: those other than runs, races and ski tours.
func gearAdvice(w *plans.Workout, hours []mapdata.Weather, timeKnown, paved bool, fueling *plans.FuelAmounts) (plans.GearAdvice, bool) {
	skiTour := w.CrossTrainingSport == plans.CrossTrainingSportSkiTouring
	switch {
	case len(hours) == 0:
		return plans.GearAdvice{}, false
	case w.Type == plans.WorkoutTypeBase, w.Type == plans.WorkoutTypeIntensity, w.Type == plans.WorkoutTypeRace, skiTour:
	default:
		return plans.GearAdvice{}, false
	}
	c := plans.GearConditions{
		TimeKnown:           timeKnown,
		MinApparentCelsius:  math.Inf(1),
		MaxApparentCelsius:  math.Inf(-1),
		Duration:            w.ExpectedDuration(),
		ElevationGainMeters: w.TargetElevationGainMeters,
		Road:                paved || w.TargetTerrain == geo.TerrainPaved,
		SkiTour:             skiTour,
	}
	for _, h := range hours {
		c.MinApparentCelsius = math.Min(c.MinApparentCelsius, h.ApparentTemperature)
		c.MaxApparentCelsius = math.Max(c.MaxApparentCelsius, h.ApparentTemperature)
		c.MaxPrecipitationProbability = max(c.MaxPrecipitationProbability, h.PrecipitationProbability)
		c.PrecipitationMM += h.PrecipitationMM
		c.SnowfallCM += h.SnowfallCM
		c.SnowOnGround = c.SnowOnGround || h.SnowDepthMeters >= snowOnGroundMeters
		c.MaxGustsKMH = math.Max(c.MaxGustsKMH, h.WindGustsKMH)
		// Only the workout's own hours say whether it's in the dark.
		c.Dark = c.Dark || (timeKnown && !h.IsDay)
		c.Sunny = c.Sunny || (h.IsDay && h.UVIndex >= sunnyUVIndex)
	}
	if !timeKnown {
		// The whole daytime is too wide a range to dress for: the hours
		// from 9 to 18, when most runs without a start time are.
		c.MinApparentCelsius, c.MaxApparentCelsius = daytimeApparentRange(hours)
	}
	if fueling != nil {
		c.FluidML = fueling.FluidML
		c.Fuel = fueling.CarbsGrams > 0
	}
	if c.Duration == 0 {
		c.Duration = defaultWorkoutDuration
	}
	return plans.AdviseGear(c, w.Gear), true
}

// daytimeApparentRange returns the lowest and highest apparent
// temperature of the hours from 9 to 18 local time, or of all the hours if
// none are.
func daytimeApparentRange(hours []mapdata.Weather) (float64, float64) {
	low, high := math.Inf(1), math.Inf(-1)
	for _, h := range hours {
		if hour := localTime(h).Hour(); hour < 9 || hour > 18 {
			continue
		}
		low, high = math.Min(low, h.ApparentTemperature), math.Max(high, h.ApparentTemperature)
	}
	if math.IsInf(low, 1) {
		for _, h := range hours {
			low, high = math.Min(low, h.ApparentTemperature), math.Max(high, h.ApparentTemperature)
		}
	}
	return low, high
}
