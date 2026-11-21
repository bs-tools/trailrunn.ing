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
