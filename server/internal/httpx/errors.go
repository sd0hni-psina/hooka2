package httpx

import (
	"errors"
	"log/slog"
	"net/http"

	"hookah/server/internal/domain"
)

type errorBody struct {
	Error errorPayload `json:"error"`
}

type errorPayload struct {
	Code    string         `json:"code"`
	Message string         `json:"message"`
	Details map[string]any `json:"details,omitempty"`
}

type mapping struct {
	err     error
	code    string
	status  int
	message string
}

// codes is the closed list of tech.md section 8.
var codes = []mapping{
	{domain.ErrValidation, "validation_failed", http.StatusBadRequest, "Проверьте введённые данные"},
	{domain.ErrUnauthorized, "unauthorized", http.StatusUnauthorized, "Нужно войти заново"},
	{domain.ErrForbidden, "forbidden", http.StatusForbidden, "Недостаточно прав"},
	{domain.ErrNotFound, "not_found", http.StatusNotFound, "Не найдено"},
	{domain.ErrInvalidJoinCode, "invalid_join_code", http.StatusNotFound, "Неверный код заведения"},
	{domain.ErrRateLimited, "rate_limited", http.StatusTooManyRequests, "Слишком много попыток, подождите"},
	{domain.ErrTableOccupied, "table_occupied", http.StatusConflict, "На этом столе уже есть активный кальян"},
	{domain.ErrTableBusy, "table_busy", http.StatusConflict, "Стол занят кальяном"},
	{domain.ErrHookahFinished, "hookah_finished", http.StatusConflict, "Кальян уже завершён"},
	{domain.ErrEventNotActionable, "event_not_actionable", http.StatusConflict, "Задачу нельзя подтвердить"},
	{domain.ErrPlanInvalid, "plan_invalid", http.StatusUnprocessableEntity, "План сервиса нарушает ограничения"},
}

// ErrorCode returns the machine code and HTTP status for err.
func ErrorCode(err error) (string, int) {
	for _, m := range codes {
		if errors.Is(err, m.err) {
			return m.code, m.status
		}
	}
	return "internal", http.StatusInternalServerError
}

// WriteError maps a domain error to the unified error body. Unknown errors
// become internal and are logged; their text never reaches the client.
func WriteError(w http.ResponseWriter, r *http.Request, err error) {
	p := errorPayload{Code: "internal", Message: "Внутренняя ошибка сервера"}
	status := http.StatusInternalServerError
	for _, m := range codes {
		if errors.Is(err, m.err) {
			p.Code, p.Message, status = m.code, m.message, m.status
			break
		}
	}
	p.Details = details(err)
	if status == http.StatusInternalServerError {
		slog.ErrorContext(r.Context(), "request failed", "err", err)
	}
	JSON(w, status, errorBody{Error: p})
}

func details(err error) map[string]any {
	var v *domain.ValidationError
	if errors.As(err, &v) {
		return map[string]any{"fields": v.Fields}
	}
	var rl *domain.RateLimitError
	if errors.As(err, &rl) {
		return map[string]any{"retry_after_sec": rl.RetryAfterSec}
	}
	var pe *domain.PlanError
	if errors.As(err, &pe) {
		return map[string]any{"reason": pe.Reason, "field": pe.Field}
	}
	var d interface{ ErrorDetails() map[string]any }
	if errors.As(err, &d) {
		return d.ErrorDetails()
	}
	return nil
}
