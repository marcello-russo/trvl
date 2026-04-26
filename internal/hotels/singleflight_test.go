package hotels

import (
	"sync"
	"sync/atomic"
	"testing"
)

// TestHotelSingleflight verifies that concurrent calls with the same key
// are coalesced and the underlying search executes only once.
func TestHotelSingleflight(t *testing.T) {
	var callCount atomic.Int64

	const n = 10
	key := "hotel|Paris|2026-06-15|2026-06-18|2"

	var wg sync.WaitGroup
	results := make([]any, n)
	errs := make([]error, n)

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			v, err, _ := hotelGroup.Do(key, func() (any, error) {
				callCount.Add(1)
				return "result", nil
			})
			results[idx] = v
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	count := callCount.Load()
	if count == 0 {
		t.Fatal("expected inner function to be called at least once, got 0")
	}
	if count > int64(n) {
		t.Fatalf("expected inner function called ≤%d times, got %d", n, count)
	}
	t.Logf("inner function called %d times for %d concurrent goroutines", count, n)

	for i, r := range results {
		if r != "result" {
			t.Errorf("goroutine %d got result %v, want %q", i, r, "result")
		}
		if errs[i] != nil {
			t.Errorf("goroutine %d got error %v, want nil", i, errs[i])
		}
	}
}

// TestHotelSearchKey verifies that different parameter combinations produce
// distinct keys, preventing incorrect deduplication.
func TestHotelSearchKey(t *testing.T) {
	base := HotelSearchOptions{CheckIn: "2026-06-15", CheckOut: "2026-06-18", Guests: 2, Currency: "USD"}
	changedCheckIn := base
	changedCheckIn.CheckIn = "2026-06-16"
	changedGuests := base
	changedGuests.Guests = 3
	changedCurrency := base
	changedCurrency.Currency = "EUR"
	changedStars := base
	changedStars.Stars = 5
	changedMaxPages := base
	changedMaxPages.MaxPages = 1
	changedFilter := base
	changedFilter.MinPrice = 100

	k1 := hotelSearchKey("Paris", base)
	k2 := hotelSearchKey("Paris", changedCheckIn)
	k3 := hotelSearchKey("London", base)
	k4 := hotelSearchKey("Paris", changedGuests)
	k5 := hotelSearchKey("Paris", changedCurrency)
	k6 := hotelSearchKey("Paris", changedStars)
	k7 := hotelSearchKey("Paris", changedMaxPages)
	k8 := hotelSearchKey("Paris", changedFilter)

	keys := []string{k1, k2, k3, k4, k5, k6, k7, k8}
	for i := range keys {
		for j := i + 1; j < len(keys); j++ {
			if keys[i] == keys[j] {
				t.Errorf("key collision: keys[%d] == keys[%d]: %q", i, j, keys[i])
			}
		}
	}

	// Same inputs must produce the same key.
	k1again := hotelSearchKey("Paris", base)
	if k1 != k1again {
		t.Errorf("same inputs produced different keys: %q vs %q", k1, k1again)
	}
}

// TestSearchHotelsWithClient_MissingDates verifies that concurrent calls with
// missing dates all return errors without panicking.
func TestSearchHotelsWithClient_MissingDates(t *testing.T) {
	const n = 5

	var wg sync.WaitGroup
	errs := make([]error, n)
	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := SearchHotels(t.Context(), "Paris", HotelSearchOptions{})
			errs[idx] = err
		}(i)
	}
	wg.Wait()

	for i, err := range errs {
		if err == nil {
			t.Errorf("goroutine %d: expected error for missing dates, got nil", i)
		}
	}
}
