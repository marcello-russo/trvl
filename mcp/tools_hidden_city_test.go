package mcp

import (
	"context"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/testutil"
)

func TestHandleSearchHiddenCity_NoOffers(t *testing.T) {
	t.Parallel()
	// `offers` is optional (not in schema "required"). Missing offers = zero candidates, no error.
	content, structured, err := handleSearchHiddenCity(context.Background(), map[string]any{"allow_hidden_city": true, "depart_date": "2026-07-01"}, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) < 2 {
		t.Fatalf("expected 2+ content blocks, got %d", len(content))
	}
	assistantBlock := content[1]
	if assistantBlock.Text == "Structured hidden-city data attached." {
		t.Fatal("assistant block still has placeholder text instead of JSON")
	}
	if structured == nil {
		t.Fatal("expected structured result")
	}
}

func TestHandleSearchHiddenCity_ContentHasJSON(t *testing.T) {
	t.Parallel()
	testutil.RequireLiveProbe(t)
	args := map[string]any{
		"offers": []any{
			map[string]any{
				"origin":      "FCO",
				"destination": "JFK",
				"price":       500.0,
				"currency":    "EUR",
			},
		},
		"allow_hidden_city": true,
		"depart_date":       "2026-07-01",
	}
	content, _, err := handleSearchHiddenCity(context.Background(), args, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) < 2 {
		t.Fatalf("expected 2+ content blocks, got %d", len(content))
	}
	assistantBlock := content[1]
	if assistantBlock.Text == "Structured hidden-city data attached." {
		t.Fatal("assistant block still has placeholder text instead of JSON")
	}
}
