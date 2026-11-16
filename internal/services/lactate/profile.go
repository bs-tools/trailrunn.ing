package lactate

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/bs-tools/trailrunn.ing/internal/authn"
	lactatev1 "github.com/bs-tools/trailrunn.ing/internal/gen/lactate/v1"
	"github.com/bs-tools/trailrunn.ing/internal/lactate"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// ProfileRepository reads the user's lactate profile and tests.
type ProfileRepository interface {
	GetProfile(ctx context.Context, userID pgtype.UUID) (*lactate.Profile, error)
	ListTests(ctx context.Context, userID pgtype.UUID) ([]*lactate.Test, error)
}

// ExpectedProfile returns what the user's next lactate test goes by: their
// profile, or until it's set, the aerobic threshold of their latest test
// that found one. Tests from before profiles were stored didn't set it.
func ExpectedProfile(ctx context.Context, repo ProfileRepository, userID pgtype.UUID) (*lactate.Profile, error) {
	profile, err := repo.GetProfile(ctx, userID)
	if err != nil || profile != nil {
		return profile, err
	}
	tests, err := repo.ListTests(ctx, userID)
	if err != nil {
		return nil, err
	}
	profile = &lactate.Profile{}
	// The tests are listed with the latest first.
	for _, test := range tests {
		if threshold := test.HeartRateAt(lactate.AerobicThresholdMmolPerLiter); threshold > 0 {
			profile.AerobicThresholdHeartRateBPM = threshold
			break
		}
	}
	return profile, nil
}

func (s *Service) GetLactateProfile(ctx context.Context, _ *lactatev1.GetLactateProfileRequest) (*lactatev1.GetLactateProfileResponse, error) {
	userID, ok := authn.GetUserID(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, "")
	}
	profile, err := ExpectedProfile(ctx, s.tests, userID)
	if err != nil {
		return nil, s.toConnectError(ctx, err, "failed to get lactate profile")
	}
	return lactatev1.GetLactateProfileResponse_builder{Profile: profileToProto(profile)}.Build(), nil
}

func (s *Service) SetLactateProfile(ctx context.Context, req *lactatev1.SetLactateProfileRequest) (*lactatev1.SetLactateProfileResponse, error) {
	userID, ok := authn.GetUserID(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, "")
	}
	p := req.GetProfile()
	threshold := p.GetAerobicThresholdHeartRateBpm()
	if p.HasAerobicThresholdHeartRateBpm() && (threshold < minHeartRateBPM || threshold > maxHeartRateBPM) {
		return nil, connect.Errorf(connect.CodeInvalidArgument, "profile.aerobic_threshold_heart_rate_bpm must be from %d to %d, or unset for a test with even steps", minHeartRateBPM, maxHeartRateBPM)
	}
	set, err := s.tests.SetProfile(ctx, userID, &lactate.Profile{AerobicThresholdHeartRateBPM: threshold})
	if err != nil {
		return nil, s.toConnectError(ctx, err, "failed to set lactate profile")
	}
	return lactatev1.SetLactateProfileResponse_builder{Profile: profileToProto(set)}.Build(), nil
}

func profileToProto(p *lactate.Profile) *lactatev1.LactateProfile {
	b := lactatev1.LactateProfile_builder{}
	if p.AerobicThresholdHeartRateBPM > 0 {
		b.AerobicThresholdHeartRateBpm = &p.AerobicThresholdHeartRateBPM
	}
	if !p.UpdateTime.IsZero() {
		b.UpdateTime = timestamppb.New(p.UpdateTime)
	}
	return b.Build()
}
