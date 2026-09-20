package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"
)

const (
	expoEndpoint = "https://exp.host/--/api/v2/push/send"
	expoBatch    = 100
	expoTimeout  = 10 * time.Second
)

// Expo is the real sender, tech.md section 19.9.
type Expo struct {
	token  string
	client *http.Client
	url    string
}

func NewExpo(accessToken string) *Expo {
	return &Expo{token: accessToken, client: &http.Client{Timeout: expoTimeout}, url: expoEndpoint}
}

type expoTicket struct {
	Status  string `json:"status"`
	ID      string `json:"id"`
	Message string `json:"message"`
	Details struct {
		Error string `json:"error"`
	} `json:"details"`
}

type expoResponse struct {
	Data   []expoTicket `json:"data"`
	Errors []struct {
		Message string `json:"message"`
	} `json:"errors"`
}

// Send delivers the batch. A transport failure returns an error for the whole
// batch; a per-message problem lands in its Result.
func (e *Expo) Send(ctx context.Context, msgs []Message) ([]Result, error) {
	out := make([]Result, 0, len(msgs))
	for start := 0; start < len(msgs); start += expoBatch {
		end := min(start+expoBatch, len(msgs))
		part, err := e.send(ctx, msgs[start:end])
		if err != nil {
			return nil, err
		}
		out = append(out, part...)
	}
	return out, nil
}

func (e *Expo) send(ctx context.Context, msgs []Message) ([]Result, error) {
	for i, m := range msgs {
		if err := Validate(m); err != nil {
			return nil, fmt.Errorf("expo push: message %d: %w", i, err)
		}
	}
	body, err := json.Marshal(msgs)
	if err != nil {
		return nil, fmt.Errorf("expo push: %w", err)
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, e.url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("expo push: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	if e.token != "" {
		req.Header.Set("Authorization", "Bearer "+e.token)
	}
	res, err := e.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("expo push: %w", err)
	}
	defer func() {
		if err := res.Body.Close(); err != nil {
			slog.DebugContext(ctx, "close expo response", "err", err)
		}
	}()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("expo push: http %d", res.StatusCode)
	}
	var parsed expoResponse
	if err := json.NewDecoder(res.Body).Decode(&parsed); err != nil {
		return nil, fmt.Errorf("expo push: decode: %w", err)
	}
	if len(parsed.Errors) > 0 {
		return nil, fmt.Errorf("expo push: %s", parsed.Errors[0].Message)
	}
	if len(parsed.Data) != len(msgs) {
		return nil, fmt.Errorf("expo push: got %d tickets for %d messages", len(parsed.Data), len(msgs))
	}

	out := make([]Result, len(msgs))
	for i, ticket := range parsed.Data {
		switch {
		case ticket.Status == "ok":
			out[i] = Result{Status: StatusSent, Receipt: ticket.ID}
		case ticket.Details.Error == "DeviceNotRegistered":
			out[i] = Result{Status: StatusInvalidToken, Error: ticket.Message}
		default:
			out[i] = Result{Status: StatusFailed, Error: ticket.Message}
		}
	}
	return out, nil
}
