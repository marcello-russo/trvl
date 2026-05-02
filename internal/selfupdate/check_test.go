package selfupdate

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"
)

// fakeReleasesServer returns an httptest.Server that responds to the
// "latest release" call with a configurable tag and counts how many
// times it has been hit (used to assert cache freshness behavior).
func fakeReleasesServer(t *testing.T, tag string, hitCount *int32) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(hitCount, 1)
		if got := r.Header.Get("Accept"); got != "application/vnd.github+json" {
			t.Errorf("Accept header = %q, want application/vnd.github+json", got)
		}
		if got := r.Header.Get("User-Agent"); got != userAgent {
			t.Errorf("User-Agent = %q, want %q", got, userAgent)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"tag_name":     "v" + tag,
			"html_url":     "https://github.com/MikkoParkkola/trvl/releases/tag/v" + tag,
			"published_at": time.Now().Format(time.RFC3339),
		})
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestCheck_DetectsUpdateAvailable: running v1.1.0, GH says latest is
// v1.1.2 → UpdateAvailable=true with the right URL.
func TestCheck_DetectsUpdateAvailable(t *testing.T) {
	var hits int32
	srv := fakeReleasesServer(t, "1.1.2", &hits)
	c, err := NewChecker("1.1.0",
		WithReleasesURL(srv.URL),
		WithCacheDir(t.TempDir()),
	)
	if err != nil {
		t.Fatalf("NewChecker: %v", err)
	}
	info, err := c.Check(context.Background(), false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.LatestVersion != "1.1.2" {
		t.Errorf("LatestVersion = %q, want 1.1.2", info.LatestVersion)
	}
	if !info.UpdateAvailable {
		t.Errorf("UpdateAvailable = false, want true (1.1.0 < 1.1.2)")
	}
	if info.ReleaseURL == "" {
		t.Errorf("ReleaseURL empty")
	}
	if info.CurrentVersion != "1.1.0" {
		t.Errorf("CurrentVersion = %q, want 1.1.0", info.CurrentVersion)
	}
	if atomic.LoadInt32(&hits) != 1 {
		t.Errorf("api hits = %d, want 1", hits)
	}
}

// TestCheck_NoUpdateWhenAhead: running v2.0.0 (somehow ahead of upstream
// v1.1.2) → UpdateAvailable=false. Tests the semver compare path.
func TestCheck_NoUpdateWhenAhead(t *testing.T) {
	var hits int32
	srv := fakeReleasesServer(t, "1.1.2", &hits)
	c, _ := NewChecker("2.0.0",
		WithReleasesURL(srv.URL),
		WithCacheDir(t.TempDir()),
	)
	info, _ := c.Check(context.Background(), false)
	if info.UpdateAvailable {
		t.Errorf("UpdateAvailable = true, want false (2.0.0 > 1.1.2)")
	}
}

// TestCheck_NoUpdateWhenEqual: running v1.1.2, upstream is v1.1.2 →
// UpdateAvailable=false. Avoids the "infinite update loop" anti-pattern.
func TestCheck_NoUpdateWhenEqual(t *testing.T) {
	var hits int32
	srv := fakeReleasesServer(t, "1.1.2", &hits)
	c, _ := NewChecker("1.1.2",
		WithReleasesURL(srv.URL),
		WithCacheDir(t.TempDir()),
	)
	info, _ := c.Check(context.Background(), false)
	if info.UpdateAvailable {
		t.Errorf("UpdateAvailable = true, want false (1.1.2 == 1.1.2)")
	}
}

// TestCheck_CacheFreshSkipsAPI: two consecutive calls within
// checkInterval should result in exactly ONE API hit. Failure here
// means we'd hammer GitHub on every trvl invocation.
func TestCheck_CacheFreshSkipsAPI(t *testing.T) {
	var hits int32
	srv := fakeReleasesServer(t, "1.1.2", &hits)
	dir := t.TempDir()
	c, _ := NewChecker("1.1.0",
		WithReleasesURL(srv.URL),
		WithCacheDir(dir),
		WithCheckInterval(1*time.Hour),
	)

	if _, err := c.Check(context.Background(), false); err != nil {
		t.Fatalf("first Check: %v", err)
	}
	if _, err := c.Check(context.Background(), false); err != nil {
		t.Fatalf("second Check: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("api hits = %d, want 1 (second call should hit cache)", got)
	}
	// Sanity: cache file exists and contains the version we'd expect.
	data, err := os.ReadFile(filepath.Join(dir, cacheFilename))
	if err != nil {
		t.Fatalf("cache file: %v", err)
	}
	var info UpdateInfo
	if err := json.Unmarshal(data, &info); err != nil {
		t.Fatalf("cache parse: %v", err)
	}
	if info.LatestVersion != "1.1.2" {
		t.Errorf("cached LatestVersion = %q, want 1.1.2", info.LatestVersion)
	}
}

// TestCheck_StaleCacheRefetches: after the cache TTL expires, the next
// call should hit the API again.
func TestCheck_StaleCacheRefetches(t *testing.T) {
	var hits int32
	srv := fakeReleasesServer(t, "1.1.2", &hits)
	c, _ := NewChecker("1.1.0",
		WithReleasesURL(srv.URL),
		WithCacheDir(t.TempDir()),
		WithCheckInterval(1*time.Millisecond), // immediately stale
	)
	_, _ = c.Check(context.Background(), false)
	time.Sleep(5 * time.Millisecond)
	_, _ = c.Check(context.Background(), false)
	if got := atomic.LoadInt32(&hits); got != 2 {
		t.Errorf("api hits = %d, want 2 (second call should re-fetch after TTL)", got)
	}
}

// TestCheck_FailSilentOnNetworkError: when the API is down or unreachable,
// Check returns the empty UpdateInfo with err == nil. trvl must NEVER
// crash because the update server is offline.
func TestCheck_FailSilentOnNetworkError(t *testing.T) {
	// Server that hangs up immediately.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hj, _ := w.(http.Hijacker)
		conn, _, _ := hj.Hijack()
		_ = conn.Close()
	}))
	defer srv.Close()

	c, _ := NewChecker("1.1.0",
		WithReleasesURL(srv.URL),
		WithCacheDir(t.TempDir()),
	)
	info, err := c.Check(context.Background(), false)
	if err != nil {
		t.Fatalf("Check returned error on network failure: %v (must fail silent)", err)
	}
	if info.UpdateAvailable {
		t.Errorf("UpdateAvailable = true with no successful API call")
	}
}

// TestCheck_FailSilentOnApiError: 500 from the API → empty UpdateInfo,
// no error. Same fail-silent contract as network failure.
func TestCheck_FailSilentOnApiError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c, _ := NewChecker("1.1.0",
		WithReleasesURL(srv.URL),
		WithCacheDir(t.TempDir()),
	)
	info, err := c.Check(context.Background(), false)
	if err != nil {
		t.Fatalf("Check returned error on 500: %v", err)
	}
	if info.UpdateAvailable {
		t.Errorf("UpdateAvailable = true on API 500")
	}
}

// TestCheck_FallsBackToStaleCacheOnNetworkError: if we have ANY cached
// info (even expired) and the network fails, surface the cached info
// rather than nothing. Recomputes UpdateAvailable against the running
// binary version so the answer is correct even if the user upgraded
// between checks.
func TestCheck_FallsBackToStaleCacheOnNetworkError(t *testing.T) {
	dir := t.TempDir()
	cache := UpdateInfo{
		LatestVersion:   "1.1.2",
		UpdateAvailable: true,
		ReleaseURL:      "https://github.com/MikkoParkkola/trvl/releases/tag/v1.1.2",
		CheckedAt:       time.Now().Add(-30 * 24 * time.Hour), // 30 days old
	}
	data, _ := json.MarshalIndent(cache, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, cacheFilename), data, 0o600); err != nil {
		t.Fatalf("seed cache: %v", err)
	}

	deadSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer deadSrv.Close()

	c, _ := NewChecker("1.1.0",
		WithReleasesURL(deadSrv.URL),
		WithCacheDir(dir),
		WithCheckInterval(1*time.Millisecond), // force re-fetch attempt
	)
	info, err := c.Check(context.Background(), false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.LatestVersion != "1.1.2" {
		t.Errorf("LatestVersion = %q, want 1.1.2 (from stale cache)", info.LatestVersion)
	}
	if !info.UpdateAvailable {
		t.Errorf("UpdateAvailable = false; want true (stale cache says 1.1.2 > running 1.1.0)")
	}
}

// TestCheck_DisabledForCI: when disableForCI is true, no API call,
// no cache read, no work. Auto-update must never fire in CI.
func TestCheck_DisabledForCI(t *testing.T) {
	var hits int32
	srv := fakeReleasesServer(t, "1.1.2", &hits)
	c, _ := NewChecker("1.1.0",
		WithReleasesURL(srv.URL),
		WithCacheDir(t.TempDir()),
	)
	info, err := c.Check(context.Background(), true)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.UpdateAvailable {
		t.Errorf("UpdateAvailable = true with CI disable flag")
	}
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Errorf("api hits = %d, want 0 (CI must skip API call)", got)
	}
}

// TestCheck_DisabledForDevBuild: a binary with Version == "dev" or
// empty has no meaningful version to compare; skip the check.
func TestCheck_DisabledForDevBuild(t *testing.T) {
	var hits int32
	srv := fakeReleasesServer(t, "1.1.2", &hits)
	for _, ver := range []string{"", "dev"} {
		c, _ := NewChecker(ver,
			WithReleasesURL(srv.URL),
			WithCacheDir(t.TempDir()),
		)
		_, _ = c.Check(context.Background(), false)
	}
	if got := atomic.LoadInt32(&hits); got != 0 {
		t.Errorf("api hits = %d for dev/empty version, want 0", got)
	}
}

// TestCheck_RecomputesAvailabilityOnCacheHit: a user upgrades trvl out
// of band (e.g. brew) but the cache hasn't expired. The next call must
// recompute UpdateAvailable against the NEW running version, not the
// version that was current when the cache was written. Otherwise we'd
// keep reporting "v1.1.2 available" forever after the user installed it.
func TestCheck_RecomputesAvailabilityOnCacheHit(t *testing.T) {
	dir := t.TempDir()
	// Cache says the user is on 1.1.0 and 1.1.2 is available.
	cache := UpdateInfo{
		CurrentVersion:  "1.1.0",
		LatestVersion:   "1.1.2",
		UpdateAvailable: true,
		ReleaseURL:      "https://github.com/MikkoParkkola/trvl/releases/tag/v1.1.2",
		CheckedAt:       time.Now(),
	}
	data, _ := json.MarshalIndent(cache, "", "  ")
	_ = os.WriteFile(filepath.Join(dir, cacheFilename), data, 0o600)

	// Now we run trvl 1.1.2. The cache is fresh; no API call. But the
	// returned UpdateAvailable must be FALSE because we've caught up.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("API hit on fresh cache")
	}))
	defer srv.Close()

	c, _ := NewChecker("1.1.2",
		WithReleasesURL(srv.URL),
		WithCacheDir(dir),
		WithCheckInterval(24*time.Hour),
	)
	info, err := c.Check(context.Background(), false)
	if err != nil {
		t.Fatalf("Check: %v", err)
	}
	if info.UpdateAvailable {
		t.Errorf("UpdateAvailable = true after running binary caught up to cached LatestVersion")
	}
}

// TestCheck_RespectsContextCancellation: callers (the cmd/trvl startup
// goroutine) pass a ctx with a deadline. If the network is slow, the
// check must abort cleanly when the deadline expires.
func TestCheck_RespectsContextCancellation(t *testing.T) {
	hangSrv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	}))
	defer hangSrv.Close()

	c, _ := NewChecker("1.1.0",
		WithReleasesURL(hangSrv.URL),
		WithCacheDir(t.TempDir()),
	)
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	start := time.Now()
	_, err := c.Check(ctx, false)
	elapsed := time.Since(start)

	if err != nil {
		t.Errorf("Check returned error on ctx cancel: %v (must fail silent)", err)
	}
	if elapsed > 500*time.Millisecond {
		t.Errorf("Check took %v with 50ms ctx timeout — not respecting cancellation", elapsed)
	}
}

// silence the unused-package warning if the import gets refactored
// during future edits — errors is needed when the file grows.
var _ = errors.New
