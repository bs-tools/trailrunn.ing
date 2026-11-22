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
	"github.com/bs-tools/trailrunn.ing/internal/store"
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
	b.Comfort = gearComfortToProto(p.Comfort)
	return b.Build()
}

var gearAreas = map[plansv1.GearArea]plans.GearArea{
	plansv1.GearArea_GEAR_AREA_LEGS:  plans.GearAreaLegs,
	plansv1.GearArea_GEAR_AREA_BODY:  plans.GearAreaBody,
	plansv1.GearArea_GEAR_AREA_HANDS: plans.GearAreaHands,
	plansv1.GearArea_GEAR_AREA_HEAD:  plans.GearAreaHead,
}

var gearFeels = map[plansv1.GearFeel]plans.GearFeel{
	plansv1.GearFeel_GEAR_FEEL_TOO_COLD:   plans.GearFeelTooCold,
	plansv1.GearFeel_GEAR_FEEL_JUST_RIGHT: plans.GearFeelJustRight,
	plansv1.GearFeel_GEAR_FEEL_TOO_WARM:   plans.GearFeelTooWarm,
}

func gearAreaToProto(area plans.GearArea) plansv1.GearArea {
	for k, v := range gearAreas {
		if v == area {
			return k
		}
	}
	return plansv1.GearArea_GEAR_AREA_UNSPECIFIED
}

func gearFeelToProto(feel plans.GearFeel) plansv1.GearFeel {
	for k, v := range gearFeels {
		if v == feel {
			return k
		}
	}
	return plansv1.GearFeel_GEAR_FEEL_UNSPECIFIED
}

// gearComfortToProto lists what feedback taught, dry workouts' first, in
// the order of the parts of the body.
func gearComfortToProto(comfort map[plans.ComfortKey]plans.GearComfort) []*plansv1.GearComfort {
	var result []*plansv1.GearComfort
	for _, wet := range []bool{false, true} {
		for _, area := range plans.GearAreas {
			c, ok := comfort[plans.ComfortKey{Area: area, Wet: wet}]
			if !ok {
				continue
			}
			result = append(result, plansv1.GearComfort_builder{
				Area:          gearAreaToProto(area),
				OffsetCelsius: c.OffsetCelsius,
				Ratings:       int32(c.Ratings),
				Wet:           wet,
			}.Build())
		}
	}
	return result
}

func gearFeedbackToProto(f *plans.GearFeedback) *plansv1.GearFeedback {
	if f == nil {
		return nil
	}
	b := plansv1.GearFeedback_builder{Notes: f.Notes, Wet: f.Wet, UpdateTime: timestamppb.New(f.UpdateTime)}
	for _, area := range plans.GearAreas {
		if feel, ok := f.Feels[area]; ok {
			b.Feels = append(b.Feels, plansv1.GearAreaFeel_builder{Area: gearAreaToProto(area), Feel: gearFeelToProto(feel)}.Build())
		}
	}
	return b.Build()
}

// maxGearNotesLength bounds the notes of gear feedback.
const maxGearNotesLength = 500

func (s *Service) GetGearFeedback(ctx context.Context, req *plansv1.GetGearFeedbackRequest) (*plansv1.GetGearFeedbackResponse, error) {
	userID, ok := authn.GetUserID(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, "")
	}
	workoutID, err := store.StringToUUID(req.GetWorkoutId())
	if err != nil {
		return nil, invalidArgument("invalid workout ID: %v", err)
	}
	f, err := s.plans.GetGearFeedback(ctx, userID, workoutID)
	if err != nil {
		return nil, s.toConnectError(ctx, err, "failed to get gear feedback")
	}
	return plansv1.GetGearFeedbackResponse_builder{Feedback: gearFeedbackToProto(f)}.Build(), nil
}

func (s *Service) SetGearFeedback(ctx context.Context, req *plansv1.SetGearFeedbackRequest) (*plansv1.SetGearFeedbackResponse, error) {
	userID, ok := authn.GetUserID(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, "")
	}
	workoutID, err := store.StringToUUID(req.GetWorkoutId())
	if err != nil {
		return nil, invalidArgument("invalid workout ID: %v", err)
	}
	f := &plans.GearFeedback{Feels: map[plans.GearArea]plans.GearFeel{}, Notes: strings.TrimSpace(req.GetFeedback().GetNotes()), Wet: req.GetFeedback().GetWet()}
	if len([]rune(f.Notes)) > maxGearNotesLength {
		return nil, invalidArgument("feedback.notes must be at most %d characters", maxGearNotesLength)
	}
	for i, af := range req.GetFeedback().GetFeels() {
		area, ok := gearAreas[af.GetArea()]
		if !ok {
			return nil, invalidArgument("feedback.feels[%d].area must be LEGS, BODY, HANDS or HEAD", i)
		}
		feel, ok := gearFeels[af.GetFeel()]
		if !ok {
			return nil, invalidArgument("feedback.feels[%d].feel must be TOO_COLD, JUST_RIGHT or TOO_WARM", i)
		}
		if _, dup := f.Feels[area]; dup {
			return nil, invalidArgument("feedback.feels[%d]: %s is rated twice", i, af.GetArea())
		}
		f.Feels[area] = feel
	}
	set, err := s.plans.SetGearFeedback(ctx, userID, workoutID, f)
	if err != nil {
		return nil, s.toConnectError(ctx, err, "failed to set gear feedback")
	}
	profile, err := s.plans.GetGearProfile(ctx, userID)
	if err != nil {
		return nil, s.toConnectError(ctx, err, "failed to get gear profile")
	}
	return plansv1.SetGearFeedbackResponse_builder{
		Feedback: gearFeedbackToProto(set),
		Comfort:  gearComfortToProto(profile.Comfort),
	}.Build(), nil
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

// The chance of rain in percent, and the rain in an hour in millimeters,
// from which rain is likely, as plans.AdviseGear has them.
const (
	rainLikelyPercent = 50
	rainLikelyMM      = 1.0
)

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
	// Whether rain is likely in the workout's first hour. Without a start
	// time, when it starts is unknown.
	first := hours[0]
	c.RainAtStart = first.PrecipitationProbability >= rainLikelyPercent || first.PrecipitationMM >= rainLikelyMM
	if !timeKnown {
		c.RainAtStart = c.MaxPrecipitationProbability >= rainLikelyPercent || c.PrecipitationMM >= rainLikelyMM
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
