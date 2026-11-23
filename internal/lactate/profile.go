package lactate

import (
	"context"
	"fmt"
	"time"

	"github.com/bs-tools/trailrunn.ing/internal/lactate/internal/gen"
	"github.com/jackc/pgx/v5/pgtype"
)

// Profile is what the user's next lactate test goes by.
type Profile struct {
	// Where the test expects the aerobic threshold, which it has closer
	// stages around, or 0 for a test with even steps.
	AerobicThresholdHeartRateBPM int32
	// The user's maximum heart rate, or 0 to estimate it from their
	// activities, see EstimateMaxHeartRate.
	MaxHeartRateBPM int32
	UpdateTime      time.Time
}

// GetProfile gets the user's lactate profile, nil if it was never set, by
// them or by a test.
func (r *Repository) GetProfile(ctx context.Context, userID pgtype.UUID) (*Profile, error) {
	rows, err := r.q.GetLactateProfile(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to get lactate profile: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return profileFromDB(rows[0]), nil
}

// SetProfile replaces the user's lactate profile.
func (r *Repository) SetProfile(ctx context.Context, userID pgtype.UUID, p *Profile) (*Profile, error) {
	row, err := r.q.SetLactateProfile(ctx, &gen.SetLactateProfileParams{
		UserID:                       userID,
		AerobicThresholdHeartRateBpm: pgtype.Int4{Int32: p.AerobicThresholdHeartRateBPM, Valid: p.AerobicThresholdHeartRateBPM > 0},
		MaxHeartRateBpm:              pgtype.Int4{Int32: p.MaxHeartRateBPM, Valid: p.MaxHeartRateBPM > 0},
	})
	if err != nil {
		return nil, fmt.Errorf("failed to set lactate profile: %w", err)
	}
	return profileFromDB(row), nil
}

// EstimateMaxHeartRate estimates the user's maximum heart rate as the
// highest of their activities in the past year, 0 if none has one.
func (r *Repository) EstimateMaxHeartRate(ctx context.Context, userID pgtype.UUID) (int32, error) {
	bpm, err := r.q.EstimateMaxHeartRate(ctx, userID)
	if err != nil {
		return 0, fmt.Errorf("failed to estimate the maximum heart rate: %w", err)
	}
	return bpm, nil
}

// setProfileFromTest sets the user's expected aerobic threshold to the one a
// stored test found, if it found one and is their latest test, so the next
// test is run around it.
func (r *Repository) setProfileFromTest(ctx context.Context, test *Test) error {
	threshold := test.HeartRateAt(AerobicThresholdMmolPerLiter)
	if threshold <= 0 {
		return nil
	}
	if err := r.q.SetLactateProfileFromTest(ctx, &gen.SetLactateProfileFromTestParams{
		TestID:                       test.ID,
		AerobicThresholdHeartRateBpm: threshold,
	}); err != nil {
		return fmt.Errorf("failed to set the lactate profile from the test: %w", err)
	}
	return nil
}

func profileFromDB(row *gen.LactateProfile) *Profile {
	return &Profile{
		AerobicThresholdHeartRateBPM: row.AerobicThresholdHeartRateBpm.Int32,
		MaxHeartRateBPM:              row.MaxHeartRateBpm.Int32,
		UpdateTime:                   row.UpdateTime.Time,
	}
}
