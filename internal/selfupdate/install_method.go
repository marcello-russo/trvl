package selfupdate

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// InstallMethod identifies how the running trvl binary was installed.
// The auto-update path branches on this value because each channel has
// a different "correct" upgrade gesture:
//
//   - Brew: the user runs `brew upgrade trvl`. Self-replacing the binary
//     under the cellar would conflict with brew's manifest tracking on
//     the next run. We surface a SUGGESTION instead.
//   - Go: installed via `go install` — the binary lives under $GOBIN /
//     $GOPATH/bin and the canonical re-build is `go install ...@latest`.
//     We never overwrite a Go-build artifact.
//   - Npm: the npm wrapper at npm/bin/trvl-mcp.js downloads a tarball
//     into npm/bin/<bin>. The npm registry handles version resolution
//     and `npm i -g trvl-mcp@latest` is the user-facing upgrade.
//   - Standalone: a tarball extracted to ~/bin / /usr/local/bin / etc.
//     This is the ONLY channel where in-place atomic replacement is the
//     correct behavior.
//   - Dev: built locally from source with no Version stamp or "dev"
//     stamp. Self-update is meaningless; the dev rebuilds with their own
//     toolchain.
//   - Unclassified: detector could not determine the channel. We treat
//     this exactly like Standalone-but-cautious: never auto-replace,
//     always defer to a user-initiated `trvl self-update`.
type InstallMethod int

const (
	InstallMethodUnclassified InstallMethod = iota
	InstallMethodDev
	InstallMethodBrew
	InstallMethodGo
	InstallMethodNpm
	InstallMethodStandalone
)

// String returns a stable lowercase identifier suitable for telemetry,
// JSON serialization, and provider_health output.
func (m InstallMethod) String() string {
	switch m {
	case InstallMethodDev:
		return "dev"
	case InstallMethodBrew:
		return "brew"
	case InstallMethodGo:
		return "go"
	case InstallMethodNpm:
		return "npm"
	case InstallMethodStandalone:
		return "standalone"
	default:
		return "unclassified"
	}
}

// SupportsInPlaceReplace reports whether trvl may overwrite its own
// binary as part of an auto-update flow. Only Standalone qualifies —
// every other channel has an external manifest (brew Cellar metadata,
// go module cache, npm package.json) that would be invalidated by
// silently swapping the binary out from under it.
func (m InstallMethod) SupportsInPlaceReplace() bool {
	return m == InstallMethodStandalone
}

// UpgradeHint returns a one-line, copy-pasteable command the user
// should run to upgrade their installation. Empty string for Dev /
// Unclassified (no actionable advice).
func (m InstallMethod) UpgradeHint() string {
	switch m {
	case InstallMethodBrew:
		return "brew upgrade trvl"
	case InstallMethodGo:
		return "go install github.com/MikkoParkkola/trvl/cmd/trvl@latest"
	case InstallMethodNpm:
		return "npm install -g trvl-mcp@latest"
	case InstallMethodStandalone:
		return "trvl self-update"
	default:
		return ""
	}
}

// DetectInstallMethod inspects the running binary's path and surrounding
// filesystem markers to classify how trvl was installed. It is purely
// read-only — never spawns subprocesses, never reaches the network.
//
// currentVer is typically main.Version. Empty string or "dev" forces
// InstallMethodDev regardless of path.
//
// Detection order (first match wins):
//  1. Dev: version is empty or "dev".
//  2. Brew: binary path contains "/Cellar/trvl/" OR the resolved real
//     path lives under a Homebrew prefix.
//  3. Go: binary path is exactly $GOBIN/trvl, $GOPATH/bin/trvl, or
//     ~/go/bin/trvl. The `go install` default output.
//  4. Npm: binary lives under a node_modules/trvl-mcp/bin/ subtree.
//  5. Standalone: any other path that is NOT under a system temp
//     directory.
//  6. Unclassified: path lookup failed entirely. Caller should treat
//     exactly like a cautious Standalone and refuse auto-replacement.
//
// The detection is deliberately conservative: when in doubt, we return
// the LESS-aggressive class so the auto-update path errs toward
// "suggest a manual command" rather than "silently overwrite the
// binary".
func DetectInstallMethod(currentVer string) InstallMethod {
	if v := strings.TrimSpace(currentVer); v == "" || v == "dev" {
		return InstallMethodDev
	}

	exePath, err := os.Executable()
	if err != nil {
		return InstallMethodUnclassified
	}
	// Resolve symlinks so a brew shim under /usr/local/bin/trvl ->
	// /opt/homebrew/Cellar/trvl/1.1.4/bin/trvl classifies as brew, not
	// standalone. Failure here is not fatal — fall back to the raw path.
	resolved, err := filepath.EvalSymlinks(exePath)
	if err != nil {
		resolved = exePath
	}

	if isBrewPath(resolved) || isBrewPath(exePath) {
		return InstallMethodBrew
	}
	if isGoBinPath(resolved) || isGoBinPath(exePath) {
		return InstallMethodGo
	}
	if isNpmPath(resolved) || isNpmPath(exePath) {
		return InstallMethodNpm
	}
	if isPlausibleStandalonePath(resolved) {
		return InstallMethodStandalone
	}
	return InstallMethodUnclassified
}

// isBrewPath returns true if p sits inside a Homebrew Cellar layout.
//
// Linuxbrew uses /home/linuxbrew/.linuxbrew/Cellar/trvl/<v>/bin/trvl.
// macOS Apple-silicon brew uses /opt/homebrew/Cellar/...
// macOS Intel brew uses /usr/local/Cellar/...
// All three contain the literal "/Cellar/trvl/" substring.
func isBrewPath(p string) bool {
	if filepath.VolumeName(p) != "" {
		return false
	}
	return strings.Contains(filepath.ToSlash(p), "/Cellar/trvl/")
}

// isGoBinPath returns true if p matches a `go install` output location.
// We check the standard $GOBIN / $GOPATH/bin / ~/go/bin landing zones.
// Note: this can produce a false-negative on exotic GOPATH layouts; the
// fallback to InstallMethodStandalone is acceptable (worse advice, not
// wrong behavior).
func isGoBinPath(p string) bool {
	dir := filepath.Dir(p)

	if gobin := os.Getenv("GOBIN"); gobin != "" {
		if filepath.Clean(gobin) == filepath.Clean(dir) {
			return true
		}
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		if filepath.Clean(filepath.Join(gopath, "bin")) == filepath.Clean(dir) {
			return true
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if filepath.Clean(filepath.Join(home, "go", "bin")) == filepath.Clean(dir) {
			return true
		}
	}
	return false
}

// isNpmPath returns true if p sits under a node_modules/trvl-mcp/bin
// subtree. Both global (~/.npm/.../node_modules/trvl-mcp/bin) and local
// (./project/node_modules/trvl-mcp/bin) layouts match.
func isNpmPath(p string) bool {
	slash := filepath.ToSlash(p)
	return strings.Contains(slash, "/node_modules/trvl-mcp/")
}

// isPlausibleStandalonePath returns true for paths that look like a
// hand-extracted tarball: ~/bin, /usr/local/bin, /opt/, /usr/bin, etc.
// We exclude system temp dirs so an in-flight `go test` build under
// $TMPDIR doesn't get auto-classified as a real install.
func isPlausibleStandalonePath(p string) bool {
	if p == "" {
		return false
	}
	dir := filepath.Dir(p)
	tmpDir := os.TempDir()
	if tmpDir != "" && strings.HasPrefix(dir, filepath.Clean(tmpDir)+string(os.PathSeparator)) {
		return false
	}
	_ = runtime.GOOS // referenced for future per-OS heuristics
	return dir != "" && dir != "."
}
