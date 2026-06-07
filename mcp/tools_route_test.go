package mcp

import (
	"context"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/testutil"
)

func TestHandleSearchRoute_MissingParams(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args map[string]any
	}{
		{"empty", map[string]any{}},
		{"missing_origin", map[string]any{"destination": "Paris", "date": "2026-07-01"}},
		{"missing_destination", map[string]any{"origin": "London", "date": "2026-07-01"}},
		{"missing_date", map[string]any{"origin": "London", "destination": "Paris"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := handleSearchRoute(context.Background(), tt.args, nil, nil, nil)
			if err == nil {
				t.Error("expected error for missing params")
			}
		})
	}
}

func TestHandleSearchRoute_ContentHasJSON(t *testing.T) {
	t.Parallel()
	testutil.RequireLiveProbe(t)
	args := map[string]any{
		"origin":      "London",
		"destination": "Paris",
		"date":        "2026-07-01",
	}
	content, _, err := handleSearchRoute(context.Background(), args, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) < 2 {
		t.Fatalf("expected 2+ content blocks, got %d", len(content))
	}
	assistantBlock := content[1]
	if assistantBlock.Text == "Structured data attached." {
		t.Fatal("assistant block still has placeholder text instead of JSON")
	}
}
