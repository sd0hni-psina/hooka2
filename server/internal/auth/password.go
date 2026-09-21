package auth

import (
	"errors"
	"fmt"

	"golang.org/x/crypto/bcrypt"
)

// PasswordCost is the bcrypt cost of new hashes, tech.md section 19.16.
const PasswordCost = 12

// HashPassword hashes at cost. Callers pass PasswordCost, tests pass a cheap
// cost so a suite does not spend minutes in bcrypt.
func HashPassword(password string, cost int) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(h), nil
}

// NeedsRehash reports whether hash was made at a lower cost than cost, so a
// login that just proved the password can replace it.
func NeedsRehash(hash string, cost int) (bool, error) {
	c, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		return false, fmt.Errorf("hash cost: %w", err)
	}
	return c < cost, nil
}

// CheckPassword reports whether password matches hash. A malformed hash is an
// error, a wrong password is not.
func CheckPassword(hash, password string) (bool, error) {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("check password: %w", err)
	}
	return true, nil
}
