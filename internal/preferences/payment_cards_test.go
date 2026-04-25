package preferences

// MIK-3083: ensure the PaymentCards field round-trips through JSON
// marshaling so the card-ranking package can consume it without losing
// fields.

import (
	"encoding/json"
	"testing"
)

func TestPreferences_PaymentCardsRoundTrip(t *testing.T) {
	in := &Preferences{
		PaymentCards: []PaymentCard{
			{
				Name:           "FB Amex Gold",
				MCCMultipliers: map[string]float64{"airline": 4, "default": 1},
				PointValueEUR:  0.012,
				IntroOffer:     "60k pts after 2k EUR spend",
				FXFeePct:       0,
			},
			{
				Name:           "Visa Curve",
				MCCMultipliers: map[string]float64{"default": 1},
				PointValueEUR:  0.005,
				FXFeePct:       0.5,
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
	if len(out.PaymentCards) != len(in.PaymentCards) {
		t.Fatalf("got %d cards, want %d", len(out.PaymentCards), len(in.PaymentCards))
	}
	if out.PaymentCards[0].Name != "FB Amex Gold" {
		t.Errorf("name = %q, want FB Amex Gold", out.PaymentCards[0].Name)
	}
	if out.PaymentCards[0].MCCMultipliers["airline"] != 4 {
		t.Errorf("airline mult = %v, want 4", out.PaymentCards[0].MCCMultipliers["airline"])
	}
	if out.PaymentCards[1].FXFeePct != 0.5 {
		t.Errorf("fx fee = %v, want 0.5", out.PaymentCards[1].FXFeePct)
	}
}

// TestPreferences_PaymentCardsOmittedWhenEmpty confirms the field is
// `omitempty` so existing on-disk preferences files do not gain a new
// JSON key until the user actually fills it in.
func TestPreferences_PaymentCardsOmittedWhenEmpty(t *testing.T) {
	in := &Preferences{HomeAirports: []string{"AMS"}}
	data, err := json.Marshal(in)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if _, present := raw["payment_cards"]; present {
		t.Errorf("empty PaymentCards should be omitted from JSON, got key present")
	}
}
