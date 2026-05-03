package selfupdate

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"
)

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
// goroutine. Returns immediately. Designed for the cmd/trvl entrypoint
// where any blocking work would be perceived as trvl being slow.
//
// The goroutine respects a ctx that the caller can cancel on shutdown.
// On cold-start cache hit (the common path) the check is non-blocking
// anyway — a single os.Stat + JSON parse takes microseconds. On cold-
// start cache miss, the HTTP call to GitHub takes 100-500ms; we run it
// in the background so the user's `trvl flights HEL AMS` returns its
// real result immediately, and the notification fires on the next
// invocation when the cache is warm.
//
// currentVer is typically main.Version. notifyW receives the one-line
// notice (typically os.Stderr); pass nil to skip notification (e.g.
// MCP server mode where stderr would interleave with structured I/O).
func CheckInBackground(ctx context.Context, currentVer string, notifyW io.Writer) {
	if strings.TrimSpace(currentVer) == "" || currentVer == "dev" {
		return
	}
	go func() {
		c, err := NewChecker(currentVer)
		if err != nil {
			return
		}
		// Cap the background work — even fail-silent paths can hang on
		// pathological networks. 6 s upper bound is plenty for the
		// 5 s HTTP timeout plus parse + cache write overhead.
		bgCtx, cancel := context.WithTimeout(ctx, 6*time.Second)
		defer cancel()
		info, _ := c.Check(bgCtx, IsCIEnv())
		if notifyW != nil {
			NotifyAvailable(notifyW, info)
		}
	}()
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
