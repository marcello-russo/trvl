package mcp

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/testutil"
	"github.com/MikkoParkkola/trvl/internal/trip"
)

func TestHandleJourney_Success(t *testing.T) {
	t.Parallel()
	args := map[string]any{
		"airport_code":    "HEL",
		"date":            "2026-07-18",
		"departure_time":  "09:40",
		"international":   true,
		"ground_minutes":  float64(30),
		"ground_mode":     "train",
		"origin_walk_min": float64(5),
		"ground_label":    "Train I to Helsinki Airport",
	}
	content, structured, err := handleJourney(context.Background(), args, nil, nil, nil)
	if err != nil {
		t.Fatalf("handleJourney error: %v", err)
	}
	if len(content) == 0 {
		t.Fatal("expected user-facing content")
	}
	// Verify assistant block has JSON (more than just a summary)
	if len(content) < 2 || content[1].Text == "" {
		t.Fatal("expected assistant-facing content block with JSON data")
	}

	data, _ := json.Marshal(structured)
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal schedule: %v", err)
	}
	if got["leave_home_by"] != "06:45" {
		t.Errorf("leave_home_by = %v, want 06:45 (HEL intl 120 buffer + 30 ground + 5 var + 5 walk + 15 safety)", got["leave_home_by"])
	}
	if got["confidence"] != "high" {
		t.Errorf("confidence = %v, want high", got["confidence"])
	}
}

func TestHandleJourney_RequiresGroundChoice(t *testing.T) {
	t.Parallel()
	args := map[string]any{
		"airport_code":   "HEL",
		"date":           "2026-07-18",
		"departure_time": "09:40",
		// missing ground_minutes/ground_mode
	}
	if _, _, err := handleJourney(context.Background(), args, nil, nil, nil); err == nil {
		t.Fatal("expected error when ground option is missing")
	}
}

func TestHandleJourney_BadDate(t *testing.T) {
	t.Parallel()
	args := map[string]any{
		"airport_code":   "HEL",
		"date":           "July 18",
		"departure_time": "09:40",
		"ground_minutes": float64(30),
		"ground_mode":    "train",
	}
	if _, _, err := handleJourney(context.Background(), args, nil, nil, nil); err == nil {
		t.Fatal("expected error on unparseable date")
	}
}

// TestPlanJourney_CallableViaIntent_NotAdvertised verifies plan_journey is a
// reachable capability (registered handler) but not part of the advertised
// legacy compatibility-alias surface in either mode.
// TestHandleJourney_AsICS verifies the calendar handoff: as_ics attaches an
// iCalendar leave-home event with a reminder alarm to the response.
func TestHandleJourney_AsICS(t *testing.T) {
	t.Parallel()
	args := map[string]any{
		"airport_code":   "HEL",
		"date":           "2026-07-18",
		"departure_time": "09:40",
		"ground_minutes": float64(30),
		"ground_mode":    "train",
		"as_ics":         true,
	}
	_, structured, err := handleJourney(context.Background(), args, nil, nil, nil)
	if err != nil {
		t.Fatalf("handleJourney error: %v", err)
	}
	data, _ := json.Marshal(structured)
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	ics, ok := got["ics"].(string)
	if !ok || ics == "" {
		t.Fatalf("expected ics field in response, got %v", got["ics"])
	}
	if !strings.Contains(ics, "BEGIN:VALARM") || !strings.Contains(ics, "Leave home for HEL") {
		t.Errorf("ics missing leave-home event with alarm: %q", ics)
	}
}

func TestPlanJourney_CallableViaIntent_NotAdvertised(t *testing.T) {
	s := NewServer()
	if _, ok := s.handlers["plan_journey"]; !ok {
		t.Fatal("plan_journey handler must be registered (callable via travel intent)")
	}
	if toolRegistered(s.tools, "plan_journey") {
		t.Error("plan_journey must NOT be advertised in the default compact surface")
	}

	t.Setenv("TRVL_MCP_TOOL_MODE", "legacy")
	legacy := NewServer()
	if toolRegistered(legacy.tools, "plan_journey") {
		t.Error("plan_journey must NOT be advertised among the legacy compatibility aliases")
	}
}

// TestHandleJourney_AutoStitchOrigin verifies B.1-phase2: when origin is given
// and no explicit ground option, plan_journey searches the home->airport leg
// (stubbed seam), schedules from the best-value option, and returns the card.
func TestHandleJourney_AutoStitchOrigin(t *testing.T) {
	t.Parallel()
	orig := journeyTransferSearch
	t.Cleanup(func() { journeyTransferSearch = orig })
	journeyTransferSearch = func(ctx context.Context, in trip.AirportTransferInput) (*trip.AirportTransferResult, error) {
		return &trip.AirportTransferResult{
			Success: true,
			Count:   2,
			Routes: []models.GroundRoute{
				{Provider: "train", Type: "train", Price: 4.10, Currency: "EUR", Duration: 33, Transfers: 0},
				{Provider: "taxi", Type: "taxi", Price: 55, Currency: "EUR", Duration: 28, Transfers: 0},
			},
		}, nil
	}

	args := map[string]any{
		"airport_code":   "HEL",
		"date":           "2026-07-18",
		"departure_time": "09:40",
		"origin":         "Espoo",
		// no ground_minutes/ground_mode — must be auto-computed
	}
	_, structured, err := handleJourney(context.Background(), args, nil, nil, nil)
	if err != nil {
		t.Fatalf("handleJourney auto-stitch error: %v", err)
	}
	data, _ := json.Marshal(structured)
	var got map[string]any
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if got["leave_home_by"] == nil || got["leave_home_by"] == "" {
		t.Error("auto-stitched journey must produce a leave_home_by time")
	}
	if got["ground_comparison"] == nil {
		t.Error("auto-stitch must attach the home-to-airport comparison card")
	}
}

func TestHandleJourney_NoGroundNoOrigin_Errors(t *testing.T) {
	t.Parallel()
	args := map[string]any{
		"airport_code":   "HEL",
		"date":           "2026-07-18",
		"departure_time": "09:40",
		// no ground option AND no origin
	}
	if _, _, err := handleJourney(context.Background(), args, nil, nil, nil); err == nil {
		t.Fatal("expected error when neither ground option nor origin is provided")
	}
}

func TestHandleJourney_MissingParams(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name string
		args map[string]any
	}{
		{"empty", map[string]any{}},
		{"missing_airport_code", map[string]any{"date": "2026-07-01", "departure_time": "14:00"}},
		{"missing_date", map[string]any{"airport_code": "FCO", "departure_time": "14:00"}},
		{"missing_departure_time", map[string]any{"airport_code": "FCO", "date": "2026-07-01"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, _, err := handleJourney(context.Background(), tt.args, nil, nil, nil)
			if err == nil {
				t.Error("expected error for missing params")
			}
		})
	}
}

func TestHandleJourney_ContentHasJSON(t *testing.T) {
	t.Parallel()
	testutil.RequireLiveProbe(t)
	args := map[string]any{
		"airport_code":    "FCO",
		"date":            "2026-07-01",
		"departure_time":  "14:00",
		"ground_minutes":  60,
		"ground_mode":     "taxi",
		"ground_label":    "Home to Airport",
	}
	content, _, err := handleJourney(context.Background(), args, nil, nil, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(content) < 2 {
		t.Fatalf("expected 2+ content blocks, got %d", len(content))
	}
	assistantBlock := content[1]
	if assistantBlock.Text == "" {
		t.Fatal("assistant block has empty text")
	}
}
