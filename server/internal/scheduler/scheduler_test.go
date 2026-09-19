package scheduler

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
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

func TestRunLogsEmptyTicks(t *testing.T) {
	buf := &syncBuffer{ch: make(chan string, 16)}
	log := slog.New(slog.NewTextHandler(buf, &slog.HandlerOptions{Level: slog.LevelDebug}))
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() { New(log, 10*time.Millisecond).Run(ctx); close(done) }()

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
