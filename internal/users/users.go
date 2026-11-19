package users

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/bs-tools/trailrunn.ing/internal/store"
	"github.com/bs-tools/trailrunn.ing/internal/users/internal/gen"
	"github.com/jackc/pgx/v5/pgtype"
)

type User interface {
	ID() pgtype.UUID
	// IsAdmin returns whether the user is an administrator, who can invite
	// new users.
	IsAdmin() bool
}

// Repository allows storing and querying of users and related data.
type Repository struct {
	log     *slog.Logger
	querier *gen.Queries
	db      *store.DB
}

// NewRepository creates a new Repository from the provided logger and database.
func NewRepository(log *slog.Logger, db *store.DB) (*Repository, error) {
	return &Repository{
		log:     log,
		querier: gen.New(db),
		db:      db,
	}, nil
}

// SetTimeZone sets the time zone the user lives in, an IANA name, or empty
// to unset it.
func (r *Repository) SetTimeZone(ctx context.Context, userID pgtype.UUID, timeZone string) error {
	if err := r.querier.SetTimeZone(ctx, &gen.SetTimeZoneParams{ID: userID, TimeZone: timeZone}); err != nil {
		return fmt.Errorf("failed to set time zone: %w", err)
	}
	return nil
}
