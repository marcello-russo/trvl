package main

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestVersionCmd_DefaultOutput(t *testing.T) {
	// versionCmd and versionJSON are package-level Cobra state.
	var buf bytes.Buffer
	versionCmd.SetOut(&buf)
	versionCmd.SetErr(&buf)
	prevJSON := versionJSON
	versionJSON = false
	defer func() { versionJSON = prevJSON }()

	versionCmd.Run(versionCmd, nil)
	got := buf.String()
	if !strings.HasPrefix(got, "trvl ") {
		t.Errorf("default output should start with 'trvl ', got: %q", got)
	}
}

func TestVersionCmd_JSONOutput(t *testing.T) {
	// versionCmd and versionJSON are package-level Cobra state.
	var buf bytes.Buffer
	versionCmd.SetOut(&buf)
	versionCmd.SetErr(&buf)
	prevJSON := versionJSON
	versionJSON = true
	defer func() { versionJSON = prevJSON }()

	versionCmd.Run(versionCmd, nil)
	var report VersionReport
	if err := json.Unmarshal(buf.Bytes(), &report); err != nil {
		t.Fatalf("expected valid JSON, parse failed: %v\noutput: %s", err, buf.String())
	}
	// Version is whatever the test binary was built with — usually "dev".
	if report.Version == "" {
		t.Errorf("version should not be empty")
	}
	if report.InstallMethod == "" {
		t.Errorf("install_method should not be empty")
	}
	// Trust anchor must be present and non-empty (build-time go:embed
	// would have failed if the keyfile were missing).
	if report.MLDSATrustAnchor == "" {
		t.Errorf("mldsa_trust_anchor should not be empty")
	}
	if len(report.MLDSATrustAnchor) != 16 {
		t.Errorf("mldsa_trust_anchor should be 16 hex chars (first half of fingerprint), got %d: %q",
			len(report.MLDSATrustAnchor), report.MLDSATrustAnchor)
	}
}

func TestBuildVersionReport_Dev(t *testing.T) {
	t.Parallel()
	rep := buildVersionReport("dev")
	if rep.Version != "dev" {
		t.Errorf("version: got %q want dev", rep.Version)
	}
	if rep.InstallMethod != "dev" {
		t.Errorf("install_method: got %q want dev", rep.InstallMethod)
	}
	if rep.SupportsInPlaceReplace {
		t.Errorf("dev should not support in-place replace")
	}
	if rep.UpgradeHint != "" {
		t.Errorf("dev upgrade_hint should be empty, got %q", rep.UpgradeHint)
	}
}

func TestBuildVersionReport_TrustAnchorAlwaysPopulated(t *testing.T) {
	t.Parallel()
	// The trust anchor is embedded via go:embed so it's available even
	// for "dev" builds — we ship the same key in every binary.
	rep := buildVersionReport("dev")
	if rep.MLDSATrustAnchor == "" {
		t.Errorf("mldsa_trust_anchor must be populated for any build")
	}
}
