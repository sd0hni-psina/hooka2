package account

import (
	"testing"

	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/bcrypt"

	"hookah/server/internal/auth"
)

// An unknown email burns a comparison against dummyHash so its latency matches
// a real account. That only works while both use the same cost.
func TestDummyHashHasTheCurrentCost(t *testing.T) {
	cost, err := bcrypt.Cost([]byte(dummyHash))
	require.NoError(t, err)
	require.Equal(t, auth.PasswordCost, cost)
}
