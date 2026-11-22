package plans

import (
	"context"
	"fmt"
	"math"
	"time"

	"github.com/bs-tools/trailrunn.ing/internal/plans/internal/gen"
	"github.com/bs-tools/trailrunn.ing/internal/store"
	"github.com/jackc/pgx/v5/pgtype"
)

// GearArea is a part of the body gear keeps warm, which feedback rates.
type GearArea string

// The parts of the body. Their values are stored, so they mustn't change.
const (
	GearAreaLegs  GearArea = "legs"
	GearAreaBody  GearArea = "body"
	GearAreaHands GearArea = "hands"
	GearAreaHead  GearArea = "head"
)

// GearAreas are the parts of the body, in the order they're listed.
var GearAreas = []GearArea{GearAreaLegs, GearAreaBody, GearAreaHands, GearAreaHead}

// GearFeel is how warm a part of the body was. Its values are stored, so
// they mustn't change.
type GearFeel string

const (
	GearFeelUnrated   GearFeel = ""
	GearFeelTooCold   GearFeel = "too_cold"
	GearFeelJustRight GearFeel = "just_right"
	GearFeelTooWarm   GearFeel = "too_warm"
)

// GearFeedback is how the gear felt on a workout, by part of the body.
type GearFeedback struct {
	Feels map[GearArea]GearFeel
	Notes string
	// Whether it was wet: raining, or wet underfoot and from above.
	Wet        bool
	UpdateTime time.Time
}

// ComfortKey is what feedback is learned for: a part of the body, on dry
// or wet workouts. Feeling cold in the rain says nothing about dry days.
type ComfortKey struct {
	Area GearArea
	Wet  bool
}

// GearComfort is what the user's feedback taught about a part of the
// body, on dry or wet workouts: how much colder than it feels to dress it
// for.
type GearComfort struct {
	// In degrees Celsius: positive if the user runs cold there, so it's
	// dressed for a colder temperature, negative if they run warm.
	OffsetCelsius float64
	// How many workouts' feedback rated it.
	Ratings int
}

// How feedback adjusts the temperature a part of the body is dressed for:
// each "too cold" lowers it by comfortStepCelsius, each "too warm" raises
// it, and "just right" keeps it, up to maxComfortOffsetCelsius either way.
// Replaying the ratings in order converges on what the user likes, and
// keeps it while they're comfortable.
const (
	comfortStepCelsius      = 1.5
	maxComfortOffsetCelsius = 6
)

// comfortFrom returns what feedback, the latest first, taught.
func comfortFrom(rows []*gen.ListRecentGearFeedbackRow) map[ComfortKey]GearComfort {
	comfort := make(map[ComfortKey]GearComfort)
	// Replayed from the oldest.
	for i := len(rows) - 1; i >= 0; i-- {
		row := rows[i]
		for area, feel := range map[GearArea]pgtype.Text{GearAreaLegs: row.Legs, GearAreaBody: row.Body, GearAreaHands: row.Hands, GearAreaHead: row.Head} {
			if !feel.Valid {
				continue
			}
			key := ComfortKey{Area: area, Wet: row.Wet}
			c := comfort[key]
			switch GearFeel(feel.String) {
			case GearFeelTooCold:
				c.OffsetCelsius += comfortStepCelsius
			case GearFeelTooWarm:
				c.OffsetCelsius -= comfortStepCelsius
			}
			c.OffsetCelsius = math.Max(-maxComfortOffsetCelsius, math.Min(maxComfortOffsetCelsius, c.OffsetCelsius))
			c.Ratings++
			comfort[key] = c
		}
	}
	return comfort
}

// queueGearFeedback queues reading the user's latest feedback on their
// gear into rows, for comfortFrom.
func queueGearFeedback(ctx context.Context, b *store.Batch, userID pgtype.UUID, rows *[]*gen.ListRecentGearFeedbackRow) {
	b.Read(func(db store.Querier) error {
		var err error
		*rows, err = gen.New(db).ListRecentGearFeedback(ctx, userID)
		return err
	})
}

// gearComfort reads what the user's feedback taught.
func (r *Repository) gearComfort(ctx context.Context, userID pgtype.UUID) (map[ComfortKey]GearComfort, error) {
	rows, err := r.q.ListRecentGearFeedback(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to list gear feedback: %w", err)
	}
	return comfortFrom(rows), nil
}

// GetGearFeedback gets the feedback on one of the user's workouts, nil if
// there's none.
func (r *Repository) GetGearFeedback(ctx context.Context, userID, workoutID pgtype.UUID) (*GearFeedback, error) {
	rows, err := r.q.GetGearFeedback(ctx, &gen.GetGearFeedbackParams{WorkoutID: workoutID, UserID: userID})
	if err != nil {
		return nil, fmt.Errorf("failed to get gear feedback: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return gearFeedbackFromDB(rows[0]), nil
}

// SetGearFeedback replaces the feedback on one of the user's workouts, or
// deletes it if f rates nothing and has no notes. It returns
// store.ErrNotFound if the workout isn't the user's.
func (r *Repository) SetGearFeedback(ctx context.Context, userID, workoutID pgtype.UUID, f *GearFeedback) (*GearFeedback, error) {
	rated := f.Notes != ""
	for _, feel := range f.Feels {
		rated = rated || feel != GearFeelUnrated
	}
	if !rated {
		if _, err := r.GetWorkout(ctx, userID, workoutID); err != nil {
			return nil, err
		}
		if err := r.q.DeleteGearFeedback(ctx, &gen.DeleteGearFeedbackParams{WorkoutID: workoutID, UserID: userID}); err != nil {
			return nil, fmt.Errorf("failed to delete gear feedback: %w", err)
		}
		return nil, nil
	}
	feel := func(area GearArea) pgtype.Text {
		v := f.Feels[area]
		return pgtype.Text{String: string(v), Valid: v != GearFeelUnrated}
	}
	rows, err := r.q.SetGearFeedback(ctx, &gen.SetGearFeedbackParams{
		WorkoutID: workoutID,
		UserID:    userID,
		Legs:      feel(GearAreaLegs),
		Body:      feel(GearAreaBody),
		Hands:     feel(GearAreaHands),
		Head:      feel(GearAreaHead),
		Notes:     f.Notes,
		Wet:       f.Wet,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set gear feedback: %w", err)
	}
	if len(rows) == 0 {
		return nil, store.ErrNotFound
	}
	return gearFeedbackFromDB(rows[0]), nil
}

func gearFeedbackFromDB(row *gen.GearFeedback) *GearFeedback {
	f := &GearFeedback{Feels: map[GearArea]GearFeel{}, Notes: row.Notes, Wet: row.Wet, UpdateTime: row.UpdateTime.Time}
	for area, feel := range map[GearArea]pgtype.Text{GearAreaLegs: row.Legs, GearAreaBody: row.Body, GearAreaHands: row.Hands, GearAreaHead: row.Head} {
		if feel.Valid {
			f.Feels[area] = GearFeel(feel.String)
		}
	}
	return f
}
