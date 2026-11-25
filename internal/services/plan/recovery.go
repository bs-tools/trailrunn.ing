package plan

import (
	"fmt"
	"slices"
	"strings"

	plansv1 "github.com/bs-tools/trailrunn.ing/internal/gen/plans/v1"
	"github.com/bs-tools/trailrunn.ing/internal/plans"
)

// enumByName maps the API's values of an enum to the stored values, whose
// names are the API's in lower case without the prefix, e.g.
// RECOVERY_MODALITY_COLD is "cold".
func enumByName[A ~int32, S ~string](values map[string]int32, prefix string, stored []S) map[A]S {
	m := make(map[A]S, len(stored))
	for _, s := range stored {
		value, ok := values[prefix+strings.ToUpper(string(s))]
		if !ok {
			panic(fmt.Sprintf("%q has no API value %s%s", s, prefix, strings.ToUpper(string(s))))
		}
		m[A(value)] = s
	}
	return m
}

var (
	recoveryModalities  = enumByName[plansv1.RecoveryModality](plansv1.RecoveryModality_value, "RECOVERY_MODALITY_", plans.RecoveryModalities)
	stimulationPrograms = enumByName[plansv1.StimulationProgram](plansv1.StimulationProgram_value, "STIMULATION_PROGRAM_", plans.StimulationPrograms)
	tensMassages        = enumByName[plansv1.TensMassage](plansv1.TensMassage_value, "TENS_MASSAGE_", plans.TENSMassages)
	recoverySides       = map[plansv1.RecoverySide]plans.Side{
		plansv1.RecoverySide_RECOVERY_SIDE_UNSPECIFIED: plans.SideBoth,
		plansv1.RecoverySide_RECOVERY_SIDE_LEFT:        plans.SideLeft,
		plansv1.RecoverySide_RECOVERY_SIDE_RIGHT:       plans.SideRight,
	}

	recoveryModalitiesToProto  = reverse(recoveryModalities)
	stimulationProgramsToProto = reverse(stimulationPrograms)
	tensMassagesToProto        = reverse(tensMassages)
	recoverySidesToProto       = reverse(recoverySides)
)

// recoveryFromProto converts and validates a recovery workout.
func recoveryFromProto(r *plansv1.RecoveryWorkout) (*plans.RecoveryWorkout, error) {
	if len(r.GetItems()) > plans.MaxRecoveryItems {
		return nil, fmt.Errorf("items: at most %d items are allowed", plans.MaxRecoveryItems)
	}
	recovery := &plans.RecoveryWorkout{Done: r.GetDone()}
	for i, item := range r.GetItems() {
		converted, err := recoveryItemFromProto(item)
		if err != nil {
			return nil, fmt.Errorf("items[%d].%w", i, err)
		}
		recovery.Items = append(recovery.Items, converted)
	}
	if err := recovery.Validate(); err != nil {
		return nil, err
	}
	return recovery, nil
}

func recoveryItemFromProto(item *plansv1.RecoveryItem) (plans.RecoveryItem, error) {
	modality, ok := recoveryModalities[item.GetModality()]
	if !ok {
		return plans.RecoveryItem{}, fmt.Errorf("modality is required: ELECTRICAL_STIMULATION, PERCUSSION, FOAM_ROLLING, MASSAGE, COMPRESSION, HEAT, COLD or OTHER")
	}
	side, ok := recoverySides[item.GetSide()]
	if !ok {
		return plans.RecoveryItem{}, fmt.Errorf("side %v is unknown", item.GetSide())
	}
	converted := plans.RecoveryItem{
		Modality:        modality,
		DurationSeconds: item.GetDurationSeconds(),
		Side:            side,
		Notes:           item.GetNotes(),
	}
	for _, m := range item.GetMuscles() {
		muscle, ok := muscles[m]
		if !ok {
			return plans.RecoveryItem{}, fmt.Errorf("muscles: invalid muscle %v", m)
		}
		if !slices.Contains(converted.Muscles, muscle) {
			converted.Muscles = append(converted.Muscles, muscle)
		}
	}
	if item.HasStimulation() {
		s := item.GetStimulation()
		stimulation := &plans.Stimulation{
			Pods:      s.GetPods(),
			Intensity: s.GetIntensity(),
		}
		if s.GetProgram() != plansv1.StimulationProgram_STIMULATION_PROGRAM_UNSPECIFIED {
			if stimulation.Program, ok = stimulationPrograms[s.GetProgram()]; !ok {
				return plans.RecoveryItem{}, fmt.Errorf("stimulation.program %v is unknown", s.GetProgram())
			}
		}
		if s.GetTensMassage() != plansv1.TensMassage_TENS_MASSAGE_UNSPECIFIED {
			if stimulation.TENSMassage, ok = tensMassages[s.GetTensMassage()]; !ok {
				return plans.RecoveryItem{}, fmt.Errorf("stimulation.tens_massage %v is unknown", s.GetTensMassage())
			}
		}
		if s.HasPainLevel() {
			level := s.GetPainLevel()
			stimulation.PainLevel = &level
		}
		converted.Stimulation = stimulation
	}
	return converted, nil
}

func recoveryToProto(w *plans.Workout) *plansv1.RecoveryWorkout {
	b := plansv1.RecoveryWorkout_builder{TargetDurationSeconds: w.TargetDurationSeconds}
	if w.Recovery == nil {
		return b.Build()
	}
	b.Done = w.Recovery.Done
	for _, item := range w.Recovery.Items {
		ib := plansv1.RecoveryItem_builder{
			Modality:        recoveryModalitiesToProto[item.Modality],
			DurationSeconds: item.DurationSeconds,
			Side:            recoverySidesToProto[item.Side],
			Notes:           item.Notes,
		}
		for _, m := range item.Muscles {
			ib.Muscles = append(ib.Muscles, musclesToProto[m])
		}
		if s := item.Stimulation; s != nil {
			ib.Stimulation = plansv1.ElectricalStimulation_builder{
				Program:     stimulationProgramsToProto[s.Program],
				Pods:        s.Pods,
				Intensity:   s.Intensity,
				PainLevel:   s.PainLevel,
				TensMassage: tensMassagesToProto[s.TENSMassage],
			}.Build()
		}
		b.Items = append(b.Items, ib.Build())
	}
	return b.Build()
}
