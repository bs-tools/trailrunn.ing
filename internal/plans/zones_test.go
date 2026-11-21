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
		{aerobic: 149, anaerobic: 170, want: HeartRateZones{Zone1BPM: 119, Zone2BPM: 134, Zone3BPM: 149, Zone4BPM: 170}, wantOK: true},
		{aerobic: 150, want: HeartRateZones{Zone1BPM: 120, Zone2BPM: 135, Zone3BPM: 150}, wantOK: true},
		// An anaerobic threshold below the aerobic one isn't a zone.
		{aerobic: 150, anaerobic: 140, want: HeartRateZones{Zone1BPM: 120, Zone2BPM: 135, Zone3BPM: 150}, wantOK: true},
		{anaerobic: 170},
	} {
		got, ok := UphillAthleteZones(tc.aerobic, tc.anaerobic)
		if got != tc.want || ok != tc.wantOK {
			t.Errorf("UphillAthleteZones(%d, %d) = %+v, %v, want %+v, %v", tc.aerobic, tc.anaerobic, got, ok, tc.want, tc.wantOK)
		}
	}
}
