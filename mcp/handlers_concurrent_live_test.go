package mcp

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/testutil"
)

func TestToolsCallSearchFlights_ConcurrentLiveIntegration(t *testing.T) {
	t.Parallel()
	testutil.RequireLiveIntegration(t)

	results := runConcurrentToolCalls(t, ToolCallParams{
		Name: "search_flights",
		Arguments: map[string]any{
			"origin":         "HEL",
			"destination":    "NRT",
			"departure_date": "2026-05-15",
		},
	})

	for i, result := range results {
		var structured models.FlightSearchResult
		assertStructuredResult(t, "flights", i, result, &structured)
	}
}

func TestToolsCallSearchGround_ConcurrentLiveIntegration(t *testing.T) {
	t.Parallel()
	testutil.RequireLiveIntegration(t)

	results := runConcurrentToolCalls(t, ToolCallParams{
		Name: "search_ground",
		Arguments: map[string]any{
			"from": "Helsinki",
			"to":   "Espoo",
			"date": "2026-05-15",
		},
	})

	for i, result := range results {
		var structured models.GroundSearchResult
		assertStructuredResult(t, "ground", i, result, &structured)
	}
}

func TestToolsCallSearchHotels_ConcurrentLiveIntegration(t *testing.T) {
	t.Parallel()
	testutil.RequireLiveIntegration(t)

	results := runConcurrentToolCalls(t, ToolCallParams{
		Name: "search_hotels",
		Arguments: map[string]any{
			"location":  "Helsinki",
			"check_in":  "2026-05-15",
			"check_out": "2026-05-18",
		},
	})

	for i, result := range results {
		var structured models.HotelSearchResult
		assertStructuredResult(t, "hotels", i, result, &structured)
	}
}

func runConcurrentToolCalls(t *testing.T, params ToolCallParams) []*ToolCallResult {
	t.Helper()

	const callers = 2

	results := make([]*ToolCallResult, callers)
	errs := make([]error, callers)

	var wg sync.WaitGroup
	start := make(chan struct{})

	for i := 0; i < callers; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			<-start
			results[idx], errs[idx] = callToolRequest(params, idx+1)
		}(i)
	}

	close(start)
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Fatalf("caller %d failed: %v", i, err)
		}
	}

	return results
}

func callToolRequest(params ToolCallParams, id int) (*ToolCallResult, error) {
	s := NewServer()

	rawParams, err := json.Marshal(params)
	if err != nil {
		return nil, fmt.Errorf("marshal params: %w", err)
	}

	req := Request{
		JSONRPC: "2.0",
		ID:      id,
		Method:  "tools/call",
		Params:  rawParams,
	}

	reqBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	in := bytes.NewBuffer(append(reqBytes, '\n'))
	out := &bytes.Buffer{}

	if err := s.ServeStdio(in, out); err != nil {
		return nil, fmt.Errorf("ServeStdio: %w", err)
	}

	resp, err := decodeToolResponse(out.String())
	if err != nil {
		return nil, err
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("rpc error %d: %s", resp.Error.Code, resp.Error.Message)
	}

	resultJSON, err := json.Marshal(resp.Result)
	if err != nil {
		return nil, fmt.Errorf("marshal result: %w", err)
	}

	var result ToolCallResult
	if err := json.Unmarshal(resultJSON, &result); err != nil {
		return nil, fmt.Errorf("unmarshal result: %w", err)
	}
	if result.IsError {
		return nil, fmt.Errorf("tool returned error result: %s", firstContentText(result.Content))
	}

	return &result, nil
}

func decodeToolResponse(raw string) (*Response, error) {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}

		var resp Response
		if err := json.Unmarshal([]byte(line), &resp); err != nil {
			continue
		}
		if resp.ID != nil || resp.Error != nil {
			return &resp, nil
		}
	}

	return nil, fmt.Errorf("no JSON-RPC response found in output")
}

func assertStructuredResult(t *testing.T, tool string, caller int, result *ToolCallResult, out any) {
	t.Helper()

	if result == nil {
		t.Fatalf("%s caller %d returned nil result", tool, caller)
	}
	if len(result.Content) == 0 {
		t.Fatalf("%s caller %d returned no content blocks", tool, caller)
	}
	if result.StructuredContent == nil {
		t.Fatalf("%s caller %d returned nil structured content", tool, caller)
	}

	structuredJSON, err := json.Marshal(result.StructuredContent)
	if err != nil {
		t.Fatalf("%s caller %d marshal structured content: %v", tool, caller, err)
	}
	if err := json.Unmarshal(structuredJSON, out); err != nil {
		t.Fatalf("%s caller %d unmarshal structured content: %v", tool, caller, err)
	}
}

func firstContentText(content []ContentBlock) string {
	if len(content) == 0 {
		return ""
	}
	return content[0].Text
}
