package mcp

import (
	"context"
	"testing"
)

func TestAssessTripTool_Definition(t *testing.T) {
	t.Parallel()
	tool := assessTripTool()
	if tool.Name != "assess_trip" {
		t.Errorf("Name = %q, want assess_trip", tool.Name)
	}
	if len(tool.InputSchema.Required) == 0 {
		t.Error("expected required params")
	}
}

func TestHandleAssessTrip_MissingParams(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args map[string]any
	}{
		{"empty", map[string]any{}},
		{"missing_origin", map[string]any{"destination": "BCN", "depart_date": "2026-08-01", "return_date": "2026-08-10"}},
		{"missing_destination", map[string]any{"origin": "HEL", "depart_date": "2026-08-01", "return_date": "2026-08-10"}},
		{"missing_depart_date", map[string]any{"origin": "HEL", "destination": "BCN", "return_date": "2026-08-10"}},
		{"missing_return_date", map[string]any{"origin": "HEL", "destination": "BCN", "depart_date": "2026-08-01"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := handleAssessTrip(context.Background(), tt.args, nil, nil, nil)
			if err == nil {
				t.Error("expected error for missing params")
			}
		})
	}
}
