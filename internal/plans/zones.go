package plans

import "math"

// HeartRateZones are the heart rate zones of Uphill Athlete's system, which
// anchors them to the two thresholds a lactate test finds rather than to a
// maximum heart rate: zone 1 is from 80% to 90% of the aerobic threshold's
// heart rate, zone 2 from there to the aerobic threshold, zone 3 from the
// aerobic to the anaerobic threshold, and zone 4 above it. Below zone 1 is
// recovery. Aerobic base training is in zones 1 and 2.
type HeartRateZones struct {
	// The heart rates the zones start at, in beats per minute. Zone 4
	// starts at the anaerobic threshold, 0 if it's unknown: zone 3 then has
	// no top.
	Zone1BPM, Zone2BPM, Zone3BPM, Zone4BPM int32
}

// The fractions of the aerobic threshold's heart rate that zones 1 and 2
// start at.
const (
	zone1Fraction = 0.8
	zone2Fraction = 0.9
)

// UphillAthleteZones returns the zones of a user whose aerobic and
// anaerobic thresholds are at the heart rates, and whether there are any:
// they need the aerobic threshold. anaerobicBPM is 0 if unknown.
func UphillAthleteZones(aerobicBPM, anaerobicBPM int32) (HeartRateZones, bool) {
	if aerobicBPM <= 0 {
		return HeartRateZones{}, false
	}
	z := HeartRateZones{
		Zone1BPM: int32(math.Round(float64(aerobicBPM) * zone1Fraction)),
		Zone2BPM: int32(math.Round(float64(aerobicBPM) * zone2Fraction)),
		Zone3BPM: aerobicBPM,
	}
	// An anaerobic threshold at or below the aerobic one is a mistaken
	// test, not a zone.
	if anaerobicBPM > aerobicBPM {
		z.Zone4BPM = anaerobicBPM
	}
	return z, true
}

// MaxHeartRateZone is the highest zone.
const MaxHeartRateZone = 4

// zone4TopBPM is the top of zone 4, which goes up to the maximum heart
// rate. That isn't known, and nobody's is higher than this.
const zone4TopBPM = 220

// Range returns the heart rates from the bottom of a zone to the top of
// another, at least as high. It reports false for zones that aren't, and
// for zones bounded by the anaerobic threshold if it's unknown: the top of
// zone 3, and the bottom of zone 4.
func (z HeartRateZones) Range(lowZone, highZone int32) (lowBPM, highBPM int32, ok bool) {
	if lowZone < 1 || highZone > MaxHeartRateZone || highZone < lowZone {
		return 0, 0, false
	}
	bottoms := [...]int32{z.Zone1BPM, z.Zone2BPM, z.Zone3BPM, z.Zone4BPM}
	tops := [...]int32{z.Zone2BPM, z.Zone3BPM, z.Zone4BPM, zone4TopBPM}
	lowBPM, highBPM = bottoms[lowZone-1], tops[highZone-1]
	if lowBPM == 0 || highBPM == 0 {
		return 0, 0, false
	}
	return lowBPM, highBPM, true
}
