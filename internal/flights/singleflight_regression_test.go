package flights

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/batchexec"
	"github.com/MikkoParkkola/trvl/internal/models"
)

func TestSearchFlightsWithClient_ConcurrentCallersGetPrivateResults(t *testing.T) {
	body := makeFlightResponseBody(t)

	var requestCount atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	client := batchexec.NewTestClient(ts.URL)

	const n = 2
	var wg sync.WaitGroup
	start := make(chan struct{})
	results := make([]*models.FlightSearchResult, n)
	errs := make([]error, n)

	for i := range n {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			results[idx], errs[idx] = SearchFlightsWithClient(t.Context(), client, "HEL", "NRT", "2026-06-15", SearchOptions{})
		}(i)
	}

	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d search error: %v", i, err)
		}
	}
	if got := requestCount.Load(); got != 1 {
		t.Fatalf("requestCount = %d, want 1 shared upstream request", got)
	}
	if results[0] == results[1] {
		t.Fatal("concurrent callers received the same FlightSearchResult pointer")
	}
	if len(results[0].Flights) == 0 || len(results[1].Flights) == 0 {
		t.Fatal("expected both callers to receive flights")
	}
	if &results[0].Flights[0] == &results[1].Flights[0] {
		t.Fatal("concurrent callers reused the same FlightResult backing storage")
	}

	otherCount := results[1].Count
	otherPrice := results[1].Flights[0].Price
	otherFlightsLen := len(results[1].Flights)

	results[0].Count = 0
	results[0].Flights[0].Price = otherPrice + 99
	results[0].Flights = results[0].Flights[:0]

	if results[1].Count != otherCount {
		t.Fatalf("caller 1 Count changed to %d, want %d", results[1].Count, otherCount)
	}
	if len(results[1].Flights) != otherFlightsLen {
		t.Fatalf("caller 1 len(Flights) changed to %d, want %d", len(results[1].Flights), otherFlightsLen)
	}
	if results[1].Flights[0].Price != otherPrice {
		t.Fatalf("caller 1 flight price changed to %v, want %v", results[1].Flights[0].Price, otherPrice)
	}
}

func TestSearchFlightsWithClient_ConcurrentEquivalentFilterSetsShareUpstreamRequest(t *testing.T) {
	body := makeFlightResponseBody(t)

	var requestCount atomic.Int64
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestCount.Add(1)
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(body)
	}))
	defer ts.Close()

	client := batchexec.NewTestClient(ts.URL)
	optsA := SearchOptions{
		Airlines:    []string{"AY", "JL"},
		Alliances:   []string{"ONEWORLD", "STAR_ALLIANCE"},
		MaxPrice:    1500,
		MaxDuration: 900,
	}
	optsB := SearchOptions{
		Airlines:    []string{"JL", "AY"},
		Alliances:   []string{"STAR_ALLIANCE", "ONEWORLD"},
		MaxPrice:    1500,
		MaxDuration: 900,
	}

	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, 2)

	wg.Add(2)
	go func() {
		defer wg.Done()
		<-start
		_, errs[0] = SearchFlightsWithClient(t.Context(), client, "HEL", "NRT", "2026-06-15", optsA)
	}()
	go func() {
		defer wg.Done()
		<-start
		_, errs[1] = SearchFlightsWithClient(t.Context(), client, "HEL", "NRT", "2026-06-15", optsB)
	}()

	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d search error: %v", i, err)
		}
	}
	if got := requestCount.Load(); got != 1 {
		t.Fatalf("requestCount = %d, want 1 shared upstream request for equivalent filters", got)
	}
	if gotA, gotB := flightSearchKey("HEL", "NRT", "2026-06-15", optsA), flightSearchKey("HEL", "NRT", "2026-06-15", optsB); gotA != gotB {
		t.Fatalf("flightSearchKey mismatch for equivalent filters: %q != %q", gotA, gotB)
	}
}

func TestSharedFlightResult_ClonesPartialErrorResult(t *testing.T) {
	carryOn := true
	checkedBags := 1
	shared := &models.FlightSearchResult{
		Success:  false,
		Count:    1,
		TripType: "one_way",
		Flights: []models.FlightResult{
			{
				Price:               123,
				Warnings:            []string{"Self-connect risk"},
				Legs:                []models.FlightLeg{{AirlineCode: "AY"}},
				CarryOnIncluded:     &carryOn,
				CheckedBagsIncluded: &checkedBags,
			},
		},
		Error: "provider partial failure",
	}

	wantErr := errors.New("provider partial failure")
	got, err := sharedFlightResult(shared, wantErr)
	if !errors.Is(err, wantErr) {
		t.Fatalf("err = %v, want %v", err, wantErr)
	}
	if got == shared {
		t.Fatal("sharedFlightResult returned the original pointer")
	}
	if &got.Flights[0] == &shared.Flights[0] {
		t.Fatal("sharedFlightResult reused the shared Flights backing array")
	}
	if got.Flights[0].CarryOnIncluded == shared.Flights[0].CarryOnIncluded {
		t.Fatal("sharedFlightResult reused the shared CarryOnIncluded pointer")
	}
	if got.Flights[0].CheckedBagsIncluded == shared.Flights[0].CheckedBagsIncluded {
		t.Fatal("sharedFlightResult reused the shared CheckedBagsIncluded pointer")
	}

	got.Flights[0].Warnings[0] = "changed"
	got.Flights[0].Legs[0].AirlineCode = "JL"
	*got.Flights[0].CarryOnIncluded = false
	*got.Flights[0].CheckedBagsIncluded = 2

	if shared.Flights[0].Warnings[0] != "Self-connect risk" {
		t.Fatalf("shared warning = %q, want %q", shared.Flights[0].Warnings[0], "Self-connect risk")
	}
	if shared.Flights[0].Legs[0].AirlineCode != "AY" {
		t.Fatalf("shared airline = %q, want %q", shared.Flights[0].Legs[0].AirlineCode, "AY")
	}
	if got := *shared.Flights[0].CarryOnIncluded; !got {
		t.Fatalf("shared CarryOnIncluded = %v, want true", got)
	}
	if got := *shared.Flights[0].CheckedBagsIncluded; got != 1 {
		t.Fatalf("shared CheckedBagsIncluded = %d, want 1", got)
	}
}

func TestDoFlightSearchSingleflight_ShortCallerDeadlineDoesNotPoisonSharedWork(t *testing.T) {
	key := "flight|HEL|NRT|2026-09-01|shared-timeout"

	var callCount atomic.Int64
	firstCtx, firstCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer firstCancel()
	firstDeadline, ok := firstCtx.Deadline()
	if !ok {
		t.Fatal("first caller context unexpectedly has no deadline")
	}

	sharedDeadlineCh := make(chan time.Time, 1)
	_, firstErr := doFlightSearchSingleflight(firstCtx, key, func(sharedCtx context.Context) (*models.FlightSearchResult, error) {
		callCount.Add(1)
		sharedDeadline, ok := sharedCtx.Deadline()
		if !ok {
			t.Fatal("shared context unexpectedly has no deadline")
		}
		sharedDeadlineCh <- sharedDeadline
		<-sharedCtx.Done()
		return nil, sharedCtx.Err()
	})

	if !errors.Is(firstErr, context.DeadlineExceeded) {
		t.Fatalf("firstErr = %v, want context deadline exceeded", firstErr)
	}
	sharedDeadline := <-sharedDeadlineCh
	if !sharedDeadline.After(firstDeadline.Add(100 * time.Millisecond)) {
		t.Fatalf("shared deadline %v unexpectedly inherited short caller deadline %v", sharedDeadline, firstDeadline)
	}
	secondCtx, secondCancel := context.WithTimeout(context.Background(), time.Second)
	defer secondCancel()
	secondResult, secondErr := doFlightSearchSingleflight(secondCtx, key, func(sharedCtx context.Context) (*models.FlightSearchResult, error) {
		callCount.Add(1)
		return &models.FlightSearchResult{Success: true, Count: 1}, nil
	})
	if secondErr != nil {
		t.Fatalf("secondErr = %v, want nil", secondErr)
	}
	if secondResult == nil || !secondResult.Success || secondResult.Count != 1 {
		t.Fatalf("secondResult = %#v, want successful result from a fresh execution", secondResult)
	}
	if got := callCount.Load(); got != 2 {
		t.Fatalf("callCount = %d, want 2 executions after the timed-out winner stops", got)
	}
}
