package plans

import (
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/bs-tools/trailrunn.ing/internal/plans/internal/gen"
	"github.com/jackc/pgx/v5/pgtype"
)

// gearOf returns the gear of items, missing gear marked with a "!".
func gearOf(items []GearItem) []string {
	names := make([]string, len(items))
	for i, item := range items {
		names[i] = string(item.Gear)
		if item.Missing {
			names[i] += "!"
		}
	}
	return names
}

func TestAdviseGear(t *testing.T) {
	t.Parallel()
	ten := 12.0
	full := &GearProfile{Gear: []Gear{
		GearShorts, GearTights, GearTShirt, GearLongSleeve, GearRainJacket, GearLinerGloves, GearGloves,
		GearToque, GearBuff, GearCap, GearSunglasses, GearTrailShoes, GearRoadShoes, GearRunningVest, GearHeadlamp,
	}}
	for _, tc := range []struct {
		name      string
		c         GearConditions
		profile   *GearProfile
		wantWear  []string
		wantCarry []string
		wantBasis string
	}{
		{
			// Below 5 °C: tights and a long sleeve, with the rain jacket in
			// the vest.
			name:      "cold and dry",
			c:         GearConditions{TimeKnown: true, MinApparentCelsius: 3, MaxApparentCelsius: 4, Duration: time.Hour},
			profile:   full,
			wantWear:  []string{"tights", "long_sleeve", "liner_gloves", "trail_shoes", "running_vest"},
			wantCarry: []string{"rain_jacket", "buff"},
			wantBasis: "Feels like 3 to 4 °C",
		},
		{
			name:     "warm and sunny on the road",
			c:        GearConditions{TimeKnown: true, MinApparentCelsius: 14, MaxApparentCelsius: 18, Sunny: true, Road: true, Duration: 45 * time.Minute},
			profile:  full,
			wantWear: []string{"shorts", "t_shirt", "cap", "sunglasses", "road_shoes"},
		},
		{
			// The user's own temperature for shorts.
			name:     "below the user's shorts temperature",
			c:        GearConditions{TimeKnown: true, MinApparentCelsius: 11, MaxApparentCelsius: 11},
			profile:  &GearProfile{ShortsMinApparentCelsius: &ten},
			wantWear: []string{"tights", "t_shirt", "trail_shoes"},
		},
		{
			name:      "wet, windy and dark",
			c:         GearConditions{TimeKnown: true, MinApparentCelsius: 7, MaxApparentCelsius: 8, MaxPrecipitationProbability: 80, PrecipitationMM: 3, MaxGustsKMH: 45, Dark: true},
			profile:   full,
			wantWear:  []string{"tights", "long_sleeve", "rain_jacket", "liner_gloves", "cap", "trail_shoes", "headlamp"},
			wantBasis: "Feels like 7 to 8 °C, rain likely (80%), gusts to 45 km/h, in the dark",
		},
		{
			// Gear the user doesn't have is suggested as missing, unless
			// something they have does instead.
			name:      "freezing without winter gear",
			c:         GearConditions{TimeKnown: true, MinApparentCelsius: -8, MaxApparentCelsius: -6, SnowOnGround: true, Duration: 2 * time.Hour, FluidML: 750, Fuel: true},
			profile:   full,
			wantWear:  []string{"tights", "long_sleeve", "rain_jacket", "gloves", "toque", "sunglasses", "trail_shoes", "running_vest"},
			wantCarry: []string{"insulated_jacket!", "buff"},
		},
		{
			// Without a gear profile, any gear is suggested.
			name:      "freezing with any gear",
			c:         GearConditions{TimeKnown: true, MinApparentCelsius: -8, MaxApparentCelsius: -6, SnowOnGround: true},
			wantWear:  []string{"thermal_tights", "base_layer", "wind_jacket", "gloves", "toque", "sunglasses", "waterproof_shoes", "running_vest"},
			wantCarry: []string{"buff", "microspikes"},
		},
		{
			name:      "ski tour",
			c:         GearConditions{TimeKnown: true, SkiTour: true, MinApparentCelsius: -7, MaxApparentCelsius: -2, SnowOnGround: true, SnowfallCM: 4, MaxGustsKMH: 40},
			wantWear:  []string{"base_layer", "shell_pants", "liner_gloves", "toque", "buff", "sunglasses", "avalanche_transceiver"},
			wantCarry: []string{"shell_jacket", "insulated_jacket", "fleece", "ski_gloves", "goggles", "helmet", "shovel", "probe", "headlamp"},
		},
	} {
		got := AdviseGear(tc.c, tc.profile)
		if wear := gearOf(got.Wear); !slices.Equal(wear, tc.wantWear) {
			t.Errorf("%s: wear %v, want %v", tc.name, wear, tc.wantWear)
		}
		if carry := gearOf(got.Carry); (tc.wantCarry != nil || len(carry) > 0) && !slices.Equal(carry, tc.wantCarry) {
			t.Errorf("%s: carry %v, want %v", tc.name, carry, tc.wantCarry)
		}
		if tc.wantBasis != "" && got.Basis != tc.wantBasis {
			t.Errorf("%s: basis %q, want %q", tc.name, got.Basis, tc.wantBasis)
		}
		for _, item := range append(got.Wear, got.Carry...) {
			if strings.TrimSpace(item.Reason) == "" {
				t.Errorf("%s: %s has no reason", tc.name, item.Gear)
			}
		}
	}
}

func TestComfortFrom(t *testing.T) {
	t.Parallel()
	text := func(feel GearFeel) pgtype.Text {
		return pgtype.Text{String: string(feel), Valid: feel != GearFeelUnrated}
	}
	row := func(legs, hands GearFeel) *gen.ListRecentGearFeedbackRow {
		return &gen.ListRecentGearFeedbackRow{Legs: text(legs), Hands: text(hands)}
	}
	// The latest first: hands were too cold twice, then just right, so
	// they keep their adjustment. Legs were too warm once, then too cold.
	comfort := comfortFrom([]*gen.ListRecentGearFeedbackRow{
		row(GearFeelTooCold, GearFeelJustRight),
		row(GearFeelUnrated, GearFeelTooCold),
		row(GearFeelTooWarm, GearFeelTooCold),
	})
	if got := comfort[GearAreaHands]; got.OffsetCelsius != 3 || got.Ratings != 3 {
		t.Errorf("got hands %+v, want 3 °C colder from 3 ratings", got)
	}
	if got := comfort[GearAreaLegs]; got.OffsetCelsius != 0 || got.Ratings != 2 {
		t.Errorf("got legs %+v, want no offset from 2 ratings", got)
	}
	if _, ok := comfort[GearAreaHead]; ok {
		t.Errorf("got head %+v without ratings", comfort[GearAreaHead])
	}
	// The offset stops at 6 °C.
	var cold []*gen.ListRecentGearFeedbackRow
	for range 10 {
		cold = append(cold, row(GearFeelTooCold, GearFeelUnrated))
	}
	if got := comfortFrom(cold)[GearAreaLegs]; got.OffsetCelsius != 6 {
		t.Errorf("got legs %+v, want 6 °C at most", got)
	}
}

func TestAdviseGearComfort(t *testing.T) {
	t.Parallel()
	c := GearConditions{TimeKnown: true, MinApparentCelsius: 11, MaxApparentCelsius: 12}
	// At 11 °C it's shorts and no gloves, unless the legs and hands run
	// cold: then it's as if it were 8 °C for them.
	got := AdviseGear(c, &GearProfile{Comfort: map[GearArea]GearComfort{
		GearAreaLegs:  {OffsetCelsius: 3, Ratings: 2},
		GearAreaHands: {OffsetCelsius: 4.5, Ratings: 3},
	}})
	if wear := gearOf(got.Wear); !slices.Equal(wear, []string{"tights", "t_shirt", "liner_gloves", "trail_shoes"}) {
		t.Errorf("got wear %v, want tights and liner gloves", wear)
	}
	if want := "Feels like 11 to 12 °C; dressed 3 °C colder for your legs, 4.5 °C colder for your hands, from your feedback"; got.Basis != want {
		t.Errorf("got basis %q, want %q", got.Basis, want)
	}
	// Running warm, the top stays a t-shirt down to 2 °C.
	got = AdviseGear(GearConditions{TimeKnown: true, MinApparentCelsius: 2, MaxApparentCelsius: 2}, &GearProfile{Comfort: map[GearArea]GearComfort{
		GearAreaBody: {OffsetCelsius: -3, Ratings: 2},
	}})
	if !slices.Contains(gearOf(got.Wear), "t_shirt") {
		t.Errorf("got wear %v, want a t-shirt", gearOf(got.Wear))
	}
}
