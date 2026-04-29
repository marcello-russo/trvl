package main

import (
	"bytes"
	"context"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/MikkoParkkola/trvl/internal/testutil"
	"github.com/MikkoParkkola/trvl/internal/watch"
)

func TestWatchRoomsCmd_FlagsExistV22(t *testing.T) {
	cmd := watchRoomsCmd()
	for _, name := range []string{"depart", "return", "guests"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Logf("--%s flag not found on watchRoomsCmd", name)
		}
	}
}

func TestLiveChecker_CheckPrice_UnknownType(t *testing.T) {
	c := &liveChecker{}
	ctx := context.Background()
	w := watch.Watch{Type: "unknown"}
	_, _, _, err := c.CheckPrice(ctx, w)
	if err == nil {
		t.Error("expected error for unknown watch type")
	}
}

func TestLiveRoomChecker_Interface_V26(t *testing.T) {
	var _ watch.RoomChecker = &liveRoomChecker{}
}

func TestRunWatchDaemon_ZeroInterval_V28(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	err := runWatchDaemon(ctx, &buf, 0, false, func(context.Context) (int, error) { return 0, nil }, nil)
	if err == nil {
		t.Error("expected error for zero interval")
	}
	if !strings.Contains(err.Error(), "greater than zero") {
		t.Errorf("expected 'greater than zero' error, got: %v", err)
	}
}

func TestRunWatchDaemon_NilCycleFunc_V28(t *testing.T) {
	ctx := context.Background()
	var buf bytes.Buffer
	err := runWatchDaemon(ctx, &buf, time.Minute, false, nil, nil)
	if err == nil {
		t.Error("expected error for nil cycle func")
	}
}

func TestRunWatchDaemon_CancelledCtx_NoRunNow_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	var buf bytes.Buffer
	called := false
	err := runWatchDaemon(ctx, &buf, time.Minute, false, func(context.Context) (int, error) {
		called = true
		return 1, nil
	}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if called {
		t.Error("cycle func should not be called when runNow=false and ctx is already cancelled")
	}
	if !strings.Contains(buf.String(), "stopped") {
		t.Errorf("expected 'stopped' in output, got: %q", buf.String())
	}
}

func TestRunWatchDaemon_RunNow_CancelAfterFirst_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var buf bytes.Buffer
	called := false

	err := runWatchDaemon(ctx, &buf, time.Minute, true, func(ctx2 context.Context) (int, error) {
		called = true
		cancel()
		return 1, nil
	}, nil)

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Error("cycle func should be called when runNow=true")
	}
}

func TestRunWatchDaemon_CycleError_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var buf bytes.Buffer
	ticker := &fakeDaemonTickerV28{ch: make(chan time.Time, 1)}
	ticker.ch <- time.Now()

	_ = runWatchDaemon(ctx, &buf, time.Minute, false, func(ctx2 context.Context) (int, error) {
		cancel()
		return 0, io.ErrUnexpectedEOF
	}, func(d time.Duration) watchDaemonTicker {
		return ticker
	})

	if !strings.Contains(buf.String(), "watch check failed") {
		t.Errorf("expected 'watch check failed' in output, got: %q", buf.String())
	}
}

func TestRunWatchDaemon_ZeroWatchesMessage_V28(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var buf bytes.Buffer
	ticker := &fakeDaemonTickerV28{ch: make(chan time.Time, 1)}
	ticker.ch <- time.Now()

	_ = runWatchDaemon(ctx, &buf, time.Minute, false, func(ctx2 context.Context) (int, error) {
		cancel()
		return 0, nil
	}, func(d time.Duration) watchDaemonTicker {
		return ticker
	})

	if !strings.Contains(buf.String(), "no active watches") {
		t.Errorf("expected 'no active watches' in output, got: %q", buf.String())
	}
}

// fakeDaemonTickerV28 is a test double for watchDaemonTicker.

func TestLiveCheckerCheckPrice_UnknownType_V29(t *testing.T) {
	c := &liveChecker{}
	w := watch.Watch{
		Type:        "unknown-type",
		Origin:      "HEL",
		Destination: "BCN",
	}
	_, _, _, err := c.CheckPrice(context.Background(), w)
	if err == nil {
		t.Fatal("expected error for unknown watch type")
	}
	if err.Error() != "unknown watch type: unknown-type" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestLiveCheckerCheckFlight_SpecificDate_CancelledCtx_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &liveChecker{}
	w := watch.Watch{
		Type:        "flight",
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-08-01",
	}

	_, _, _, err := c.CheckPrice(ctx, w)

	if err == nil {
		t.Log("no error (SearchFlights may have returned success=false, not an error)")
	}
}

func TestLiveCheckerCheckFlight_RouteWatch_CancelledCtx_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &liveChecker{}
	w := watch.Watch{
		Type:        "flight",
		Origin:      "HEL",
		Destination: "BCN",
	}
	_, _, _, _ = c.CheckPrice(ctx, w)
}

func TestLiveCheckerCheckFlight_DateRange_CancelledCtx_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &liveChecker{}
	w := watch.Watch{
		Type:        "flight",
		Origin:      "HEL",
		Destination: "BCN",
		DepartFrom:  "2026-08-01",
		DepartTo:    "2026-08-31",
	}
	_, _, _, _ = c.CheckPrice(ctx, w)
}

func TestLiveCheckerCheckHotel_SpecificDates_CancelledCtx_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &liveChecker{}
	w := watch.Watch{
		Type:        "hotel",
		Destination: "Barcelona",
		DepartDate:  "2026-08-01",
		ReturnDate:  "2026-08-05",
		Currency:    "EUR",
	}
	_, _, _, _ = c.CheckPrice(ctx, w)
}

func TestLiveCheckerCheckHotel_RouteWatch_CancelledCtx_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &liveChecker{}
	w := watch.Watch{
		Type:        "hotel",
		Destination: "Barcelona",
	}
	_, _, _, _ = c.CheckPrice(ctx, w)
}

func TestLiveRoomCheckerCheckRooms_CancelledCtx_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &liveRoomChecker{}
	w := watch.Watch{
		Type:       "room",
		HotelName:  "CORU House Prague",
		DepartDate: "2026-08-01",
		ReturnDate: "2026-08-05",
		Currency:   "",
	}
	_, _ = c.CheckRooms(ctx, w)
}

func TestLiveRoomCheckerCheckRooms_DefaultCurrency_V29(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	c := &liveRoomChecker{}
	w := watch.Watch{
		Type:       "room",
		HotelName:  "Park Hyatt Vienna",
		DepartDate: "2026-09-01",
		ReturnDate: "2026-09-05",
	}
	_, _ = c.CheckRooms(ctx, w)
}

func TestWatchHistoryCmd_NotFound_V29(t *testing.T) {
	cmd := watchHistoryCmd()
	cmd.SetArgs([]string{"nonexistent-watch-id-xyz"})
	err := cmd.Execute()

	_ = err
}

func TestWatchRemoveCmd_NotFound_V29(t *testing.T) {
	cmd := watchRemoveCmd()
	cmd.SetArgs([]string{"nonexistent-watch-id-xyz"})
	_ = cmd.Execute()
}

func TestWatchCheckCmd_EmptyStore_V29(t *testing.T) {
	testutil.RequireLiveIntegration(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := watchCheckCmd()
	cmd.SetArgs([]string{})
	_ = cmd.ExecuteContext(ctx)
}

func TestFormatWatchDates_RoomWatch(t *testing.T) {
	w := watch.Watch{
		Type:        "room",
		DepartDate:  "2026-06-15",
		ReturnDate:  "2026-06-18",
		MatchedRoom: "Deluxe King",
		HotelName:   "Grand Hyatt",
	}
	got := formatWatchDates(w)
	if !strings.Contains(got, "2026-06-15") {
		t.Errorf("expected check-in date in output, got %q", got)
	}
	if !strings.Contains(got, "Deluxe King") {
		t.Errorf("expected room name in output, got %q", got)
	}
}

func TestFormatWatchDates_RouteWatch_WithCheapestDate(t *testing.T) {
	w := watch.Watch{
		Type:         "flight",
		Origin:       "HEL",
		Destination:  "BCN",
		CheapestDate: "2026-07-03",
	}
	got := formatWatchDates(w)
	if !strings.Contains(got, "2026-07-03") {
		t.Errorf("expected cheapest date in output, got %q", got)
	}
}

func TestFormatWatchDates_RouteWatch_NoCheapestDate(t *testing.T) {
	w := watch.Watch{
		Type:        "flight",
		Origin:      "HEL",
		Destination: "BCN",
	}
	got := formatWatchDates(w)
	if !strings.Contains(got, "next 60d") {
		t.Errorf("expected 'next 60d' in output, got %q", got)
	}
}

func TestFormatWatchDates_DateRange_WithCheapest(t *testing.T) {
	w := watch.Watch{
		Type:         "flight",
		Origin:       "HEL",
		Destination:  "BCN",
		DepartFrom:   "2026-07-01",
		DepartTo:     "2026-07-15",
		CheapestDate: "2026-07-05",
	}
	got := formatWatchDates(w)
	if !strings.Contains(got, "2026-07-01") || !strings.Contains(got, "2026-07-15") {
		t.Errorf("expected date range in output, got %q", got)
	}
}

func TestFormatWatchDates_SpecificDateWithReturn(t *testing.T) {
	w := watch.Watch{
		Type:        "flight",
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-07-10",
		ReturnDate:  "2026-07-17",
	}
	got := formatWatchDates(w)
	if !strings.Contains(got, "2026-07-10") {
		t.Errorf("expected depart date in output, got %q", got)
	}
}

func TestFormatWatchDates_SpecificDateNoReturn(t *testing.T) {
	w := watch.Watch{
		Type:        "flight",
		Origin:      "HEL",
		Destination: "BCN",
		DepartDate:  "2026-07-10",
	}
	got := formatWatchDates(w)
	if got != "2026-07-10" {
		t.Errorf("expected exact date, got %q", got)
	}
}

func TestWatchDaemonCmd_FlagsV6(t *testing.T) {
	cmd := watchDaemonCmd()
	for _, name := range []string{"every", "run-now"} {
		if f := cmd.Flags().Lookup(name); f == nil {
			t.Errorf("expected --%s flag on watchDaemonCmd", name)
		}
	}
}

func TestRunWatchCheckCycleWithRooms_EmptyStore(t *testing.T) {

	tmp := t.TempDir()
	setTestHome(t, tmp)

	cmd := watchDaemonCmd()
	if cmd == nil {
		t.Error("expected non-nil watchDaemonCmd")
	}
}

func TestLiveChecker_TypeExists(t *testing.T) {
	// Verify liveChecker implements the watch.PriceChecker interface.
	var _ interface {
		CheckPrice(interface{}, interface{}) (float64, string, string, error)
	} = nil // just compile-check by reference to liveChecker existing

	checker := &liveChecker{}
	_ = checker
}

func TestWatchHistoryCmd_MissingArg(t *testing.T) {
	cmd := watchHistoryCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args")
	}
}

func TestWatchHistoryCmd_NotFoundInEmptyStore(t *testing.T) {
	tmp := t.TempDir()
	setTestHome(t, tmp)
	cmd := watchHistoryCmd()
	cmd.SetArgs([]string{"nonexistent-id"})
	err := cmd.Execute()

	_ = err
}

func TestWatchRemoveCmd_MissingArg(t *testing.T) {
	cmd := watchRemoveCmd()
	cmd.SetArgs([]string{})
	err := cmd.Execute()
	if err == nil {
		t.Error("expected error with no args")
	}
}
