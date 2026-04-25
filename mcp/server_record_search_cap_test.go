package mcp

// MIK-3073: cap unbounded tripState.Searches slice growth.

import "testing"

func TestRecordSearch_CapAtMax(t *testing.T) {
	s := NewServer()
	// Push well past the cap; assert length never exceeds, FIFO eviction works.
	const total = maxTripStateSearches + 50
	for i := 0; i < total; i++ {
		s.recordSearch("flight", "TEST", float64(i), "EUR")
	}
	s.tripState.mu.Lock()
	defer s.tripState.mu.Unlock()
	if got := len(s.tripState.Searches); got != maxTripStateSearches {
		t.Fatalf("len after %d records = %d; want cap %d", total, got, maxTripStateSearches)
	}
	// The oldest 50 should have been evicted; the first remaining record should
	// be the (50)th by BestPrice.
	first := s.tripState.Searches[0]
	if first.BestPrice != 50.0 {
		t.Errorf("first record BestPrice = %v; want 50 (oldest 50 evicted)", first.BestPrice)
	}
	last := s.tripState.Searches[len(s.tripState.Searches)-1]
	if last.BestPrice != float64(total-1) {
		t.Errorf("last record BestPrice = %v; want %v", last.BestPrice, float64(total-1))
	}
}
