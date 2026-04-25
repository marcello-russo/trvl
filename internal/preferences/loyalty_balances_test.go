package preferences

// MIK-3082: ensure the LoyaltyBalances field round-trips through JSON
// marshaling and is omitted when empty.

import (
	"encoding/json"
	"testing"
)

func TestPreferences_LoyaltyBalancesRoundTrip(t *testing.T) {
	in := &Preferences{
		LoyaltyBalances: []LoyaltyBalance{
			{
				Program:               "Flying Blue",
				Balance:               14975,
				ExpiresAt:             "2027-01-31",
				StatusTier:            "Gold",
				StatusRenewalDeadline: "2026-12-31",
				QualSegmentsNeeded:    1,
			},
		},
	}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var out Preferences
	if err := json.Unmarshal(data, &out); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(out.LoyaltyBalances) != 1 {
		t.Fatalf("got %d balances, want 1", len(out.LoyaltyBalances))
	}
	got := out.LoyaltyBalances[0]
	if got.Program != "Flying Blue" || got.Balance != 14975 {
		t.Errorf("balance round-trip lost fields: %+v", got)
	}
	if got.QualSegmentsNeeded != 1 {
		t.Errorf("QualSegmentsNeeded = %d, want 1", got.QualSegmentsNeeded)
	}
}

func TestPreferences_LoyaltyBalancesOmittedWhenEmpty(t *testing.T) {
	in := &Preferences{HomeAirports: []string{"AMS"}}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, present := raw["loyalty_balances"]; present {
		t.Errorf("empty LoyaltyBalances should be omitted from JSON")
	}
}
