package account

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"hookah/server/internal/auth"
)

// An unknown email burns a comparison against dummyHash so its latency matches
// a real account. That only works while both use the same cost.
func TestDummyHashFollowsThePasswordCost(t *testing.T) {
	for _, cost := range []int{bcrypt.MinCost, bcrypt.MinCost + 1, auth.PasswordCost} {
		svc, err := NewService(nil, nil, nil, nil, nil, nil, cost)
		require.NoError(t, err)
		got, err := bcrypt.Cost([]byte(svc.dummyHash))
		require.NoError(t, err)
		require.Equal(t, cost, got)
	}
}

func TestNewServiceRejectsAnImpossibleCost(t *testing.T) {
	_, err := NewService(nil, nil, nil, nil, nil, nil, bcrypt.MaxCost+1)
	require.Error(t, err)
}
