package plans

import "testing"

func TestUphillAthleteZones(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		aerobic, anaerobic int32
		want               HeartRateZones
		wantOK             bool
	}{
		// 80% and 90% of 149 are 119.2 and 134.1.
		{aerobic: 149, anaerobic: 170, want: HeartRateZones{Zone1BPM: 119, Zone2BPM: 134, Zone3BPM: 149, Zone4BPM: 170, MaxBPM: 220}, wantOK: true},
		{aerobic: 150, want: HeartRateZones{Zone1BPM: 120, Zone2BPM: 135, Zone3BPM: 150, MaxBPM: 220}, wantOK: true},
		// An anaerobic threshold below the aerobic one isn't a zone.
		{aerobic: 150, anaerobic: 140, want: HeartRateZones{Zone1BPM: 120, Zone2BPM: 135, Zone3BPM: 150, MaxBPM: 220}, wantOK: true},
		{anaerobic: 170},
	} {
		got, ok := UphillAthleteZones(tc.aerobic, tc.anaerobic, 0)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("UphillAthleteZones(%d, %d) = %+v, %v, want %+v, %v", tc.aerobic, tc.anaerobic, got, ok, tc.want, tc.wantOK)
		}
	}
}

func TestHeartRateZoneRange(t *testing.T) {
	t.Parallel()
	both, _ := UphillAthleteZones(149, 170, 0)
	aerobic, _ := UphillAthleteZones(149, 0, 0)
	for _, tc := range []struct {
		zones             HeartRateZones
		low, high         int32
		wantLow, wantHigh int32
		wantOK            bool
	}{
		{zones: both, low: 1, high: 1, wantLow: 119, wantHigh: 134, wantOK: true},
		{zones: both, low: 1, high: 2, wantLow: 119, wantHigh: 149, wantOK: true},
		{zones: both, low: 2, high: 3, wantLow: 134, wantHigh: 170, wantOK: true},
		// Zone 4 has no top short of the maximum heart rate.
		{zones: both, low: 4, high: 4, wantLow: 170, wantHigh: 220, wantOK: true},
		// The anaerobic threshold is the top of zone 3 and the bottom of 4.
		{zones: aerobic, low: 1, high: 2, wantLow: 119, wantHigh: 149, wantOK: true},
		{zones: aerobic, low: 3, high: 3},
		{zones: aerobic, low: 4, high: 4},
		{zones: aerobic, low: 3, high: 4, wantLow: 149, wantHigh: 220, wantOK: true},
		{zones: both, low: 0, high: 2},
		{zones: both, low: 3, high: 2},
		{zones: both, low: 4, high: 5},
	} {
		low, high, ok := tc.zones.Range(tc.low, tc.high)
		if low != tc.wantLow || high != tc.wantHigh || ok != tc.wantOK {
			t.Errorf("Range(%d, %d) = %d, %d, %v, want %d, %d, %v", tc.low, tc.high, low, high, ok, tc.wantLow, tc.wantHigh, tc.wantOK)
		}
	}
}

func TestZonesWithMaxHeartRate(t *testing.T) {
	t.Parallel()
	z, _ := UphillAthleteZones(149, 170, 188)
	if low, high, ok := z.Range(4, 4); !ok || low != 170 || high != 188 {
		t.Errorf("zone 4 = %d to %d, %v, want 170 to 188", low, high, ok)
	}
	// A maximum below the anaerobic threshold is a mistake.
	if z, _ := UphillAthleteZones(149, 170, 160); z.MaxBPM != 220 {
		t.Errorf("got maximum %d, want 220 for a mistaken one", z.MaxBPM)
	}
}
