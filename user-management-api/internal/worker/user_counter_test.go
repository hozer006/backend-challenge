package worker

import (
	"bytes"
	"context"
	"errors"
	"log/slog"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type fakeCounter struct {
	calls atomic.Int32
	n     int64
	err   error
}

func (f *fakeCounter) Count(context.Context) (int64, error) {
	f.calls.Add(1)
	return f.n, f.err
}

// syncBuffer is a goroutine-safe bytes.Buffer for capturing logs.
type syncBuffer struct {
	mu sync.Mutex
	b  bytes.Buffer
}

func (s *syncBuffer) Write(p []byte) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.Write(p)
}

func (s *syncBuffer) String() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.b.String()
}

func runFor(t *testing.T, c *fakeCounter, d time.Duration) string {
	t.Helper()
	var buf syncBuffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	ctx, cancel := context.WithTimeout(context.Background(), d)
	defer cancel()

	done := make(chan struct{})
	go func() { RunUserCounter(ctx, c, 10*time.Millisecond, logger); close(done) }()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("RunUserCounter did not stop after context cancellation")
	}
	return buf.String()
}

func TestRunUserCounter_LogsCount(t *testing.T) {
	c := &fakeCounter{n: 42}
	out := runFor(t, c, 150*time.Millisecond)

	if c.calls.Load() < 2 {
		t.Fatalf("Count called %d times, want >= 2", c.calls.Load())
	}
	if !strings.Contains(out, "total users") || !strings.Contains(out, "count=42") {
		t.Fatalf("log output missing count:\n%s", out)
	}
	if !strings.Contains(out, "user counter stopped") {
		t.Fatalf("expected stop log:\n%s", out)
	}
}

func TestRunUserCounter_ContinuesOnError(t *testing.T) {
	c := &fakeCounter{err: errors.New("db down")}
	out := runFor(t, c, 150*time.Millisecond)

	if c.calls.Load() < 2 {
		t.Fatalf("worker should keep ticking after errors, calls=%d", c.calls.Load())
	}
	if !strings.Contains(out, "count users failed") {
		t.Fatalf("expected error log:\n%s", out)
	}
}
