package plans

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// RecoveryWorkout is recovery work: electrical muscle stimulation, massage,
// compression, heat or cold, done to recover or treat pain rather than to
// train. It's stored as JSON with the workout, so its field names and
// values mustn't change.
type RecoveryWorkout struct {
	Items []RecoveryItem `json:"items"`
	// Whether the user did it.
	Done bool `json:"done,omitempty"`
}

// RecoveryModality is a kind of recovery work.
type RecoveryModality string

const (
	RecoveryElectricalStimulation RecoveryModality = "electrical_stimulation"
	RecoveryPercussion            RecoveryModality = "percussion"
	RecoveryFoamRolling           RecoveryModality = "foam_rolling"
	RecoveryMassage               RecoveryModality = "massage"
	RecoveryCompression           RecoveryModality = "compression"
	RecoveryHeat                  RecoveryModality = "heat"
	RecoveryCold                  RecoveryModality = "cold"
	RecoveryOther                 RecoveryModality = "other"
)

// RecoveryModalities are the kinds of recovery work, in the API's order.
var RecoveryModalities = []RecoveryModality{
	RecoveryElectricalStimulation, RecoveryPercussion, RecoveryFoamRolling, RecoveryMassage,
	RecoveryCompression, RecoveryHeat, RecoveryCold, RecoveryOther,
}

// Side is the side of the body recovery work is done on.
type Side string

const (
	// SideBoth is both sides, or a part of the body without sides.
	SideBoth  Side = ""
	SideLeft  Side = "left"
	SideRight Side = "right"
)

// StimulationProgram is a program of an electrical stimulation device,
// named after PowerDot's.
type StimulationProgram string

// The programs, by kind.
const (
	ProgramActiveRecovery    StimulationProgram = "active_recovery"
	ProgramLightRecovery     StimulationProgram = "light_recovery"
	ProgramExtendedRecovery  StimulationProgram = "extended_recovery"
	ProgramMuscleEndurance   StimulationProgram = "muscle_endurance"
	ProgramStrengthEndurance StimulationProgram = "strength_endurance"
	ProgramResistance        StimulationProgram = "resistance"
	ProgramStrength          StimulationProgram = "strength"
	ProgramExplosiveStrength StimulationProgram = "explosive_strength"
	ProgramPotentiation      StimulationProgram = "potentiation"
	ProgramMassage           StimulationProgram = "massage"
	ProgramPainRelief        StimulationProgram = "pain_relief"
	ProgramPeriodPainRelief  StimulationProgram = "period_pain_relief"
)

// StimulationPrograms are the programs, in the API's order.
var StimulationPrograms = []StimulationProgram{
	ProgramActiveRecovery, ProgramLightRecovery, ProgramExtendedRecovery,
	ProgramMuscleEndurance, ProgramStrengthEndurance, ProgramResistance, ProgramStrength,
	ProgramExplosiveStrength, ProgramPotentiation, ProgramMassage,
	ProgramPainRelief, ProgramPeriodPainRelief,
}

// TENS returns whether the program eases pain with TENS, rather than
// contracting the muscles with EMS.
func (p StimulationProgram) TENS() bool {
	return p == ProgramPainRelief || p == ProgramPeriodPainRelief
}

// TENSMassage is the massage style of a TENS pain relief program.
type TENSMassage string

const (
	TENSMassageNone         TENSMassage = ""
	TENSMassageRefreshing   TENSMassage = "refreshing"
	TENSMassageLowFrequency TENSMassage = "low_frequency"
	TENSMassageMultiPhase   TENSMassage = "multi_phase"
)

// TENSMassages are the massage styles, in the API's order.
var TENSMassages = []TENSMassage{TENSMassageRefreshing, TENSMassageLowFrequency, TENSMassageMultiPhase}

// RecoveryItem is a part of a recovery workout.
type RecoveryItem struct {
	Modality        RecoveryModality `json:"modality"`
	DurationSeconds int32            `json:"duration_seconds,omitempty"`
	// The muscles it's for, where pads or a massage gun go. Empty for the
	// whole body.
	Muscles []Muscle `json:"muscles,omitempty"`
	Side    Side     `json:"side,omitempty"`
	Notes   string   `json:"notes,omitempty"`
	// The settings of electrical stimulation, nil for other modalities.
	Stimulation *Stimulation `json:"stimulation,omitempty"`
}

// Stimulation is how to set up electrical stimulation.
type Stimulation struct {
	Program StimulationProgram `json:"program"`
	// How many pods to use, 0 if not set.
	Pods int32 `json:"pods,omitempty"`
	// The intensity to aim for, from 1 to MaxStimulationIntensity, 0 if not
	// set.
	Intensity int32 `json:"intensity,omitempty"`
	// The pain level a pain program is built for, nil if not given.
	PainLevel   *int32      `json:"pain_level,omitempty"`
	TENSMassage TENSMassage `json:"tens_massage,omitempty"`
}

const (
	// MaxRecoveryItems is how many items a recovery workout can have.
	MaxRecoveryItems = 20
	// MaxRecoveryItemSeconds is how long an item can take: 4 hours, longer
	// than any program or treatment.
	MaxRecoveryItemSeconds = 4 * 60 * 60
	// MaxRecoveryNotesRunes is how long an item's notes can be.
	MaxRecoveryNotesRunes = 1000
	// MaxStimulationPods is how many pods a session can use, as a PowerDot
	// Duo has two.
	MaxStimulationPods = 2
	// MaxStimulationIntensity is the top of the intensity scale.
	MaxStimulationIntensity = 100
	// MaxPainLevel is the top of the pain scale.
	MaxPainLevel = 10
)

// Validate checks a recovery workout, with errors naming the API's fields,
// and trims its notes.
func (r *RecoveryWorkout) Validate() error {
	if len(r.Items) == 0 {
		return errors.New("items: add at least one item, saying what recovery work to do")
	}
	if len(r.Items) > MaxRecoveryItems {
		return fmt.Errorf("items: at most %d items are allowed", MaxRecoveryItems)
	}
	for i := range r.Items {
		if err := r.Items[i].validate(); err != nil {
			return fmt.Errorf("items[%d].%w", i, err)
		}
	}
	return nil
}

func (item *RecoveryItem) validate() error {
	item.Notes = strings.TrimSpace(item.Notes)
	switch {
	case item.Modality == "":
		return errors.New("modality is required")
	case item.DurationSeconds < 0 || item.DurationSeconds > MaxRecoveryItemSeconds:
		return fmt.Errorf("duration_seconds must be from 0 to %d", MaxRecoveryItemSeconds)
	case len([]rune(item.Notes)) > MaxRecoveryNotesRunes:
		return fmt.Errorf("notes must be at most %d characters", MaxRecoveryNotesRunes)
	case item.Modality == RecoveryOther && item.Notes == "":
		return errors.New("notes are required for OTHER, saying what it is")
	}
	if item.Modality != RecoveryElectricalStimulation {
		if item.Stimulation != nil {
			return errors.New("stimulation is only for ELECTRICAL_STIMULATION; leave it unset")
		}
		return nil
	}
	if len(item.Muscles) == 0 {
		return errors.New("muscles are required for electrical stimulation, saying where the pads go")
	}
	s := item.Stimulation
	switch {
	case s == nil || s.Program == "":
		return errors.New("stimulation.program is required for electrical stimulation")
	case s.Pods < 0 || s.Pods > MaxStimulationPods:
		return fmt.Errorf("stimulation.pods must be from 1 to %d, or 0 if not set", MaxStimulationPods)
	case s.Intensity < 0 || s.Intensity > MaxStimulationIntensity:
		return fmt.Errorf("stimulation.intensity must be from 1 to %d, or 0 if not set", MaxStimulationIntensity)
	case s.PainLevel != nil && !s.Program.TENS():
		return errors.New("stimulation.pain_level is only for the PAIN_RELIEF and PERIOD_PAIN_RELIEF programs")
	case s.PainLevel != nil && (*s.PainLevel < 0 || *s.PainLevel > MaxPainLevel):
		return fmt.Errorf("stimulation.pain_level must be from 0 to %d", MaxPainLevel)
	case s.TENSMassage != TENSMassageNone && s.Program != ProgramPainRelief:
		return errors.New("stimulation.tens_massage is only for the PAIN_RELIEF program")
	}
	return nil
}

// Done returns whether the user did the workout: an activity completed
// it, they logged a strength workout, or marked recovery work done.
func (w *Workout) Done() bool {
	return w.CompletedActivityID.Valid || w.Log != nil || w.Recovery != nil && w.Recovery.Done
}

// Duration returns how long the items take together, 0 if none has a
// duration.
func (r *RecoveryWorkout) Duration() time.Duration {
	var total time.Duration
	for _, item := range r.Items {
		total += time.Duration(item.DurationSeconds) * time.Second
	}
	return total
}

// recoveryToDB returns a recovery workout for the database, empty if it
// has none.
func recoveryToDB(r *RecoveryWorkout) string {
	if r == nil {
		return ""
	}
	data, err := json.Marshal(r)
	if err != nil {
		// The workout is plain values, which always marshal.
		panic(fmt.Sprintf("marshaling recovery workout: %v", err))
	}
	return string(data)
}

// recoveryFromDB returns a recovery workout from the database, nil if it
// has none.
func recoveryFromDB(b []byte) *RecoveryWorkout {
	if len(b) == 0 {
		return nil
	}
	var r RecoveryWorkout
	if err := json.Unmarshal(b, &r); err != nil {
		return nil
	}
	return &r
}
