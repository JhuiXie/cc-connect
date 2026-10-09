package core

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// gatedUsageAgent blocks GetUsage until release is closed, mimicking the
// Claude Code /usage probe which spawns a CLI and takes several seconds.
type gatedUsageAgent struct {
	stubAgent
	release chan struct{}
	calls   atomic.Int32
	mu      sync.Mutex
	used    int
	err     error
}

func newGatedUsageAgent(used int) *gatedUsageAgent {
	return &gatedUsageAgent{release: make(chan struct{}), used: used}
}

func (a *gatedUsageAgent) GetUsage(ctx context.Context) (*UsageReport, error) {
	a.calls.Add(1)
	select {
	case <-a.release:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.err != nil {
		return nil, a.err
	}
	return &UsageReport{Buckets: []UsageBucket{{Windows: []UsageWindow{{
		UsedPercent:   a.used,
		WindowSeconds: 18000,
	}}}}}, nil
}

func (a *gatedUsageAgent) set(used int, err error) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.used, a.err = used, err
}

func waitUsageRefreshIdle(t *testing.T, e *Engine) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		e.replyFooterMu.Lock()
		idle := e.replyFooterUsage.refreshing == nil || isClosed(e.replyFooterUsage.refreshing)
		fetched := !e.replyFooterUsage.fetchedAt.IsZero()
		e.replyFooterMu.Unlock()
		if idle && fetched {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("background usage refresh did not finish")
}

func waitUsageCalls(t *testing.T, a *gatedUsageAgent, want int32) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for a.calls.Load() < want && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
}

func isClosed(ch chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func expireUsageCache(e *Engine) {
	e.replyFooterMu.Lock()
	e.replyFooterUsage.fetchedAt = time.Now().Add(-2 * replyFooterUsageCacheTTL)
	e.replyFooterMu.Unlock()
}

func TestReplyFooterUsage_SlowFirstFetchIsBoundedThenCached(t *testing.T) {
	agent := newGatedUsageAgent(40)
	e := NewEngine("test", agent, nil, "", LangEnglish)

	start := time.Now()
	if got := e.replyFooterUsageText(nil, agent); got != "" {
		t.Fatalf("first lookup = %q, want empty while probe is slow", got)
	}
	if elapsed := time.Since(start); elapsed > replyFooterUsageTimeout+time.Second {
		t.Fatalf("first lookup took %v, want bounded by footer timeout", elapsed)
	}

	// The probe finishes well after the footer gave up; its result must
	// land in the cache instead of being discarded.
	time.Sleep(300 * time.Millisecond)
	close(agent.release)
	waitUsageRefreshIdle(t, e)

	start = time.Now()
	got := e.replyFooterUsageText(nil, agent)
	if got != "5h 60% left" {
		t.Fatalf("second lookup = %q, want background result", got)
	}
	if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
		t.Fatalf("cached lookup took %v, want immediate", elapsed)
	}
	if n := agent.calls.Load(); n != 1 {
		t.Fatalf("GetUsage calls = %d, want 1 (cache hit)", n)
	}
}

func TestReplyFooterUsage_StaleCacheReturnsImmediatelyAndRefreshesOnce(t *testing.T) {
	agent := newGatedUsageAgent(10)
	e := NewEngine("test", agent, nil, "", LangEnglish)
	close(agent.release)
	if got := e.replyFooterUsageText(nil, agent); got != "5h 90% left" {
		t.Fatalf("warm-up lookup = %q", got)
	}
	waitUsageRefreshIdle(t, e)

	// Block the next refresh and make the cache stale.
	agent.release = make(chan struct{})
	agent.set(30, nil)
	expireUsageCache(e)

	for i := 0; i < 5; i++ {
		start := time.Now()
		if got := e.replyFooterUsageText(nil, agent); got != "5h 90% left" {
			t.Fatalf("stale lookup %d = %q, want previous text", i, got)
		}
		if elapsed := time.Since(start); elapsed > 100*time.Millisecond {
			t.Fatalf("stale lookup %d took %v, must not wait on the probe", i, elapsed)
		}
	}
	waitUsageCalls(t, agent, 2)
	time.Sleep(50 * time.Millisecond)
	if n := agent.calls.Load(); n != 2 {
		t.Fatalf("GetUsage calls = %d, want 2 (one warm-up, one deduped refresh)", n)
	}

	close(agent.release)
	waitUsageRefreshIdle(t, e)
	if got := e.replyFooterUsageText(nil, agent); got != "5h 70% left" {
		t.Fatalf("after refresh = %q, want refreshed text", got)
	}
}

func TestReplyFooterUsage_FailedRefreshKeepsLastText(t *testing.T) {
	agent := newGatedUsageAgent(10)
	e := NewEngine("test", agent, nil, "", LangEnglish)
	close(agent.release)
	if got := e.replyFooterUsageText(nil, agent); got != "5h 90% left" {
		t.Fatalf("warm-up lookup = %q", got)
	}
	waitUsageRefreshIdle(t, e)

	agent.set(0, errors.New("probe failed"))
	expireUsageCache(e)
	e.replyFooterUsageText(nil, agent)
	waitUsageRefreshIdle(t, e)

	if got := e.replyFooterUsageText(nil, agent); got != "5h 90% left" {
		t.Fatalf("after failed refresh = %q, want last good text", got)
	}
	time.Sleep(50 * time.Millisecond)
	if n := agent.calls.Load(); n != 2 {
		t.Fatalf("GetUsage calls = %d, want 2 (failure is cached for the TTL)", n)
	}
}
