package providers

// MIK-3075: tests for the provider-config schema-version migration chain.

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// testEndpoint is a syntactically-valid endpoint URL accepted by
// ProviderConfig.Validate. Using a localhost fixture keeps the test
// suite self-contained and free of references to any reserved domain.
const testEndpoint = "https://provider.test.localdomain/api"

func TestMigrate_NilConfigErrors(t *testing.T) {
	if err := Migrate(nil); err == nil {
		t.Error("Migrate(nil) returned nil error")
	}
}

func TestMigrate_AlreadyCurrentIsNoOp(t *testing.T) {
	cfg := &ProviderConfig{
		ID: "x", SchemaVersion: CurrentSchemaVersion,
		Method: "POST", Version: 5,
	}
	if err := Migrate(cfg); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("schema unchanged check: got %q", cfg.SchemaVersion)
	}
	if cfg.Method != "POST" || cfg.Version != 5 {
		t.Errorf("Migrate(noop) mutated fields: %+v", cfg)
	}
}

func TestMigrate_LegacyToV1_0PromotesEmptyMethodAndVersion(t *testing.T) {
	cfg := &ProviderConfig{ID: "legacy"}
	if err := Migrate(cfg); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q", cfg.SchemaVersion, CurrentSchemaVersion)
	}
	if cfg.Method != "GET" {
		t.Errorf("Method = %q, want GET (migrated default)", cfg.Method)
	}
	if cfg.Version < 1 {
		t.Errorf("Version = %d, want >= 1 (migrated default)", cfg.Version)
	}
}

func TestMigrate_LegacyKeepsExistingFields(t *testing.T) {
	cfg := &ProviderConfig{ID: "preexisting", Method: "POST", Version: 7}
	if err := Migrate(cfg); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	if cfg.Method != "POST" {
		t.Errorf("Method got mutated: %q", cfg.Method)
	}
	if cfg.Version != 7 {
		t.Errorf("Version got mutated: %d", cfg.Version)
	}
}

func TestMigrate_FutureVersionRejected(t *testing.T) {
	cfg := &ProviderConfig{ID: "tomorrow", SchemaVersion: "v99.0"}
	err := Migrate(cfg)
	if err == nil {
		t.Fatal("expected error from future version, got nil")
	}
	if !strings.Contains(err.Error(), "newer than this binary") {
		t.Errorf("error = %q, want mention 'newer than this binary'", err)
	}
}

func TestMigrate_UnregisteredVersionStringRejected(t *testing.T) {
	// "v0.5" is not a known From in any migration → should fail loudly.
	cfg := &ProviderConfig{ID: "weird", SchemaVersion: "v0.5"}
	err := Migrate(cfg)
	if err == nil {
		t.Fatal("expected error from unmigratable version, got nil")
	}
	if !strings.Contains(err.Error(), "no migration from") {
		t.Errorf("error = %q, want mention 'no migration from'", err)
	}
}

func TestParseVersion_BadFormatsRejected(t *testing.T) {
	bad := []string{"", "1.0", "v1", "vfoo.bar", "v1.x"}
	for _, in := range bad {
		if _, _, err := parseVersion(in); err == nil {
			t.Errorf("parseVersion(%q) = nil err, want error", in)
		}
	}
}

func TestIsNewerThanCurrent(t *testing.T) {
	cases := map[string]bool{
		"v0.9":  false,
		"v1.0":  false,
		"v1.1":  true,
		"v2.0":  true,
		"v99.0": true,
	}
	for in, want := range cases {
		got, err := isNewerThanCurrent(in)
		if err != nil {
			t.Fatalf("isNewerThanCurrent(%q): unexpected err: %v", in, err)
		}
		if got != want {
			t.Errorf("isNewerThanCurrent(%q) = %v, want %v", in, got, want)
		}
	}
}

// TestRegistry_NewRegistryAt_MigratesLegacyConfig confirms the load path
// runs Migrate on legacy on-disk configs. Writes a config without
// schema_version, calls NewRegistryAt, and asserts the in-memory copy
// is at CurrentSchemaVersion.
func TestRegistry_NewRegistryAt_MigratesLegacyConfig(t *testing.T) {
	dir := t.TempDir()
	legacy := map[string]any{
		"id":       "legacy-prov",
		"name":     "Legacy",
		"category": "hotels",
		"endpoint": testEndpoint,
		"response_mapping": map[string]any{
			"results_path": "data.hotels",
		},
	}
	data, _ := json.MarshalIndent(legacy, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "legacy-prov.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatalf("NewRegistryAt: %v", err)
	}
	got := reg.Get("legacy-prov")
	if got == nil {
		t.Fatal("config not loaded")
	}
	if got.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("SchemaVersion = %q, want %q (migrated)", got.SchemaVersion, CurrentSchemaVersion)
	}
	if got.Method != "GET" {
		t.Errorf("Method = %q, want GET (migrated default)", got.Method)
	}
}

// TestRegistry_NewRegistryAt_RejectsFutureSchema confirms the load path
// fails loudly when a config declares a version newer than this binary.
func TestRegistry_NewRegistryAt_RejectsFutureSchema(t *testing.T) {
	dir := t.TempDir()
	future := map[string]any{
		"schema_version": "v99.0",
		"id":             "future-prov",
		"name":           "Future",
		"category":       "hotels",
		"endpoint":       testEndpoint,
		"response_mapping": map[string]any{
			"results_path": "data.hotels",
		},
	}
	data, _ := json.MarshalIndent(future, "", "  ")
	if err := os.WriteFile(filepath.Join(dir, "future-prov.json"), data, 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := NewRegistryAt(dir); err == nil {
		t.Fatal("expected NewRegistryAt to reject future schema, got nil")
	}
}

// TestRegistry_Save_StampsCurrentSchemaVersion confirms saveLocked writes
// the current schema_version even when the in-memory config did not have
// one set explicitly.
func TestRegistry_Save_StampsCurrentSchemaVersion(t *testing.T) {
	dir := t.TempDir()
	reg, err := NewRegistryAt(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg := &ProviderConfig{
		ID:              "fresh-prov",
		Name:            "Fresh",
		Category:        "hotels",
		Endpoint:        testEndpoint,
		Method:          "GET",
		ResponseMapping: ResponseMapping{ResultsPath: "data.hotels"},
	}
	if err := reg.Save(cfg); err != nil {
		t.Fatalf("Save: %v", err)
	}
	if cfg.SchemaVersion != CurrentSchemaVersion {
		t.Errorf("post-Save in-memory SchemaVersion = %q, want %q", cfg.SchemaVersion, CurrentSchemaVersion)
	}

	// Re-read the file off disk and confirm the field landed there too.
	raw, err := os.ReadFile(filepath.Join(dir, "fresh-prov.json"))
	if err != nil {
		t.Fatal(err)
	}
	var probe map[string]any
	_ = json.Unmarshal(raw, &probe)
	if probe["schema_version"] != CurrentSchemaVersion {
		t.Errorf("on-disk schema_version = %v, want %q", probe["schema_version"], CurrentSchemaVersion)
	}
}
