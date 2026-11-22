package plans

import (
	"context"
	"fmt"
	"math"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/bs-tools/trailrunn.ing/internal/plans/internal/gen"
	"github.com/bs-tools/trailrunn.ing/internal/store"
	"github.com/jackc/pgx/v5/pgtype"
)

// Gear is a piece of running or ski touring gear. Its value is stored, so
// it mustn't change.
type Gear string

// Running gear.
const (
	GearShorts          Gear = "shorts"
	GearTights          Gear = "tights"
	GearThermalTights   Gear = "thermal_tights"
	GearRainPants       Gear = "rain_pants"
	GearTShirt          Gear = "t_shirt"
	GearLongSleeve      Gear = "long_sleeve"
	GearBaseLayer       Gear = "base_layer"
	GearFleece          Gear = "fleece"
	GearWindJacket      Gear = "wind_jacket"
	GearRainJacket      Gear = "rain_jacket"
	GearInsulatedJacket Gear = "insulated_jacket"
	GearCap             Gear = "cap"
	GearToque           Gear = "toque"
	GearBuff            Gear = "buff"
	GearSunglasses      Gear = "sunglasses"
	GearLinerGloves     Gear = "liner_gloves"
	GearGloves          Gear = "gloves"
	GearThickGloves     Gear = "thick_gloves"
	GearRoadShoes       Gear = "road_shoes"
	GearTrailShoes      Gear = "trail_shoes"
	GearWaterproofShoes Gear = "waterproof_shoes"
	GearMicrospikes     Gear = "microspikes"
	GearRunningVest     Gear = "running_vest"
	GearRunningBelt     Gear = "running_belt"
	GearHeadlamp        Gear = "headlamp"
	GearPoles           Gear = "poles"
)

// Ski touring gear, with the running gear also worn touring, like base
// layers, a toque and a headlamp.
const (
	GearSoftshellPants       Gear = "softshell_pants"
	GearShellPants           Gear = "shell_pants"
	GearShellJacket          Gear = "shell_jacket"
	GearSkiGloves            Gear = "ski_gloves"
	GearGoggles              Gear = "goggles"
	GearHelmet               Gear = "helmet"
	GearAvalancheTransceiver Gear = "avalanche_transceiver"
	GearShovel               Gear = "shovel"
	GearProbe                Gear = "probe"
	GearSkiCrampons          Gear = "ski_crampons"
)

// AllGear is all the gear, in the order the API lists it.
var AllGear = []Gear{
	GearShorts, GearTights, GearThermalTights, GearRainPants,
	GearTShirt, GearLongSleeve, GearBaseLayer, GearFleece, GearWindJacket, GearRainJacket, GearInsulatedJacket,
	GearCap, GearToque, GearBuff, GearSunglasses,
	GearLinerGloves, GearGloves, GearThickGloves,
	GearRoadShoes, GearTrailShoes, GearWaterproofShoes, GearMicrospikes,
	GearRunningVest, GearRunningBelt, GearHeadlamp, GearPoles,
	GearSoftshellPants, GearShellPants, GearShellJacket, GearSkiGloves, GearGoggles, GearHelmet,
	GearAvalancheTransceiver, GearShovel, GearProbe, GearSkiCrampons,
}

// DefaultShortsMinApparentCelsius is the apparent temperature from which
// runs are in shorts, unless the user sets their own.
const DefaultShortsMinApparentCelsius = 10

// GearProfile is the gear the user has, and how they like to dress.
type GearProfile struct {
	// The gear the user has. Empty if they haven't said, when any gear is
	// suggested.
	Gear []Gear
	// The apparent temperature in degrees Celsius from which the user runs
	// in shorts. Nil for DefaultShortsMinApparentCelsius.
	ShortsMinApparentCelsius *float64
	// When the profile was last set, zero if it never was.
	UpdateTime time.Time
	// What the user's feedback on their gear taught, by part of the body,
	// on dry and wet workouts.
	Comfort map[ComfortKey]GearComfort
}

// comfort returns what the user's feedback taught about a part of the body
// in dry or wet conditions, and whether it taught anything. Wet conditions
// go by the feedback of wet workouts, or else of dry ones, as what's warm
// enough when it's dry is the least for the rain. Dry ones only go by dry
// workouts': being cold in the rain doesn't mean a long sleeve isn't enough
// on a dry day.
func (p *GearProfile) comfort(area GearArea, wet bool) (GearComfort, bool) {
	if p == nil {
		return GearComfort{}, false
	}
	if wet {
		if c, ok := p.Comfort[ComfortKey{Area: area, Wet: true}]; ok {
			return c, true
		}
	}
	c, ok := p.Comfort[ComfortKey{Area: area}]
	return c, ok
}

// shortsMin returns the apparent temperature from which runs are in
// shorts.
func (p *GearProfile) shortsMin() float64 {
	if p == nil || p.ShortsMinApparentCelsius == nil {
		return DefaultShortsMinApparentCelsius
	}
	return *p.ShortsMinApparentCelsius
}

// has reports whether the user has a piece of gear: any, if they haven't
// said what they have.
func (p *GearProfile) has(g Gear) bool {
	return p == nil || len(p.Gear) == 0 || slices.Contains(p.Gear, g)
}

// GetGearProfile gets the user's gear profile, empty if they haven't set
// one.
func (r *Repository) GetGearProfile(ctx context.Context, userID pgtype.UUID) (*GearProfile, error) {
	rows, err := r.q.GetGearProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get gear profile: %w", err)
	}
	p := gearProfileFromDB(rows)
	if p.Comfort, err = r.gearComfort(ctx, userID); err != nil {
		return nil, err
	}
	return p, nil
}

// SetGearProfile replaces the user's gear profile.
func (r *Repository) SetGearProfile(ctx context.Context, userID pgtype.UUID, p *GearProfile) (*GearProfile, error) {
	gear := make([]string, len(p.Gear))
	for i, g := range p.Gear {
		gear[i] = string(g)
	}
	var shorts pgtype.Float8
	if p.ShortsMinApparentCelsius != nil {
		shorts = pgtype.Float8{Float64: *p.ShortsMinApparentCelsius, Valid: true}
	}
	row, err := r.q.SetGearProfile(ctx, &gen.SetGearProfileParams{UserID: userID, Gear: gear, ShortsMinApparentCelsius: shorts})
	if err != nil {
		return nil, fmt.Errorf("failed to set gear profile: %w", err)
	}
	set := gearProfileFromDB([]*gen.GearProfile{row})
	if set.Comfort, err = r.gearComfort(ctx, userID); err != nil {
		return nil, err
	}
	return set, nil
}

// queueGearProfile queues reading the user's gear profile into profile,
// with the reads of their workouts.
func queueGearProfile(ctx context.Context, b *store.Batch, userID pgtype.UUID, profile **GearProfile) {
	b.Read(func(db store.Querier) error {
		rows, err := gen.New(db).GetGearProfile(ctx, userID)
		*profile = gearProfileFromDB(rows)
		return err
	})
}

func gearProfileFromDB(rows []*gen.GearProfile) *GearProfile {
	if len(rows) == 0 {
		return &GearProfile{}
	}
	row := rows[0]
	p := &GearProfile{UpdateTime: row.UpdateTime.Time}
	for _, g := range row.Gear {
		// Gear that's no longer listed is dropped.
		if slices.Contains(AllGear, Gear(g)) {
			p.Gear = append(p.Gear, Gear(g))
		}
	}
	if row.ShortsMinApparentCelsius.Valid {
		v := row.ShortsMinApparentCelsius.Float64
		p.ShortsMinApparentCelsius = &v
	}
	return p
}

// GearConditions are what a workout's gear depends on: the weather during
// it, and the workout.
type GearConditions struct {
	// Whether the weather is for the workout's hours. Otherwise it's for
	// the daytime, as the workout has no start time.
	TimeKnown bool
	// The lowest and highest apparent (feels like) temperature in degrees
	// Celsius.
	MinApparentCelsius, MaxApparentCelsius float64
	// The highest chance of precipitation in percent, and how much falls in
	// millimeters, of which snow in centimeters.
	MaxPrecipitationProbability int
	PrecipitationMM             float64
	SnowfallCM                  float64
	// Whether rain is likely at the start of the workout, rather than
	// later. Without a start time, whether it's likely at all.
	RainAtStart bool
	// Whether there's snow on the ground.
	SnowOnGround bool
	// The strongest gusts in kilometers per hour.
	MaxGustsKMH float64
	// Whether any of the workout is in the dark, and in sunshine.
	Dark, Sunny bool
	// How long the workout is expected to take, 0 if unknown.
	Duration time.Duration
	// The workout's elevation gain in meters, 0 if unknown.
	ElevationGainMeters float64
	// Whether the workout is mostly on paved roads and paths.
	Road bool
	// Whether the workout is a ski tour, rather than a run.
	SkiTour bool
	// The fluid in milliliters to carry, from the workout's fueling plan, 0
	// if it has none.
	FluidML float64
	// Whether there's fuel to carry, from the workout's fueling plan.
	Fuel bool
}

// GearItem is a piece of gear to wear or carry, and why.
type GearItem struct {
	Gear   Gear
	Reason string
	// Whether the user doesn't have it: it's what the conditions call for,
	// and nothing they have does instead.
	Missing bool
}

// GearAdvice is what to wear and carry on a workout.
type GearAdvice struct {
	// The conditions the advice is for, e.g. "Feels like 2 to 6 °C, rain
	// likely".
	Basis string
	Wear  []GearItem
	Carry []GearItem
}

// The conditions gear depends on.
const (
	// A chance of precipitation from which rain is likely, and from which
	// it's possible.
	rainLikelyPercent   = 50
	rainPossiblePercent = 25
	// Precipitation in millimeters from which rain is likely anyway.
	rainLikelyMM = 1.0
	// Heavy rain, in millimeters, for which rain pants are worth it in the
	// cold.
	heavyRainMM = 5.0
	// Gusts in kilometers per hour from which wind chills.
	windyGustsKMH = 30
	// A duration from which a run needs a vest rather than a belt, and from
	// which a cold run carries a warm layer in case of a stop.
	vestDuration = 90 * time.Minute
	longDuration = 2 * time.Hour
	// Fluid in milliliters more than a belt carries.
	vestFluidML = 600
	// Elevation gain in meters from which poles help.
	polesElevationGainMeters = 1000
)

// gearAdviser builds a workout's advice, picking the gear the user has.
type gearAdviser struct {
	profile *GearProfile
	advice  GearAdvice
}

// pick returns the first of the choices the user has, in order of
// preference, or else the first, as missing.
func (a *gearAdviser) pick(reason string, choices ...Gear) GearItem {
	for _, g := range choices {
		if a.profile.has(g) {
			return GearItem{Gear: g, Reason: reason}
		}
	}
	return GearItem{Gear: choices[0], Reason: reason, Missing: true}
}

func (a *gearAdviser) wear(reason string, choices ...Gear) {
	a.add(&a.advice.Wear, a.pick(reason, choices...))
}

func (a *gearAdviser) carry(reason string, choices ...Gear) {
	a.add(&a.advice.Carry, a.pick(reason, choices...))
}

// wearOptional and carryOptional add gear only if the user has it: gear
// that helps but isn't needed, like poles.
func (a *gearAdviser) wearOptional(reason string, g Gear) {
	if a.profile.has(g) {
		a.add(&a.advice.Wear, GearItem{Gear: g, Reason: reason})
	}
}

func (a *gearAdviser) carryOptional(reason string, g Gear) {
	if a.profile.has(g) {
		a.add(&a.advice.Carry, GearItem{Gear: g, Reason: reason})
	}
}

// add adds an item, unless the gear is already worn or carried: one
// jacket can be the answer to two conditions.
func (a *gearAdviser) add(list *[]GearItem, item GearItem) {
	if a.wearing(item.Gear) || a.carrying(item.Gear) {
		return
	}
	*list = append(*list, item)
}

func (a *gearAdviser) wearing(g Gear) bool {
	return slices.ContainsFunc(a.advice.Wear, func(i GearItem) bool { return i.Gear == g })
}

func (a *gearAdviser) carrying(g Gear) bool {
	return slices.ContainsFunc(a.advice.Carry, func(i GearItem) bool { return i.Gear == g })
}

// feels returns the temperature to dress a part of the body for: what it
// feels like, made colder or warmer by what the user's feedback taught.
func (a *gearAdviser) feels(c GearConditions, area GearArea) float64 {
	comfort, _ := a.profile.comfort(area, c.wet())
	return c.MinApparentCelsius - comfort.OffsetCelsius
}

// comfortNote describes how the user's feedback changed the advice in
// dry or wet conditions, e.g. "dressed 3 °C colder for your hands, from
// your feedback on wet runs", empty if it didn't.
func (p *GearProfile) comfortNote(wet bool) string {
	var parts []string
	fromWet := false
	for _, area := range GearAreas {
		c, _ := p.comfort(area, wet)
		if wet && c.OffsetCelsius != 0 {
			// The comfort came from wet workouts' feedback if they have any.
			_, fromWetRatings := p.Comfort[ComfortKey{Area: area, Wet: true}]
			fromWet = fromWet || fromWetRatings
		}
		switch {
		case c.OffsetCelsius > 0:
			parts = append(parts, fmt.Sprintf("%s °C colder for your %s", formatOffset(c.OffsetCelsius), area))
		case c.OffsetCelsius < 0:
			parts = append(parts, fmt.Sprintf("%s °C warmer for your %s", formatOffset(-c.OffsetCelsius), area))
		}
	}
	if len(parts) == 0 {
		return ""
	}
	source := "your feedback"
	if fromWet {
		source = "your feedback on wet workouts"
	}
	return "dressed " + strings.Join(parts, ", ") + ", from " + source
}

// formatOffset formats an offset in degrees, e.g. "3" or "1.5".
func formatOffset(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// AdviseGear returns what to wear and carry in the conditions, from the
// gear the user has. Runs follow common layering rules, with the user's
// temperature for shorts; ski tours dress for the climb and carry layers
// for the descent and for stops.
func AdviseGear(c GearConditions, profile *GearProfile) GearAdvice {
	a := &gearAdviser{profile: profile}
	a.advice.Basis = c.basis()
	if note := profile.comfortNote(c.wet()); note != "" {
		a.advice.Basis += "; " + note
	}
	if c.SkiTour {
		a.skiTour(c)
	} else {
		a.run(c)
	}
	return a.advice
}

// basis describes the conditions, e.g. "Feels like 2 to 6 °C, rain likely,
// gusts to 40 km/h, in the dark".
func (c GearConditions) basis() string {
	low, high := math.Round(c.MinApparentCelsius), math.Round(c.MaxApparentCelsius)
	s := fmt.Sprintf("Feels like %.0f °C", low)
	if high != low {
		s = fmt.Sprintf("Feels like %.0f to %.0f °C", low, high)
	}
	if !c.TimeKnown {
		s += " during the day"
	}
	switch {
	case c.SnowfallCM >= 1:
		s += ", snow likely"
	case c.rainLikely():
		s += fmt.Sprintf(", rain likely (%d%%)", c.MaxPrecipitationProbability)
	case c.MaxPrecipitationProbability >= rainPossiblePercent:
		s += fmt.Sprintf(", %d%% chance of rain", c.MaxPrecipitationProbability)
	}
	if c.MaxGustsKMH >= windyGustsKMH {
		s += fmt.Sprintf(", gusts to %.0f km/h", c.MaxGustsKMH)
	}
	if c.SnowOnGround {
		s += ", snow on the ground"
	}
	if c.Dark {
		s += ", in the dark"
	}
	return s
}

func (c GearConditions) rainLikely() bool {
	return c.MaxPrecipitationProbability >= rainLikelyPercent || c.PrecipitationMM >= rainLikelyMM
}

// wet reports whether the workout is wet, for which feedback on wet
// workouts dresses it.
func (c GearConditions) wet() bool {
	return c.rainLikely()
}

func (c GearConditions) windy() bool {
	return c.MaxGustsKMH >= windyGustsKMH
}

// run advises a run: layers for the coldest it feels, a jacket for rain
// and wind, shoes for the ground, and something to carry it all in.
func (a *gearAdviser) run(c GearConditions) {
	feels := c.MinApparentCelsius
	// Each part of the body is dressed for the temperature the user's
	// feedback says it feels like to them.
	legs, body, hands, head := a.feels(c, GearAreaLegs), a.feels(c, GearAreaBody), a.feels(c, GearAreaHands), a.feels(c, GearAreaHead)
	// Legs.
	shortsMin := a.profile.shortsMin()
	switch {
	case legs >= shortsMin:
		a.wear(fmt.Sprintf("From %.0f °C", shortsMin), GearShorts)
	case legs < -5:
		a.wear("Below −5 °C", GearThermalTights, GearTights)
	default:
		a.wear(fmt.Sprintf("Below %.0f °C", shortsMin), GearTights, GearThermalTights)
	}
	if feels < 5 && c.PrecipitationMM >= heavyRainMM {
		a.carryOptional("Heavy rain in the cold", GearRainPants)
	}

	// Body.
	switch {
	case body < -5:
		a.wear("Below −5 °C", GearBaseLayer, GearLongSleeve)
		a.wear("Below −5 °C, over the base layer", GearWindJacket, GearRainJacket)
	case body < 5:
		a.wear("Below 5 °C", GearLongSleeve, GearBaseLayer)
	case body < 10 && (c.windy() || c.rainLikely()):
		a.wear("Cool and wet or windy", GearLongSleeve, GearTShirt)
	default:
		a.wear("From 5 °C", GearTShirt)
	}
	switch {
	case c.rainLikely() && feels < 15 && (c.RainAtStart || c.Duration < vestDuration):
		a.wear("Rain likely", GearRainJacket)
	case c.rainLikely() && feels < 15:
		a.carry("Rain likely later: put it on when it starts", GearRainJacket, GearWindJacket)
	case c.rainLikely() || c.MaxPrecipitationProbability >= rainPossiblePercent:
		a.carry("In case of rain", GearRainJacket, GearWindJacket)
	case body < 5 && !a.wearing(GearRainJacket) && !a.wearing(GearWindJacket):
		a.carry("A warm, windproof layer for the cold", GearRainJacket, GearWindJacket)
	}
	if c.windy() && body < 10 && !a.wearing(GearRainJacket) && !a.wearing(GearWindJacket) {
		a.wear("Windy and cool", GearWindJacket, GearRainJacket)
	}
	if body < 5 && c.Duration >= longDuration {
		a.carry("Long and cold: for a stop or an injury", GearInsulatedJacket, GearFleece)
	}

	// Hands and head.
	switch {
	case hands < -10:
		a.wear("Below −10 °C", GearThickGloves, GearGloves)
	case hands < 0:
		a.wear("Below 0 °C", GearGloves, GearThickGloves, GearLinerGloves)
	case hands < 8:
		a.wear("Below 8 °C", GearLinerGloves, GearGloves)
	}
	switch {
	case head < 0:
		a.wear("Below 0 °C", GearToque, GearBuff)
	case head < 5 && c.windy():
		a.wear("Cold wind on the ears", GearBuff, GearToque)
	}
	if head < 5 && !a.wearing(GearBuff) {
		a.carryOptional("For the neck or ears if it's colder than it feels", GearBuff)
	}
	switch {
	case c.rainLikely() && !a.wearing(GearToque):
		a.wearOptional("Keeps rain out of the eyes", GearCap)
	case c.Sunny && feels >= 10:
		a.wearOptional("Sun", GearCap)
	}
	if c.Sunny || (c.SnowOnGround && !c.Dark) {
		reason := "Sun"
		if !c.Sunny {
			reason = "Glare off the snow"
		}
		a.wearOptional(reason, GearSunglasses)
	}

	// Feet.
	switch {
	case c.SnowOnGround || c.SnowfallCM >= 1:
		a.wear("Snow", GearWaterproofShoes, GearTrailShoes)
		if !c.Road {
			a.carryOptional("For ice and packed snow", GearMicrospikes)
		}
	case c.Road:
		a.wear("Mostly paved", GearRoadShoes, GearTrailShoes)
	// Waterproof shoes only for those who have them: trail shoes do too,
	// unlike in snow.
	case c.rainLikely() && feels < 5 && a.profile.has(GearWaterproofShoes):
		a.wear("Cold and wet", GearWaterproofShoes)
	default:
		a.wear("Trails", GearTrailShoes, GearRoadShoes)
	}
	if c.ElevationGainMeters >= polesElevationGainMeters {
		a.carryOptional(fmt.Sprintf("%.0f m of climbing", c.ElevationGainMeters), GearPoles)
	}

	// Light.
	if c.Dark {
		a.wear("Dark", GearHeadlamp)
	}

	// Something to carry it in.
	carrying := len(a.advice.Carry) > 0 || c.FluidML > 0 || c.Fuel
	switch {
	case c.FluidML > vestFluidML || c.Duration >= vestDuration || len(a.advice.Carry) > 1 || a.carrying(GearRainJacket) || a.carrying(GearInsulatedJacket):
		a.wear("For the water, food and layers to carry", GearRunningVest, GearRunningBelt)
	case carrying:
		a.wear("For what to carry", GearRunningBelt, GearRunningVest)
	}
}

// skiTour advises a ski tour: light layers for the climb, warm and
// windproof ones for the descent and stops, and avalanche safety gear.
func (a *gearAdviser) skiTour(c GearConditions) {
	body, legs, head := a.feels(c, GearAreaBody), a.feels(c, GearAreaLegs), a.feels(c, GearAreaHead)
	a.wear("Wicks sweat on the climb", GearBaseLayer, GearLongSleeve)
	if body < -10 {
		a.wear("Below −10 °C", GearFleece, GearInsulatedJacket)
	}
	if legs < -10 {
		a.wear("Below −10 °C, under the pants", GearThermalTights, GearTights)
	}
	switch {
	case c.SnowfallCM >= 1 || c.rainLikely():
		a.wear("Snow or rain", GearShellPants, GearSoftshellPants)
	default:
		a.wear("Breathable on the climb", GearSoftshellPants, GearShellPants)
	}
	a.wear("Light gloves for the climb", GearLinerGloves, GearGloves)
	if head < -5 {
		a.wear("Below −5 °C", GearToque, GearBuff)
	} else {
		a.carry("For the descent and stops", GearToque, GearBuff)
	}
	a.wearOptional("Sun and wind on the face and neck", GearBuff)
	a.wear("Glare off the snow", GearSunglasses, GearGoggles)
	a.wear("Always, switched on to send", GearAvalancheTransceiver)

	a.carry("For the descent and stops", GearShellJacket, GearRainJacket)
	a.carry("For stops, and in case of an injury", GearInsulatedJacket, GearFleece)
	if !a.wearing(GearFleece) && body < 0 {
		a.carryOptional("An extra layer below 0 °C", GearFleece)
	}
	a.carry("Warm gloves for the descent", GearSkiGloves, GearThickGloves)
	if c.windy() || c.SnowfallCM >= 1 || head < -10 {
		a.carry("Wind, snow or cold on the descent", GearGoggles)
	}
	a.carryOptional("For the descent", GearHelmet)
	a.carry("Avalanche rescue", GearShovel)
	a.carry("Avalanche rescue", GearProbe)
	// Without new snow, the snow can be hard or icy.
	if c.SnowfallCM < 1 {
		a.carryOptional("For hard or icy snow on the climb", GearSkiCrampons)
	}
	reason := "Short days: in case it takes longer"
	if c.Dark {
		reason = "Dark"
		a.wear(reason, GearHeadlamp)
	} else {
		a.carry(reason, GearHeadlamp)
	}
}
