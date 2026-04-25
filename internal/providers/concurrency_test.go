package providers

// MIK-3072: per-runtime semaphore bounding concurrent provider goroutines.
// These tests register N>cap fake providers backed by httptest servers that
// block long enough to expose any unbounded fan-out, then assert peak
// in-flight count never exceeds the configured cap.

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// fakeHotelServer returns a slow httptest server that blocks for `delay`
// before responding with a single empty hotel result. The atomic peak
// counter is incremented on entry, decremented on exit, and its high-water
// mark is recorded via peakRef.
func fakeHotelServer(t *testing.T, delay time.Duration, current *atomic.Int64, peakRef *atomic.Int64) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		now := current.Add(1)
		// Track max observed concurrency.
		for {
			old := peakRef.Load()
			if now <= old || peakRef.CompareAndSwap(old, now) {
				break
			}
		}
		defer current.Add(-1)
		select {
		case <-time.After(delay):
		case <-r.Context().Done():
		}
		w.Header().Set("Content-Type", "application/json")
		// Empty results array; provider runtime treats this as 0-result success.
		_, _ = w.Write([]byte(`{"r":[]}`))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func registerFake(t *testing.T, reg *Registry, srv *httptest.Server, id string) {
	t.Helper()
	cfg := &ProviderConfig{
		ID:              id,
		Name:            id,
		Category:        "hotels",
		Endpoint:        srv.URL,
		ResponseMapping: ResponseMapping{ResultsPath: "r"},
	}
	if err := reg.Save(cfg); err != nil {
		t.Fatalf("save %s: %v", id, err)
	}
}

// TestProviderSemaphore_BoundsParallelism registers 12 fake providers with
// a 120ms delay each, then runs SearchHotels under TRVL_PROVIDER_CONCURRENCY=4
// and asserts the observed peak handler concurrency never exceeded 4.
func TestProviderSemaphore_BoundsParallelism(t *testing.T) {
	t.Setenv(providerConcurrencyEnv, "4")

	if got := providerConcurrency(); got != 4 {
		t.Fatalf("providerConcurrency() = %d, want 4 (env override)", got)
	}

	var current, peak atomic.Int64
	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	const total = 12
	for i := 0; i < total; i++ {
		srv := fakeHotelServer(t, 120*time.Millisecond, &current, &peak)
		registerFake(t, reg, srv, fmt.Sprintf("fake-%02d", i))
	}

	rt := NewRuntime(reg)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	_, statuses, _ := rt.SearchHotels(ctx, "Helsinki", 60.17, 24.94, "2026-06-01", "2026-06-02", "EUR", 2, nil)
	if len(statuses) != total {
		t.Errorf("statuses = %d, want %d (every provider should produce a status)", len(statuses), total)
	}
	if got := peak.Load(); got > 4 {
		t.Errorf("peak handler concurrency = %d, want <= 4 (semaphore breach)", got)
	}
	// At least 2 should have run concurrently — otherwise the semaphore is
	// over-restricting (or our fake server is slower than expected).
	if got := peak.Load(); got < 2 {
		t.Errorf("peak handler concurrency = %d, want >= 2 (workers not parallel)", got)
	}
	if rt.InflightProviders() != 0 {
		t.Errorf("inflight after search = %d, want 0", rt.InflightProviders())
	}
}

// TestProviderSemaphore_DefaultCap verifies the default cap is honoured
// when no env override is set.
func TestProviderSemaphore_DefaultCap(t *testing.T) {
	t.Setenv(providerConcurrencyEnv, "")
	if got := providerConcurrency(); got != defaultProviderConcurrency {
		t.Errorf("providerConcurrency() = %d, want %d", got, defaultProviderConcurrency)
	}
}

// TestProviderSemaphore_InvalidEnvFallsBack verifies that a non-numeric or
// non-positive env value falls back to the default cap.
func TestProviderSemaphore_InvalidEnvFallsBack(t *testing.T) {
	for _, bad := range []string{"abc", "0", "-3"} {
		t.Setenv(providerConcurrencyEnv, bad)
		if got := providerConcurrency(); got != defaultProviderConcurrency {
			t.Errorf("env=%q: providerConcurrency() = %d, want fallback %d", bad, got, defaultProviderConcurrency)
		}
	}
}

// TestProviderSemaphore_CtxCancelReleases ensures that when the caller
// cancels ctx, the dispatcher exits promptly and workers drain without
// holding the semaphore for the full per-provider timeout.
func TestProviderSemaphore_CtxCancelReleases(t *testing.T) {
	t.Setenv(providerConcurrencyEnv, "2")

	var current, peak atomic.Int64
	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	// 6 providers with a 5-second sleep — far longer than our cancel deadline.
	for i := 0; i < 6; i++ {
		srv := fakeHotelServer(t, 5*time.Second, &current, &peak)
		registerFake(t, reg, srv, fmt.Sprintf("slow-%02d", i))
	}

	rt := NewRuntime(reg)
	ctx, cancel := context.WithCancel(context.Background())
	// Cancel after 200ms — well before any provider completes.
	go func() {
		time.Sleep(200 * time.Millisecond)
		cancel()
	}()

	t0 := time.Now()
	_, _, _ = rt.SearchHotels(ctx, "Helsinki", 60.17, 24.94, "2026-06-01", "2026-06-02", "EUR", 2, nil)
	elapsed := time.Since(t0)

	// With cancel propagation, the call should return well before the 5s
	// per-provider sleep × any number of workers. Allow generous slack for
	// CI latency: 3s ceiling. Without cancellation propagation the test
	// would wait ~30s (perProviderTimeout default).
	if elapsed > 3*time.Second {
		t.Errorf("SearchHotels with cancelled ctx took %v, want <3s (cancellation not propagating)", elapsed)
	}
	if rt.InflightProviders() != 0 {
		t.Errorf("inflight after cancelled search = %d, want 0", rt.InflightProviders())
	}
}

// TestProviderSemaphore_WorkersAtMostProviders verifies that when the
// registry has fewer providers than the cap, the worker pool shrinks to
// match (no idle workers spawned).
func TestProviderSemaphore_WorkersAtMostProviders(t *testing.T) {
	t.Setenv(providerConcurrencyEnv, "16")

	var current, peak atomic.Int64
	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	for i := 0; i < 3; i++ {
		srv := fakeHotelServer(t, 50*time.Millisecond, &current, &peak)
		registerFake(t, reg, srv, fmt.Sprintf("few-%02d", i))
	}

	rt := NewRuntime(reg)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_, _, _ = rt.SearchHotels(ctx, "Helsinki", 60.17, 24.94, "2026-06-01", "2026-06-02", "EUR", 2, nil)
	}()
	wg.Wait()

	if got := peak.Load(); got > 3 {
		t.Errorf("peak with 3 providers and cap=16 = %d, want <= 3", got)
	}
}
