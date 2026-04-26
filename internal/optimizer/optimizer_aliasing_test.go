package optimizer

import (
	"sync"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
)

func optimizerAliasingCandidate() *candidate {
	return &candidate{
		searched:     true,
		origin:       "TLL",
		dest:         "BCN",
		departDate:   "2026-06-01",
		strategy:     "Fly from Tallinn",
		hackTypes:    []string{"positioning", "departure_tax"},
		transferCost: 30,
		currency:     "EUR",
		baseCost:     120,
		allInCost:    150,
		flights: []models.FlightResult{
			{
				Price:    120,
				Currency: "EUR",
			},
		},
	}
}

func TestCandidateToOption_clonesHacksApplied(t *testing.T) {
	c := optimizerAliasingCandidate()

	opt := candidateToOption(c, 1, OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
	})
	opt.HacksApplied[0] = "mutated"

	if got := c.hackTypes[0]; got != "positioning" {
		t.Fatalf("candidate hackTypes mutated through BookingOption: got %q", got)
	}
}

func TestCandidateToOption_parallelOptionsDoNotShareHacksApplied(t *testing.T) {
	c := optimizerAliasingCandidate()
	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
	}

	const workers = 24
	start := make(chan struct{})
	results := make(chan BookingOption, workers)

	var wg sync.WaitGroup
	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func(rank int) {
			defer wg.Done()
			<-start
			results <- candidateToOption(c, rank, input)
		}(i + 1)
	}

	close(start)
	wg.Wait()
	close(results)

	var opts []BookingOption
	for opt := range results {
		opts = append(opts, opt)
	}
	if len(opts) != workers {
		t.Fatalf("got %d options, want %d", len(opts), workers)
	}

	opts[0].HacksApplied[0] = "mutated"
	for i := 1; i < len(opts); i++ {
		if got := opts[i].HacksApplied[0]; got != "positioning" {
			t.Fatalf("option %d shared HacksApplied backing storage: got %q", i, got)
		}
	}
	if got := c.hackTypes[0]; got != "positioning" {
		t.Fatalf("candidate hackTypes mutated after parallel conversion: got %q", got)
	}
}

func TestRankCandidates_clonesHacksAppliedAcrossCalls(t *testing.T) {
	candidates := []*candidate{
		{
			origin:     "HEL",
			dest:       "BCN",
			departDate: "2026-06-01",
			strategy:   "Direct booking",
			currency:   "EUR",
			searched:   true,
			baseCost:   180,
			allInCost:  180,
			flights: []models.FlightResult{
				{Price: 180, Currency: "EUR"},
			},
		},
		optimizerAliasingCandidate(),
	}

	input := OptimizeInput{
		Origin:      "HEL",
		Destination: "BCN",
		Currency:    "EUR",
		MaxResults:  2,
	}

	res1 := rankCandidates(candidates, input)
	res2 := rankCandidates(candidates, input)

	var hacked1, hacked2 *BookingOption
	for i := range res1.Options {
		if len(res1.Options[i].HacksApplied) > 0 {
			hacked1 = &res1.Options[i]
			break
		}
	}
	for i := range res2.Options {
		if len(res2.Options[i].HacksApplied) > 0 {
			hacked2 = &res2.Options[i]
			break
		}
	}
	if hacked1 == nil || hacked2 == nil {
		t.Fatal("expected hacked booking options in both ranking results")
	}

	hacked1.HacksApplied[0] = "mutated"

	if got := hacked2.HacksApplied[0]; got != "positioning" {
		t.Fatalf("rankCandidates reused HacksApplied backing storage across calls: got %q", got)
	}
	if got := candidates[1].hackTypes[0]; got != "positioning" {
		t.Fatalf("candidate hackTypes mutated across rankCandidates calls: got %q", got)
	}
}
