package push

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func message(token, event string) Message {
	return Message{
		To: token, Title: "Стол 15", Body: "Стряхните угли", Sound: "default", Priority: "high", CategoryID: "service_action",
		Data: Data{EventID: event, HookahID: "h", TableID: "t", HallID: "hall", Kind: "shake", DueAt: time.Now()},
	}
}

func expoWith(t *testing.T, handler http.HandlerFunc) *Expo {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	e := NewExpo("secret-token")
	e.url = srv.URL
	return e
}

func TestExpoSend(t *testing.T) {
	t.Run("tickets map to results in order", func(t *testing.T) {
		var seenAuth string
		var sent []Message
		e := expoWith(t, func(w http.ResponseWriter, r *http.Request) {
			seenAuth = r.Header.Get("Authorization")
			body, err := io.ReadAll(r.Body)
			require.NoError(t, err)
			require.NoError(t, json.Unmarshal(body, &sent))
			_, err = w.Write([]byte(`{"data":[{"status":"ok","id":"receipt-1"},
				{"status":"error","message":"not registered","details":{"error":"DeviceNotRegistered"}},
				{"status":"error","message":"boom","details":{"error":"MessageTooBig"}}]}`))
			require.NoError(t, err)
		})

		res, err := e.Send(context.Background(), []Message{
			message("ExponentPushToken[a]", "e1"), message("ExponentPushToken[b]", "e2"), message("ExponentPushToken[c]", "e3"),
		})
		require.NoError(t, err)
		require.Equal(t, "Bearer secret-token", seenAuth)
		require.Len(t, sent, 3)
		require.Equal(t, "Стол 15", sent[0].Title)
		require.Equal(t, []Result{
			{Status: StatusSent, Receipt: "receipt-1"},
			{Status: StatusInvalidToken, Error: "not registered"},
			{Status: StatusFailed, Error: "boom"},
		}, res)
	})

	t.Run("http 500 is a transport failure of the batch", func(t *testing.T) {
		e := expoWith(t, func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusInternalServerError) })
		_, err := e.Send(context.Background(), []Message{message("ExponentPushToken[a]", "e1")})
		require.ErrorContains(t, err, "http 500")
	})

	t.Run("a timeout is a transport failure", func(t *testing.T) {
		// The handler outlives the client timeout without blocking the test.
		e := expoWith(t, func(w http.ResponseWriter, _ *http.Request) {
			time.Sleep(200 * time.Millisecond)
		})
		e.client.Timeout = 20 * time.Millisecond
		_, err := e.Send(context.Background(), []Message{message("ExponentPushToken[a]", "e1")})
		require.Error(t, err)
	})

	t.Run("a top level error fails the batch", func(t *testing.T) {
		e := expoWith(t, func(w http.ResponseWriter, _ *http.Request) {
			_, err := w.Write([]byte(`{"errors":[{"message":"rate limited"}]}`))
			require.NoError(t, err)
		})
		_, err := e.Send(context.Background(), []Message{message("ExponentPushToken[a]", "e1")})
		require.ErrorContains(t, err, "rate limited")
	})

	t.Run("a malformed payload never reaches expo", func(t *testing.T) {
		e := expoWith(t, func(http.ResponseWriter, *http.Request) { t.Fatal("expo must not be called") })
		_, err := e.Send(context.Background(), []Message{message("fcm-token", "e1")})
		require.ErrorContains(t, err, "bad token")
	})

	t.Run("more than a hundred messages go in several batches", func(t *testing.T) {
		batches := 0
		e := expoWith(t, func(w http.ResponseWriter, r *http.Request) {
			var in []Message
			require.NoError(t, json.NewDecoder(r.Body).Decode(&in))
			batches++
			tickets := make([]map[string]string, len(in))
			for i := range in {
				tickets[i] = map[string]string{"status": "ok", "id": "r"}
			}
			body, err := json.Marshal(map[string]any{"data": tickets})
			require.NoError(t, err)
			_, err = w.Write(body)
			require.NoError(t, err)
		})
		msgs := make([]Message, 150)
		for i := range msgs {
			msgs[i] = message("ExponentPushToken[a]", "e")
		}
		res, err := e.Send(context.Background(), msgs)
		require.NoError(t, err)
		require.Len(t, res, 150)
		require.Equal(t, 2, batches)
	})
}
