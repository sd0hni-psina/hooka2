package staff

import (
	"context"
	"errors"
	"fmt"
	"time"

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

// FindMaster looks up a code-joined staff member by name, ignoring case.
func (Repository) FindMaster(ctx context.Context, q store.Querier, venueID, name string) (domain.Staff, bool, error) {
	s, err := scanStaff(q.QueryRow(ctx,
		`select `+staffColumns+` from staff
		 where venue_id = $1 and account_id is null and lower(display_name) = lower($2)`, venueID, name))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Staff{}, false, nil
	}
	if err != nil {
		return domain.Staff{}, false, fmt.Errorf("find master: %w", err)
	}
	return s, true, nil
}

// CreateMaster inserts a master unless a concurrent join already created one
// with that name; created is false then and the caller reads it back.
func (Repository) CreateMaster(ctx context.Context, q store.Querier, venueID, name string) (domain.Staff, bool, error) {
	s, err := scanStaff(q.QueryRow(ctx,
		`insert into staff (venue_id, display_name, role) values ($1, $2, 'master')
		 on conflict (venue_id, lower(display_name)) where account_id is null do nothing
		 returning `+staffColumns, venueID, name))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Staff{}, false, nil
	}
	if err != nil {
		return domain.Staff{}, false, fmt.Errorf("create master: %w", err)
	}
	return s, true, nil
}

func (Repository) CreateOwner(ctx context.Context, q store.Querier, venueID, accountID, name string) (domain.Staff, error) {
	s, err := scanStaff(q.QueryRow(ctx,
		`insert into staff (venue_id, account_id, display_name, role) values ($1, $2, $3, 'owner')
		 returning `+staffColumns, venueID, accountID, name))
	if err != nil {
		return domain.Staff{}, fmt.Errorf("create owner: %w", err)
	}
	return s, nil
}

// GetByAccount returns the staff row that links an account to its venue.
func (Repository) GetByAccount(ctx context.Context, q store.Querier, accountID string) (domain.Staff, error) {
	s, err := scanStaff(q.QueryRow(ctx,
		`select `+staffColumns+` from staff where account_id = $1 order by created_at limit 1`, accountID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Staff{}, fmt.Errorf("staff of account: %w", domain.ErrNotFound)
	}
	if err != nil {
		return domain.Staff{}, fmt.Errorf("staff of account: %w", err)
	}
	return s, nil
}

// UpsertSession keeps one session per staff and device: a repeated login on
// the same device rotates its refresh token and revives it.
func (Repository) UpsertSession(ctx context.Context, q store.Querier, staffID string, d domain.Device, refreshHash string, now time.Time) (string, error) {
	var id string
	err := q.QueryRow(ctx,
		`insert into staff_sessions (staff_id, device_id, platform, refresh_token_hash, last_seen_at)
		 values ($1, $2, $3, $4, $5)
		 on conflict (staff_id, device_id) do update
		   set platform = excluded.platform, refresh_token_hash = excluded.refresh_token_hash,
		       revoked_at = null, last_seen_at = excluded.last_seen_at
		 returning id`, staffID, d.DeviceID, d.Platform, refreshHash, now).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("upsert session: %w", err)
	}
	return id, nil
}

type RefreshSession struct {
	SessionID string
	Staff     domain.Staff
	Revoked   bool
	IssuedAt  time.Time
}

func (Repository) SessionByRefresh(ctx context.Context, q store.Querier, hash string) (RefreshSession, bool, error) {
	var r RefreshSession
	s := &r.Staff
	err := q.QueryRow(ctx,
		`select ss.id, ss.revoked_at is not null, ss.last_seen_at,
		        st.id, st.venue_id, st.display_name, st.role, st.blocked_at is not null, st.created_at, st.last_seen_at
		 from staff_sessions ss join staff st on st.id = ss.staff_id
		 where ss.refresh_token_hash = $1`, hash,
	).Scan(&r.SessionID, &r.Revoked, &r.IssuedAt, &s.ID, &s.VenueID, &s.DisplayName, &s.Role, &s.Blocked, &s.CreatedAt, &s.LastSeenAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return RefreshSession{}, false, nil
	}
	if err != nil {
		return RefreshSession{}, false, fmt.Errorf("session by refresh: %w", err)
	}
	return r, true, nil
}

// RotateRefresh swaps the hash only if it still matches, so two concurrent
// refreshes with one token cannot both win.
func (Repository) RotateRefresh(ctx context.Context, q store.Querier, sessionID, oldHash, newHash string, now time.Time) (bool, error) {
	tag, err := q.Exec(ctx,
		`update staff_sessions set refresh_token_hash = $3, last_seen_at = $4
		 where id = $1 and refresh_token_hash = $2 and revoked_at is null`, sessionID, oldHash, newHash, now)
	if err != nil {
		return false, fmt.Errorf("rotate refresh: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (Repository) RevokeSession(ctx context.Context, q store.Querier, sessionID string) error {
	if _, err := q.Exec(ctx,
		`update staff_sessions set revoked_at = coalesce(revoked_at, now()), expo_push_token = null where id = $1`, sessionID); err != nil {
		return fmt.Errorf("revoke session: %w", err)
	}
	return nil
}

func (Repository) SetPushToken(ctx context.Context, q store.Querier, sessionID, token string) error {
	if _, err := q.Exec(ctx, `update staff_sessions set expo_push_token = $2 where id = $1`, sessionID, token); err != nil {
		return fmt.Errorf("set push token: %w", err)
	}
	return nil
}

// LockDevice serializes join attempts of one device for the transaction.
func (Repository) LockDevice(ctx context.Context, q store.Querier, deviceID string) error {
	if _, err := q.Exec(ctx, `select pg_advisory_xact_lock(hashtextextended($1, 0))`, "join:"+deviceID); err != nil {
		return fmt.Errorf("lock device: %w", err)
	}
	return nil
}

func (Repository) RecentAttempts(ctx context.Context, q store.Querier, deviceID string, since time.Time) ([]time.Time, error) {
	rows, err := q.Query(ctx,
		`select created_at from join_attempts where device_id = $1 and created_at > $2`, deviceID, since)
	if err != nil {
		return nil, fmt.Errorf("recent attempts: %w", err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[time.Time])
	if err != nil {
		return nil, fmt.Errorf("recent attempts: %w", err)
	}
	return out, nil
}

func (Repository) RecordAttempt(ctx context.Context, q store.Querier, deviceID, code string, success bool, now time.Time) error {
	if _, err := q.Exec(ctx,
		`insert into join_attempts (device_id, code, success, created_at) values ($1, $2, $3, $4)`,
		deviceID, code, success, now); err != nil {
		return fmt.Errorf("record attempt: %w", err)
	}
	return nil
}

// ListVenue returns the whole staff of a venue, managers first.
func (Repository) ListVenue(ctx context.Context, q store.Querier, venueID string) ([]domain.Staff, error) {
	rows, err := q.Query(ctx,
		`select `+staffColumns+` from staff where venue_id = $1
		 order by case role when 'owner' then 0 when 'admin' then 1 else 2 end, display_name`, venueID)
	if err != nil {
		return nil, fmt.Errorf("list staff: %w", err)
	}
	out, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (domain.Staff, error) {
		return scanStaff(row)
	})
	if err != nil {
		return nil, fmt.Errorf("list staff: %w", err)
	}
	return out, nil
}

// SetRole changes the role of a staff member.
func (Repository) SetRole(ctx context.Context, q store.Querier, staffID string, role domain.Role) error {
	if _, err := q.Exec(ctx, `update staff set role = $2 where id = $1`, staffID, role); err != nil {
		return fmt.Errorf("set role: %w", err)
	}
	return nil
}

// SetBlocked blocks or unblocks a staff member. Blocking also revokes every
// session, tech.md section 19.11.
func (Repository) SetBlocked(ctx context.Context, q store.Querier, staffID string, blocked bool, now time.Time) error {
	if _, err := q.Exec(ctx,
		`update staff set blocked_at = case when $2 then coalesce(blocked_at, $3) else null end where id = $1`,
		staffID, blocked, now); err != nil {
		return fmt.Errorf("set blocked: %w", err)
	}
	if !blocked {
		return nil
	}
	if _, err := q.Exec(ctx,
		`update staff_sessions set revoked_at = coalesce(revoked_at, $2), expo_push_token = null
		 where staff_id = $1 and revoked_at is null`, staffID, now); err != nil {
		return fmt.Errorf("revoke sessions of blocked staff: %w", err)
	}
	return nil
}
