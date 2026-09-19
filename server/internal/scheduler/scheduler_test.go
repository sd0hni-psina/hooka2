package scheduler

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

type syncBuffer struct {
	ch chan string
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	// Drop lines once the test stopped reading, so Run never blocks on logging.
	select {
	case b.ch <- string(bytes.Clone(p)):
	default:
	}
	return len(p), nil
}

type tickerFunc func(context.Context, time.Time) (Result, error)

func (f tickerFunc) Tick(ctx context.Context, now time.Time) (Result, error) { return f(ctx, now) }

func TestRunLogsEmptyTicks(t *testing.T) {
	buf := &syncBuffer{ch: make(chan string, 16)}
	log := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	ticker := tickerFunc(func(context.Context, time.Time) (Result, error) { return Result{}, nil })
	go func() { New(log, 10*time.Millisecond, ticker).Run(ctx); close(done) }()

	select {
	case line := <-buf.ch:
		require.True(t, strings.Contains(line, "scheduler tick"), line)
		require.Contains(t, line, "due=0")
	case <-time.After(2 * time.Second):
		t.Fatal("no tick logged")
	}
	cancel()
	<-done
}

func TestRunKeepsTickingAfterAFailure(t *testing.T) {
	buf := &syncBuffer{ch: make(chan string, 16)}
	log := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	var calls atomic.Int32
	ticker := tickerFunc(func(context.Context, time.Time) (Result, error) {
		if calls.Add(1) == 1 {
			return Result{}, errors.New("database down")
		}
		return Result{Due: 1}, nil
	})
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go New(log, 5*time.Millisecond, ticker).Run(ctx)

	require.Contains(t, <-buf.ch, "database down")
	require.Contains(t, <-buf.ch, "due=1")
}
