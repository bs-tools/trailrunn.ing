package route

import (
	"context"

	"connectrpc.com/connect/v2"
	"github.com/bs-tools/trailrunn.ing/internal/geo"
	"github.com/bs-tools/trailrunn.ing/internal/routes"
	"github.com/jackc/pgx/v5/pgtype"
)

// StravaSegments gets Strava segments, to make routes of them.
type StravaSegments interface {
	// Track returns a segment's name, its link on Strava and its path.
	// segment is a link to the segment or its ID. Errors are Connect
	// errors for users.
	Track(ctx context.Context, userID pgtype.UUID, segment string) (name, link string, points []geo.Point, err error)
}

// SetStravaSegments sets where Strava segments come from. Without it,
// routes can't be made from segments, as on a server without Strava.
func (s *Service) SetStravaSegments(segments StravaSegments) {
	s.segments = segments
}

// segmentTrack returns the track of a Strava segment, checked with
// checkTrack, with a description linking to the segment.
func (s *Service) segmentTrack(ctx context.Context, userID pgtype.UUID, segment string) (*recording, string, error) {
	if s.segments == nil {
		return nil, "", connect.NewError(connect.CodeFailedPrecondition,
			"Strava isn't set up on this server, so routes can't be made from segments")
	}
	name, link, points, err := s.segments.Track(ctx, userID, segment)
	if err != nil {
		return nil, "", err
	}
	if err := checkTrack(points); err != nil {
		return nil, "", err
	}
	// Segments are stored like imported GPX files, which they are on
	// sites like doogal.co.uk: a path recorded elsewhere.
	return &recording{points: points, name: name, source: routes.SourceGPX}, "From the Strava segment " + link, nil
}
