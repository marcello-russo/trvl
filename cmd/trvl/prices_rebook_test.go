package main

import (
	"testing"

	"github.com/MikkoParkkola/trvl/internal/hotelarb"
)

func TestPricesHoldCmdPersistsActiveHold(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)

	cmd := pricesHoldCmd()
	cmd.SetArgs([]string{
		"/g/testhotel",
		"--name", "Test Hotel",
		"--checkin", "2026-07-01",
		"--checkout", "2026-07-03",
		"--price", "420",
		"--currency", "EUR",
		"--refundable",
	})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("prices hold: %v", err)
	}

	store, err := hotelarb.DefaultHoldStore()
	if err != nil {
		t.Fatalf("DefaultHoldStore: %v", err)
	}
	holds, err := store.Load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(holds) != 1 {
		t.Fatalf("holds = %d, want 1", len(holds))
	}
	if holds[0].HotelName != "Test Hotel" {
		t.Fatalf("HotelName = %q, want Test Hotel", holds[0].HotelName)
	}
}

func TestPricesRebookCmdWithManualCurrentPriceUpdatesHold(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)
	origFormat := format
	format = "table"
	t.Cleanup(func() { format = origFormat })

	store, err := hotelarb.DefaultHoldStore()
	if err != nil {
		t.Fatalf("DefaultHoldStore: %v", err)
	}
	if _, err := store.Load(); err != nil {
		t.Fatalf("Load: %v", err)
	}
	id, err := store.Add(hotelarb.Hold{
		HotelID:       "/g/testhotel",
		HotelName:     "Test Hotel",
		CheckIn:       "2026-07-01",
		CheckOut:      "2026-07-03",
		OriginalPrice: 420,
		Currency:      "EUR",
		Refundable:    true,
	})
	if err != nil {
		t.Fatalf("Add: %v", err)
	}

	cmd := pricesRebookCmd()
	cmd.SetArgs([]string{id, "--current-price", "300", "--currency", "EUR", "--provider", "manual", "--min-savings", "10"})
	if err := cmd.Execute(); err != nil {
		t.Fatalf("prices rebook: %v", err)
	}

	if _, err := store.Load(); err != nil {
		t.Fatalf("Reload: %v", err)
	}
	updated, ok := store.Get(id)
	if !ok {
		t.Fatal("hold missing after rebook check")
	}
	if updated.LastSeenPrice != 300 {
		t.Fatalf("LastSeenPrice = %.0f, want 300", updated.LastSeenPrice)
	}
	if updated.LastSeenAt.IsZero() {
		t.Fatal("LastSeenAt should be set")
	}
}
