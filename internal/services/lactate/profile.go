package lactate

import (
	"context"
	"slices"

	"connectrpc.com/connect/v2"
	"github.com/bs-tools/trailrunn.ing/internal/authn"
	lactatev1 "github.com/bs-tools/trailrunn.ing/internal/gen/lactate/v1"
	"github.com/bs-tools/trailrunn.ing/internal/lactate"
	"github.com/bs-tools/trailrunn.ing/internal/plans"
	"github.com/jackc/pgx/v5/pgtype"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// The bounds of a maximum heart rate.
const minMaxHeartRateBPM = 100

// heartRates are the user's heart rates the profile shows.
type heartRates struct {
	profile *lactate.Profile
	// The aerobic threshold of the zones: the profile's, or else the latest
	// test's.
	aerobic   int32
	anaerobic int32
	estimated int32
}

// userHeartRates reads the user's profile, with the thresholds of their
// latest tests and their estimated maximum heart rate. Until the profile
// is set, its aerobic threshold is that of the latest test that found one:
// tests from before profiles were stored didn't set it.
func (s *Service) userHeartRates(ctx context.Context, userID pgtype.UUID) (*heartRates, error) {
	profile, err := s.tests.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	tests, err := s.tests.ListTests(ctx, userID)
	if err != nil {
		return nil, err
	}
	h := &heartRates{profile: profile}
	// The tests are listed with the latest first.
	for _, test := range tests {
		if h.aerobic == 0 {
			h.aerobic = test.HeartRateAt(lactate.AerobicThresholdMmolPerLiter)
		}
		if h.anaerobic == 0 {
			h.anaerobic = test.HeartRateAt(lactate.AnaerobicThresholdMmolPerLiter)
		}
	}
	if h.profile == nil {
		h.profile = &lactate.Profile{AerobicThresholdHeartRateBPM: h.aerobic}
	} else if h.profile.AerobicThresholdHeartRateBPM > 0 {
		h.aerobic = h.profile.AerobicThresholdHeartRateBPM
	}
	if h.estimated, err = s.tests.EstimateMaxHeartRate(ctx, userID); err != nil {
		return nil, err
	}
	return h, nil
}

func (s *Service) GetLactateProfile(ctx context.Context, _ *lactatev1.GetLactateProfileRequest) (*lactatev1.GetLactateProfileResponse, error) {
	userID, ok := authn.GetUserID(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, "")
	}
	h, err := s.userHeartRates(ctx, userID)
	if err != nil {
		return nil, s.toConnectError(ctx, err, "failed to get lactate profile")
	}
	return lactatev1.GetLactateProfileResponse_builder{Profile: h.toProto()}.Build(), nil
}

func (s *Service) SetLactateProfile(ctx context.Context, req *lactatev1.SetLactateProfileRequest) (*lactatev1.SetLactateProfileResponse, error) {
	userID, ok := authn.GetUserID(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, "")
	}
	paths := req.GetUpdateMask().GetPaths()
	for _, path := range paths {
		if !slices.Contains([]string{"aerobic_threshold_heart_rate_bpm", "aerobicThresholdHeartRateBpm", "max_heart_rate_bpm", "maxHeartRateBpm"}, path) {
			return nil, connect.Errorf(connect.CodeInvalidArgument, "invalid update mask path %q: only aerobic_threshold_heart_rate_bpm and max_heart_rate_bpm can be set", path)
		}
	}
	set := func(field string) bool {
		return len(paths) == 0 || slices.ContainsFunc(paths, func(p string) bool {
			return p == field || p == map[string]string{
				"aerobic_threshold_heart_rate_bpm": "aerobicThresholdHeartRateBpm",
				"max_heart_rate_bpm":               "maxHeartRateBpm",
			}[field]
		})
	}
	p := req.GetProfile()
	threshold, maxHR := p.GetAerobicThresholdHeartRateBpm(), p.GetMaxHeartRateBpm()
	if set("aerobic_threshold_heart_rate_bpm") && p.HasAerobicThresholdHeartRateBpm() && (threshold < minHeartRateBPM || threshold > maxHeartRateBPM) {
		return nil, connect.Errorf(connect.CodeInvalidArgument, "profile.aerobic_threshold_heart_rate_bpm must be from %d to %d, or unset for a test with even steps", minHeartRateBPM, maxHeartRateBPM)
	}
	if set("max_heart_rate_bpm") && p.HasMaxHeartRateBpm() && (maxHR < minMaxHeartRateBPM || maxHR > maxHeartRateBPM) {
		return nil, connect.Errorf(connect.CodeInvalidArgument, "profile.max_heart_rate_bpm must be from %d to %d, or unset to estimate it from activities", minMaxHeartRateBPM, maxHeartRateBPM)
	}
	existing, err := s.tests.GetProfile(ctx, userID)
	if err != nil {
		return nil, s.toConnectError(ctx, err, "failed to get lactate profile")
	}
	profile := lactate.Profile{}
	if existing != nil {
		profile = *existing
	}
	if set("aerobic_threshold_heart_rate_bpm") {
		profile.AerobicThresholdHeartRateBPM = threshold
	}
	if set("max_heart_rate_bpm") {
		profile.MaxHeartRateBPM = maxHR
	}
	if _, err := s.tests.SetProfile(ctx, userID, &profile); err != nil {
		return nil, s.toConnectError(ctx, err, "failed to set lactate profile")
	}
	h, err := s.userHeartRates(ctx, userID)
	if err != nil {
		return nil, s.toConnectError(ctx, err, "failed to get lactate profile")
	}
	return lactatev1.SetLactateProfileResponse_builder{Profile: h.toProto()}.Build(), nil
}

// zoneDescriptions say what training each zone is for.
var zoneDescriptions = [...]string{
	"Recovery and easy aerobic base",
	"Aerobic base: most of the volume, up to the aerobic threshold",
	"Tempo, between the thresholds: use sparingly",
	"Threshold intervals and above, up to the maximum",
}

func (h *heartRates) toProto() *lactatev1.LactateProfile {
	p := h.profile
	b := lactatev1.LactateProfile_builder{
		EstimatedMaxHeartRateBpm:       h.estimated,
		AnaerobicThresholdHeartRateBpm: h.anaerobic,
	}
	if p.AerobicThresholdHeartRateBPM > 0 {
		b.AerobicThresholdHeartRateBpm = &p.AerobicThresholdHeartRateBPM
	}
	if p.MaxHeartRateBPM > 0 {
		b.MaxHeartRateBpm = &p.MaxHeartRateBPM
	}
	if !p.UpdateTime.IsZero() {
		b.UpdateTime = timestamppb.New(p.UpdateTime)
	}
	maxHR := p.MaxHeartRateBPM
	if maxHR == 0 {
		maxHR = h.estimated
	}
	if zones, ok := plans.UphillAthleteZones(h.aerobic, h.anaerobic, maxHR); ok {
		for zone := int32(1); zone <= plans.MaxHeartRateZone; zone++ {
			low, high, ok := zones.Range(zone, zone)
			switch {
			case ok:
			case zone == 3:
				// Without an anaerobic threshold, zone 3 has no top, and
				// there's no zone 4.
				low, high = zones.Zone3BPM, 0
			default:
				continue
			}
			b.Zones = append(b.Zones, lactatev1.HeartRateZone_builder{
				Zone: zone, LowBpm: low, HighBpm: high, Description: zoneDescriptions[zone-1],
			}.Build())
		}
	}
	return b.Build()
}
