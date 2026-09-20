package staff_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"hookah/server/internal/apitest"
	"hookah/server/internal/domain"
	"hookah/server/seed"
)

func TestStaffList(t *testing.T) {
	e := apitest.New(t, true)
	res := e.Do("GET", "/venues/current/staff", e.TokenFor(e.Fx.Owner), nil)
	require.Equal(t, 200, res.Status)
	var list []domain.Staff
	res.Decode(t, &list)
	require.Len(t, list, 4)
	require.Equal(t, domain.RoleOwner, list[0].Role)
	require.Equal(t, "Арсен", list[0].DisplayName)

	require.Equal(t, 403, e.Do("GET", "/venues/current/staff", e.TokenFor(e.Fx.Masters[0]), nil).Status)
}

func TestBlockStaff(t *testing.T) {
	block := func(e *apitest.Env, token, staffID string, blocked bool) apitest.Response {
		return e.Do("PATCH", "/staff/"+staffID, token, map[string]any{"blocked": blocked})
	}

	t.Run("a blocked master loses access and cannot join again", func(t *testing.T) {
		e := apitest.New(t, true)
		master := e.Fx.Masters[0]
		masterToken := e.TokenFor(master)
		require.Equal(t, 200, e.Do("GET", "/me", masterToken, nil).Status)

		res := block(e, e.TokenFor(e.Fx.Owner), master.StaffID, true)
		require.Equal(t, 200, res.Status, string(res.Raw))
		var st domain.Staff
		res.Decode(t, &st)
		require.True(t, st.Blocked)

		require.Equal(t, 403, e.Do("GET", "/me", masterToken, nil).Status)
		require.Equal(t, 0, e.Count(`staff_sessions where staff_id = $1 and revoked_at is null`, master.StaffID))

		rejoin := e.Do("POST", "/join", "", apitest.With(
			map[string]any{"code": seed.JoinCode, "display_name": master.Name}, apitest.Device("device-blocked-1")))
		require.Equal(t, 403, rejoin.Status)
		require.Equal(t, "forbidden", rejoin.Code())
	})

	t.Run("blocking closes the open socket", func(t *testing.T) {
		e := apitest.New(t, true)
		master := e.Fx.Masters[0]
		ws, _ := e.Subscribe(e.TokenFor(master), e.Fx.HallIDs[0])
		require.Equal(t, 200, block(e, e.TokenFor(e.Fx.Owner), master.StaffID, true).Status)
		require.True(t, ws.Closed(), "the connection of a blocked master is dropped")
	})

	t.Run("unblocking lets the master join again", func(t *testing.T) {
		e := apitest.New(t, true)
		master := e.Fx.Masters[1]
		owner := e.TokenFor(e.Fx.Owner)
		require.Equal(t, 200, block(e, owner, master.StaffID, true).Status)
		require.Equal(t, 200, block(e, owner, master.StaffID, false).Status)

		res := e.Do("POST", "/join", "", apitest.With(
			map[string]any{"code": seed.JoinCode, "display_name": master.Name}, apitest.Device("device-unblocked-1")))
		require.Equal(t, 200, res.Status, string(res.Raw))
	})

	t.Run("the owner and yourself are off limits", func(t *testing.T) {
		e := apitest.New(t, true)
		owner := e.TokenFor(e.Fx.Owner)
		require.Equal(t, 403, block(e, owner, e.Fx.Owner.StaffID, true).Status)

		e.Exec(`update staff set role = 'admin' where id = $1`, e.Fx.Masters[0].StaffID)
		admin := e.TokenFor(seedMember(e, 0, domain.RoleAdmin))
		require.Equal(t, 403, block(e, admin, e.Fx.Masters[0].StaffID, true).Status, "blocking yourself")
		require.Equal(t, 200, block(e, admin, e.Fx.Masters[1].StaffID, true).Status, "an admin blocks a master")
	})

	t.Run("only the owner moves roles", func(t *testing.T) {
		e := apitest.New(t, true)
		owner := e.TokenFor(e.Fx.Owner)
		target := e.Fx.Masters[1].StaffID

		res := e.Do("PATCH", "/staff/"+target, owner, map[string]any{"role": "admin"})
		require.Equal(t, 200, res.Status, string(res.Raw))
		require.Equal(t, "admin", res.Body["role"])

		e.Exec(`update staff set role = 'admin' where id = $1`, e.Fx.Masters[0].StaffID)
		admin := e.TokenFor(seedMember(e, 0, domain.RoleAdmin))
		require.Equal(t, 403, e.Do("PATCH", "/staff/"+target, admin, map[string]any{"role": "master"}).Status)
		require.Equal(t, 403, e.Do("PATCH", "/staff/"+e.Fx.Owner.StaffID, owner, map[string]any{"role": "admin"}).Status)
		require.Equal(t, 400, e.Do("PATCH", "/staff/"+target, owner, map[string]any{"role": "owner"}).Status)
		require.Equal(t, 400, e.Do("PATCH", "/staff/"+target, owner, map[string]any{}).Status)
		require.Equal(t, 404, e.Do("PATCH", "/staff/00000000-0000-4000-8000-000000000000", owner, map[string]any{"blocked": true}).Status)
	})
}

// seedMember re-reads a seeded master with the role it has now.
func seedMember(e *apitest.Env, index int, role domain.Role) seed.Member {
	m := e.Fx.Masters[index]
	m.Role = role
	return m
}
