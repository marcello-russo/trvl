package selfupdate

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/MikkoParkkola/trvl/internal/upgrade"
)

// semverCmp delegates to the upgrade package — same comparison used
// throughout trvl for version ordering.
func semverCmp(a, b string) int {
	return upgrade.CompareSemver(a, b)
}

// IsCIEnv heuristically detects continuous-integration / sandboxed
// environments where auto-update notifications are noise. The set is
// the standard "common-CI" union used by tools like git-lfs, gh, and
// pnpm: every CI provider and ephemeral runner sets at least one of
// these.
//
// We deliberately err on the side of "skip when unsure" — false
// positives just suppress a notification (no harm), false negatives
// spam a CI log every job (annoying).
func IsCIEnv() bool {
	for _, name := range []string{
		"CI",
		"CONTINUOUS_INTEGRATION",
		"GITHUB_ACTIONS",
		"GITLAB_CI",
		"CIRCLECI",
		"TRAVIS",
		"BUILDKITE",
		"DRONE",
		"JENKINS_URL",
		"TEAMCITY_VERSION",
		"BITBUCKET_BUILD_NUMBER",
		"APPVEYOR",
		"CODEBUILD_BUILD_ID",
	} {
		if v := os.Getenv(name); v != "" && v != "0" && v != "false" {
			return true
		}
	}
	// Honor the project-specific kill switch independently.
	if v := os.Getenv("TRVL_DISABLE_UPDATE_CHECK"); v != "" && v != "0" && v != "false" {
		return true
	}
	return false
}

// NotifyAvailable writes a single-line stderr notice when the cached
// UpdateInfo says a newer version is available. Best-effort — any I/O
// failure is silently ignored so we never break trvl's actual output.
//
// The notice format is intentionally short and machine-parseable:
//
//	trvl: v1.1.3 available (you have v1.1.2). Release notes: <url>
//
// Callers wire this into the CLI startup path (after rootCmd.Execute
// returns) and the MCP server startup path. Both invocations read the
// same on-disk cache, so cost is one os.Stat + one JSON parse per
// process invocation.
func NotifyAvailable(w io.Writer, info UpdateInfo) {
	if !info.UpdateAvailable || info.LatestVersion == "" {
		return
	}
	current := info.CurrentVersion
	if current == "" {
		current = "dev"
	}
	msg := fmt.Sprintf("trvl: v%s available (you have v%s). Release notes: %s\n",
		info.LatestVersion, current, info.ReleaseURL)
	_, _ = io.WriteString(w, msg)
}

// CheckInBackground fires off a daily update check in a detached
// goroutine AND synchronously prints any notice from the warm cache
// before returning. This split design exists because main() typically
// exits within milliseconds of rootCmd.Execute() returning, and a pure
// background goroutine would be killed before it could write to stderr.
//
// Behavior:
//   - SYNC fast path: read on-disk cache (microseconds). If cache says
//     UpdateAvailable, recompute against currentVer and call
//     NotifyAvailable. This always completes before main() exits.
//   - ASYNC slow path: if the cache is stale OR absent, spawn a
//     goroutine that fetches the GH releases API and writes the result
//     back to disk. The goroutine has up to 6s to finish; main() does
//     NOT wait for it. The user sees the notice on the NEXT invocation
//     once the cache is warm.
//
// Net effect: notification latency is "next invocation after the first
// one that hit a cold/stale cache", typically the very next CLI run.
// Cost on the hot path is one os.Stat + JSON parse — well under 1ms.
//
// currentVer is typically main.Version. notifyW receives the one-line
// notice (typically os.Stderr); pass nil to skip notification (e.g.
// MCP server mode where stderr would interleave with structured I/O).
func CheckInBackground(ctx context.Context, currentVer string, notifyW io.Writer) {
	if strings.TrimSpace(currentVer) == "" || currentVer == "dev" {
		return
	}
	if IsCIEnv() {
		return
	}
	c, err := NewChecker(currentVer)
	if err != nil {
		return
	}

	// SYNC: warm-cache read. Microseconds — safe to do before main exits.
	// We bypass Checker.Check's "fresh" gate here so even a stale cache
	// surfaces a notice while we re-fetch in the background.
	if cached, ok := c.readCache(); ok && cached.LatestVersion != "" {
		cached.CurrentVersion = currentVer
		// Recompute against the running binary version in case the
		// user upgraded out of band since the cache was written.
		cached.UpdateAvailable = compareVersions(cached.LatestVersion, currentVer) > 0
		if notifyW != nil {
			NotifyAvailable(notifyW, cached)
		}
	}

	// ASYNC: refresh the cache for next time. Detached goroutine so
	// main() can exit immediately. Bounded to 6s. We do NOT print a
	// notice from this goroutine — even if it succeeds, main is likely
	// already gone, and surfacing now would race the user's actual
	// program output. The result lives in the cache for next time.
	go func() {
		bgCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
		defer cancel()
		_, _ = c.Check(bgCtx, false)
	}()
}

// compareVersions wraps the upgrade package's CompareSemver to avoid
// re-importing it at every callsite.
func compareVersions(a, b string) int {
	return semverCmp(a, b)
}

// LoadCachedInfo returns the most recently cached UpdateInfo, or the
// zero value if the cache file is absent / unreadable / malformed.
// Used by surfaces that should reflect "what we currently know" without
// triggering a network call (the MCP provider_health tool, the
// `trvl version` command). Callers should not rely on UpdateAvailable
// being correct against a freshly-changed running version — that's
// what Checker.Check is for.
func LoadCachedInfo() UpdateInfo {
	c, err := NewChecker("placeholder")
	if err != nil {
		return UpdateInfo{}
	}
	info, ok := c.readCache()
	if !ok {
		return UpdateInfo{}
	}
	return info
}
