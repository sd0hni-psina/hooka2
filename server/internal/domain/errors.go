package domain

import (
	"errors"
	"strconv"
)

// Sentinel errors. The HTTP layer maps each one to a code from tech.md section 8.
var (
	ErrValidation         = errors.New("validation failed")
	ErrUnauthorized       = errors.New("unauthorized")
	ErrForbidden          = errors.New("forbidden")
	ErrNotFound           = errors.New("not found")
	ErrInvalidJoinCode    = errors.New("invalid join code")
	ErrRateLimited        = errors.New("rate limited")
	ErrTableOccupied      = errors.New("table occupied")
	ErrTableBusy          = errors.New("table busy")
	ErrHookahFinished     = errors.New("hookah finished")
	ErrEventNotActionable = errors.New("event not actionable")
	ErrPlanInvalid        = errors.New("plan invalid")
)

// Validation reasons from tech.md section 19.1.
const (
	ReasonRequired = "required"
	ReasonTooShort = "too_short"
	ReasonTooLong  = "too_long"
	ReasonInvalid  = "invalid"
	ReasonTaken    = "taken"
)

// ValidationError carries per-field reasons for details.fields.
type ValidationError struct {
	Fields map[string]string
}

func (e *ValidationError) Error() string        { return "validation failed" }
func (e *ValidationError) Is(target error) bool { return target == ErrValidation }

// Add records the first reason for a field; later reasons for the same field are ignored.
func (e *ValidationError) Add(field, reason string) {
	if e.Fields == nil {
		e.Fields = map[string]string{}
	}
	if _, ok := e.Fields[field]; !ok {
		e.Fields[field] = reason
	}
}

// Err returns nil when no field failed, so callers can `return v.Err()`.
func (e *ValidationError) Err() error {
	if len(e.Fields) == 0 {
		return nil
	}
	return e
}

type RateLimitError struct {
	RetryAfterSec int
}

func (e *RateLimitError) Error() string {
	return "rate limited, retry after " + strconv.Itoa(e.RetryAfterSec) + "s"
}
func (e *RateLimitError) Is(target error) bool { return target == ErrRateLimited }

// PlanError names the offending field so the client can show the reason next to it.
type PlanError struct {
	Field  string
	Reason string
}

func (e *PlanError) Error() string        { return "plan invalid: " + e.Field + ": " + e.Reason }
func (e *PlanError) Is(target error) bool { return target == ErrPlanInvalid }
