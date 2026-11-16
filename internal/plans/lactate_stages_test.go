package plans

import (
	"slices"
	"testing"
)

func TestLactateTestStages(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		stages LactateTestStages
		want   []LactateTestStage
	}{
		{
			name:   "defaults",
			stages: LactateTestStagesFrom(0, 0),
			want:   []LactateTestStage{{100, 5}, {110, 5}, {120, 5}, {130, 5}, {140, 5}, {150, 5}, {160, 5}, {170, 5}, {180, 5}},
		},
		{
			// The threshold is rounded to 145: 5 bpm steps, held more
			// closely, from 135 to 155.
			name:   "around the last threshold",
			stages: LactateTestStagesFrom(100, 143),
			want:   []LactateTestStage{{115, 5}, {125, 5}, {135, 3}, {140, 3}, {145, 3}, {150, 3}, {155, 3}, {165, 5}, {175, 5}},
		},
		{
			// A start that isn't a whole number of steps below the close
			// stages still reaches their first.
			name:   "a chosen start",
			stages: LactateTestStages{StartBPM: 100, StepBPM: 10, EndBPM: 160, RangeBPM: 5, ThresholdBPM: 138},
			want:   []LactateTestStage{{100, 5}, {110, 5}, {120, 5}, {130, 3}, {135, 3}, {140, 3}, {145, 3}, {150, 3}, {160, 5}},
		},
		{
			// A high threshold still gets a stage past the close ones.
			name:   "a high threshold",
			stages: LactateTestStagesFrom(0, 172),
			want:   []LactateTestStage{{140, 5}, {150, 5}, {160, 3}, {165, 3}, {170, 3}, {175, 3}, {180, 3}, {190, 5}},
		},
		{
			// Steps smaller than the close ones stay as they are.
			name:   "small steps",
			stages: LactateTestStages{StartBPM: 140, StepBPM: 4, EndBPM: 152, RangeBPM: 2, ThresholdBPM: 150},
			want:   []LactateTestStage{{140, 2}, {144, 2}, {148, 2}, {152, 2}},
		},
	} {
		if got := tc.stages.Stages(); !slices.Equal(got, tc.want) {
			t.Errorf("%s: got stages %v, want %v", tc.name, got, tc.want)
		}
	}
}
