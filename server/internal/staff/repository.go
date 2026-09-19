package staff

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"

	"hookah/server/internal/domain"
	"hookah/server/internal/store"
)

type Repository struct{}

func NewRepository() *Repository { return &Repository{} }

const staffColumns = `id, venue_id, display_name, role, blocked_at is not null, created_at, last_seen_at`

func scanStaff(row pgx.Row) (domain.Staff, error) {
	var s domain.Staff
	err := row.Scan(&s.ID, &s.VenueID, &s.DisplayName, &s.Role, &s.Blocked, &s.CreatedAt, &s.LastSeenAt)
	return s, err
}

func (Repository) Get(ctx context.Context, q store.Querier, venueID, id string) (domain.Staff, error) {
	s, err := scanStaff(q.QueryRow(ctx,
		`select `+staffColumns+` from staff where id = $1 and venue_id = $2`, id, venueID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Staff{}, fmt.Errorf("get staff %s: %w", id, domain.ErrNotFound)
	}
	if err != nil {
		return domain.Staff{}, fmt.Errorf("get staff %s: %w", id, err)
	}
	return s, nil
}

// SessionState reports whether the session is revoked and its staff blocked.
// found is false when the session does not belong to that staff and venue.
func (Repository) SessionState(ctx context.Context, q store.Querier, sessionID, staffID, venueID string) (found, revoked, blocked bool, err error) {
	err = q.QueryRow(ctx,
		`select s.revoked_at is not null, st.blocked_at is not null
		 from staff_sessions s join staff st on st.id = s.staff_id
		 where s.id = $1 and s.staff_id = $2 and st.venue_id = $3`,
		sessionID, staffID, venueID,
	).Scan(&revoked, &blocked)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, false, false, nil
	}
	if err != nil {
		return false, false, false, fmt.Errorf("session state: %w", err)
	}
	return true, revoked, blocked, nil
}

type Online struct {
	ID          string `json:"id"`
	DisplayName string `json:"display_name"`
}

func (Repository) ListOnline(ctx context.Context, q store.Querier, venueID string, ids []string) ([]Online, error) {
	rows, err := q.Query(ctx,
		`select id, display_name from staff where venue_id = $1 and id = any($2::uuid[]) order by display_name`, venueID, ids)
	if err != nil {
		return nil, fmt.Errorf("list online staff: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (Online, error) {
		var o Online
		return o, row.Scan(&o.ID, &o.DisplayName)
	})
	if err != nil {
		return nil, fmt.Errorf("list online staff: %w", err)
	}
	return out, nil
}
