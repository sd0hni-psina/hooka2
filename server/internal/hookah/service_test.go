package hookah_test

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"hookah/server/internal/apitest"
	"hookah/server/internal/domain"
	"hookah/server/internal/staff"
)

func TestHallState(t *testing.T) {
	e := apitest.New(t, true)
	ctx := context.Background()
	fx, svc := e.Fx, e.Hookah

	t.Run("second hall shows its eight tables and the busy hookah as yellow", func(t *testing.T) {
		st, err := svc.HallState(ctx, fx.VenueID, fx.HallIDs[1], []string{fx.Masters[0].StaffID}, time.Now())
		require.NoError(t, err)
		require.Equal(t, "Второй зал", st.Hall.Name)
		require.Len(t, st.Tables, 8)
		require.Len(t, st.Hookahs, 1)
		h := st.Hookahs[0]
		require.Equal(t, fx.BusyHookahID, h.ID)
		require.Equal(t, fx.TableIDs["15"], h.TableID)
		require.Equal(t, domain.StatusYellow, h.Status)
		require.Equal(t, "Айдос", h.StartedByName)
		require.Len(t, h.Events, 7)
		require.Equal(t, "Ерлан", *h.Events[0].ConfirmedByName)
		require.Equal(t, "Айдос", *h.Events[1].ClaimedByName)
		require.Equal(t, []staff.Online{{ID: fx.Masters[0].StaffID, DisplayName: "Ерлан"}}, st.StaffOnline)
	})

	t.Run("first hall shows the fresh hookah as green", func(t *testing.T) {
		st, err := svc.HallState(ctx, fx.VenueID, fx.HallIDs[0], nil, time.Now())
		require.NoError(t, err)
		require.Len(t, st.Tables, 10)
		require.Len(t, st.Hookahs, 1)
		require.Equal(t, domain.StatusGreen, st.Hookahs[0].Status)
		require.Empty(t, st.StaffOnline)
	})

	t.Run("hall of another venue or a malformed id is not found", func(t *testing.T) {
		_, err := svc.HallState(ctx, "00000000-0000-4000-8000-000000000000", fx.HallIDs[0], nil, time.Now())
		require.ErrorIs(t, err, domain.ErrNotFound)
		_, err = svc.HallState(ctx, fx.VenueID, "15", nil, time.Now())
		require.ErrorIs(t, err, domain.ErrNotFound)
	})
}
