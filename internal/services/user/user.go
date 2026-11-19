package user

import (
	"context"
	"strings"
	"time"

	"connectrpc.com/connect/v2"
	"github.com/bs-tools/trailrunn.ing/internal/authn"
	usersv1 "github.com/bs-tools/trailrunn.ing/internal/gen/users/v1"
	"github.com/bs-tools/trailrunn.ing/internal/store"
	"github.com/bs-tools/trailrunn.ing/internal/users"
)

func (s *Service) GetCurrentUser(ctx context.Context, req *usersv1.GetCurrentUserRequest) (*usersv1.GetCurrentUserResponse, error) {
	user, ok := authn.GetUser(ctx)
	if !ok {
		// Being signed out is an answer to who is signed in, not an error:
		// the web app asks on every page load, and Cloud Run logs every 401
		// as a warning. Invalid credentials are still rejected, by the
		// authentication middleware.
		return usersv1.GetCurrentUserResponse_builder{}.Build(), nil
	}
	u, err := userToProto(user)
	if err != nil {
		return nil, err
	}
	return usersv1.GetCurrentUserResponse_builder{User: u}.Build(), nil
}

func (s *Service) SetTimeZone(ctx context.Context, req *usersv1.SetTimeZoneRequest) (*usersv1.SetTimeZoneResponse, error) {
	user, ok := authn.GetUser(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, "")
	}
	timeZone := strings.TrimSpace(req.GetTimeZone())
	// "Local" loads as the server's zone, and names without a slash are
	// abbreviations and offsets like "EST" and "UTC", which have no daylight
	// saving time, not where someone lives.
	if timeZone != "" {
		if _, err := time.LoadLocation(timeZone); err != nil || !strings.Contains(timeZone, "/") {
			return nil, connect.Errorf(connect.CodeInvalidArgument, "time_zone %q isn't an IANA time zone name like \"America/Vancouver\" or \"Europe/Zurich\"", timeZone)
		}
	}
	if err := s.users.SetTimeZone(ctx, user.ID(), timeZone); err != nil {
		return nil, s.toConnectError(ctx, err, "failed to set time zone")
	}
	passkeyUser, ok := user.(*users.PasskeyUser)
	if !ok {
		return nil, connect.Errorf(connect.CodeInternal, "unknown user type: %T", user)
	}
	updated := *passkeyUser
	updated.TimeZone = timeZone
	u, err := userToProto(&updated)
	if err != nil {
		return nil, err
	}
	return usersv1.SetTimeZoneResponse_builder{User: u}.Build(), nil
}

func userToProto(user users.User) (*usersv1.User, error) {
	switch u := user.(type) {
	case *users.PasskeyUser:
		return usersv1.User_builder{
			Id:        store.UUIDToString(user.ID()),
			Name:      u.Name,
			AvatarUrl: AvatarURL(u.AvatarKey),
			Admin:     u.IsAdmin(),
			TimeZone:  u.TimeZone,
		}.Build(), nil
	default:
		return nil, connect.Errorf(connect.CodeInternal, "unknown user type: %T", u)
	}
}
