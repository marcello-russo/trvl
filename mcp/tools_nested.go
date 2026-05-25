package mcp

// optimize_nested_rt — MCP surface for the nested/overlapping round-trip
// combinator (internal/hacks/nested_rt.go). Prices both visit windows live
// (parallel fan-out across the existing flight providers), feeds the priced
// VisitWindows into the pure ranker, and returns the cheapest pairings vs the
// naive two-round-trip baseline. Serves the two-base commute pattern (e.g.
// HEL+AMS) where overlapping round-trips beat booking two separate returns.
//
// Tracking: MIK-3076.

import (
	"context"
	"fmt"
	"sync"

	"github.com/MikkoParkkola/trvl/internal/flights"
	"github.com/MikkoParkkola/trvl/internal/hacks"
)

// legPricer returns the cheapest price for a search; returnDate empty = one-way.
// Injected so tests run offline.
type legPricer func(ctx context.Context, origin, dest, date, returnDate string) float64

// liveLegPricer prices via the real flight providers (cheapest comparable).
func liveLegPricer(ctx context.Context, origin, dest, date, returnDate string) float64 {
	res, err := flights.SearchFlights(ctx, origin, dest, date, flights.SearchOptions{ReturnDate: returnDate})
	if err != nil || res == nil || len(res.Flights) == 0 {
		return 0
	}
	best := res.Flights[0].PriceForRanking()
	for _, f := range res.Flights[1:] {
		if p := f.PriceForRanking(); p > 0 && (best == 0 || p < best) {
			best = p
		}
	}
	return best
}

// priceVisitWindow fans out the four quotes a VisitWindow needs, in parallel.
func priceVisitWindow(ctx context.Context, a, b, departDate, returnDate string, price legPricer) hacks.VisitWindow {
	var w hacks.VisitWindow
	w.DepartDate, w.ReturnDate = departDate, returnDate
	var wg sync.WaitGroup
	wg.Add(4)
	go func() { defer wg.Done(); w.RoundTripFromA = price(ctx, a, b, departDate, returnDate) }()
	go func() { defer wg.Done(); w.LegFromAToB = price(ctx, a, b, departDate, "") }()
	go func() { defer wg.Done(); w.LegFromBToA = price(ctx, b, a, returnDate, "") }()
	go func() { defer wg.Done(); w.RoundTripFromB = price(ctx, b, a, departDate, returnDate) }()
	wg.Wait()
	return w
}

// nestedRTResult is the structured output of optimize_nested_rt.
type nestedRTResult struct {
	Origin   string                `json:"origin"`
	Dest     string                `json:"destination"`
	Pairings []hacks.PairingResult `json:"pairings"`
	BestSave float64               `json:"best_savings_eur"`
}

func handleOptimizeNestedRT(ctx context.Context, args map[string]any, _ ElicitFunc, _ SamplingFunc, _ ProgressFunc) ([]ContentBlock, interface{}, error) {
	return optimizeNestedRT(ctx, args, liveLegPricer)
}

func optimizeNestedRT(ctx context.Context, args map[string]any, price legPricer) ([]ContentBlock, interface{}, error) {
	origin, dest, err := validateOriginDest(args)
	if err != nil {
		return nil, nil, err
	}
	w1d, err := validateDate(args, "window1_depart")
	if err != nil {
		return nil, nil, err
	}
	w1r, err := validateDate(args, "window1_return")
	if err != nil {
		return nil, nil, err
	}
	w2d, err := validateDate(args, "window2_depart")
	if err != nil {
		return nil, nil, err
	}
	w2r, err := validateDate(args, "window2_return")
	if err != nil {
		return nil, nil, err
	}

	// Price both windows concurrently.
	var w1, w2 hacks.VisitWindow
	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); w1 = priceVisitWindow(ctx, origin, dest, w1d, w1r, price) }()
	go func() { defer wg.Done(); w2 = priceVisitWindow(ctx, origin, dest, w2d, w2r, price) }()
	wg.Wait()

	ranked := hacks.EnumerateAndRank(w1, w2, 3)
	if len(ranked) == 0 {
		return nil, nil, fmt.Errorf("could not price both visit windows for %s<->%s", origin, dest)
	}
	out := nestedRTResult{Origin: origin, Dest: dest, Pairings: ranked}
	for _, p := range ranked {
		if p.SavingsEUR > out.BestSave {
			out.BestSave = p.SavingsEUR
		}
	}

	summary := fmt.Sprintf("Best plan for two %s<->%s visits: %s at %.0f", origin, dest, ranked[0].Kind, ranked[0].Cost)
	if out.BestSave > 0 {
		summary += fmt.Sprintf(" (saves %.0f vs two separate round-trips)", out.BestSave)
	} else {
		summary += " (no saving vs two separate round-trips)"
	}
	blocks, err := buildAnnotatedContentBlocks(summary, out)
	if err != nil {
		return nil, nil, err
	}
	return blocks, out, nil
}

func nestedRTTool() ToolDef {
	return ToolDef{
		Name:        "optimize_nested_rt",
		Title:       "Nested Round-Trip Optimizer",
		Description: "For two visits between the same two cities, find whether overlapping/nested round-trips beat booking two separate returns. Prices both windows live and ranks the cheapest pairing.",
		InputSchema: InputSchema{
			Type: "object",
			Properties: map[string]Property{
				"origin":         {Type: "string", Description: "Home IATA code (side A), e.g. HEL"},
				"destination":    {Type: "string", Description: "Visited IATA code (side B), e.g. AMS"},
				"window1_depart": {Type: "string", Description: "First visit outbound date YYYY-MM-DD"},
				"window1_return": {Type: "string", Description: "First visit return date YYYY-MM-DD"},
				"window2_depart": {Type: "string", Description: "Second visit outbound date YYYY-MM-DD"},
				"window2_return": {Type: "string", Description: "Second visit return date YYYY-MM-DD"},
			},
			Required: []string{"origin", "destination", "window1_depart", "window1_return", "window2_depart", "window2_return"},
		},
		Annotations: &ToolAnnotations{Title: "Nested Round-Trip Optimizer", ReadOnlyHint: true, OpenWorldHint: true},
	}
}
