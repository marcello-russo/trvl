package mcp

import (
	"context"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/testutil"
)

func TestHandleSearchGround_MissingParams(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args map[string]any
	}{
		{"empty", map[string]any{}},
		{"missing_from", map[string]any{"to": "Paris", "date": "2026-07-01"}},
		{"missing_to", map[string]any{"from": "London", "date": "2026-07-01"}},
		{"missing_date", map[string]any{"from": "London", "to": "Paris"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := handleSearchGround(context.Background(), tt.args, nil, nil, nil)
			if err == nil {
				t.Error("expected error for missing params")
			}
		})
	}
}

func TestHandleSearchGround_ContentHasJSON(t *testing.T) {
	t.Parallel()
	testutil.RequireLiveProbe(t)
	args := map[string]any{
		"from": "London",
		"to":   "Paris",
		"date": "2026-07-01",
	}
	content, structured, err := handleSearchGround(context.Background(), args, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) < 2 {
		t.Fatalf("expected 2+ content blocks, got %d", len(content))
	}
	// Block at [1] is assistant-facing: should contain JSON, not placeholder
	assistantBlock := content[1]
	if assistantBlock.Annotations == nil || len(assistantBlock.Annotations.Audience) == 0 {
		t.Fatal("assistant block missing audience annotation")
	}
	if assistantBlock.Text == "" {
		t.Fatal("assistant block has empty text")
	}
	if assistantBlock.Text == "Structured data attached." {
		t.Fatal("assistant block still has placeholder text instead of JSON")
	}
	// Verify structured result is returned
	if structured == nil {
		t.Fatal("expected structured result")
	}
}
