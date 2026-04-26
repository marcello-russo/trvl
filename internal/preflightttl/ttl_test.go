package preflightttl

// MIK-3089 (partial): tests for the AIMD adaptive preflight TTL.

import (
	"testing"
	"time"
)

func TestUpdate_SuccessExtendsAdditively(t *testing.T) {
	now := time.Now()
	s := Update(State{}, OutcomeSuccess, now)
	want := FloorTTL + AdditiveStep
	if s.CurrentTTL != want {
		t.Errorf("CurrentTTL=%v, want %v", s.CurrentTTL, want)
	}
	if s.ConsecutiveSuccesses != 1 {
		t.Errorf("ConsecutiveSuccesses=%d, want 1", s.ConsecutiveSuccesses)
	}
	if !s.LastSuccess.Equal(now) {
		t.Errorf("LastSuccess=%v, want %v", s.LastSuccess, now)
	}
}

func TestUpdate_SuccessClampsAtCeiling(t *testing.T) {
	s := State{CurrentTTL: CeilingTTL}
	got := Update(s, OutcomeSuccess, time.Now())
	if got.CurrentTTL != CeilingTTL {
		t.Errorf("CurrentTTL=%v, want %v (ceiling)", got.CurrentTTL, CeilingTTL)
	}
}

func TestUpdate_FailureHalves(t *testing.T) {
	s := State{CurrentTTL: 40 * time.Minute, ConsecutiveSuccesses: 5}
	got := Update(s, OutcomeFailure, time.Now())
	if got.CurrentTTL != 20*time.Minute {
		t.Errorf("CurrentTTL=%v, want 20m", got.CurrentTTL)
	}
	if got.ConsecutiveSuccesses != 0 {
		t.Errorf("ConsecutiveSuccesses=%d, want 0 (reset on failure)", got.ConsecutiveSuccesses)
	}
	if got.ConsecutiveFailures != 1 {
		t.Errorf("ConsecutiveFailures=%d, want 1", got.ConsecutiveFailures)
	}
}

func TestUpdate_FailureClampsAtFloor(t *testing.T) {
	s := State{CurrentTTL: 12 * time.Minute}
	got := Update(s, OutcomeFailure, time.Now())
	if got.CurrentTTL != FloorTTL {
		t.Errorf("CurrentTTL=%v, want floor %v", got.CurrentTTL, FloorTTL)
	}
}

func TestUpdate_NoOpDoesNothing(t *testing.T) {
	s := State{CurrentTTL: 30 * time.Minute, ConsecutiveSuccesses: 5}
	got := Update(s, OutcomeNoOp, time.Now())
	if got.CurrentTTL != s.CurrentTTL {
		t.Errorf("NoOp changed CurrentTTL: %v -> %v", s.CurrentTTL, got.CurrentTTL)
	}
	if got.ConsecutiveSuccesses != s.ConsecutiveSuccesses {
		t.Errorf("NoOp changed counter: %d -> %d", s.ConsecutiveSuccesses, got.ConsecutiveSuccesses)
	}
}

func TestUpdate_ZeroStateInitialisesAtFloor(t *testing.T) {
	got := Update(State{}, OutcomeFailure, time.Now())
	if got.CurrentTTL != FloorTTL {
		t.Errorf("zero -> failure should land at floor, got %v", got.CurrentTTL)
	}
}

func TestUpdate_FullyHealthyProviderReachesCeiling(t *testing.T) {
	s := State{}
	now := time.Now()
	for i := 0; i < 100; i++ {
		s = Update(s, OutcomeSuccess, now)
	}
	if s.CurrentTTL != CeilingTTL {
		t.Errorf("after 100 successes CurrentTTL=%v, want ceiling %v", s.CurrentTTL, CeilingTTL)
	}
}

func TestUpdate_RecoveryAfterFailureBlock(t *testing.T) {
	s := State{}
	now := time.Now()
	// Climb high.
	for i := 0; i < 30; i++ {
		s = Update(s, OutcomeSuccess, now)
	}
	high := s.CurrentTTL
	// One failure halves.
	s = Update(s, OutcomeFailure, now)
	if s.CurrentTTL >= high {
		t.Errorf("failure did not reduce CurrentTTL: %v then %v", high, s.CurrentTTL)
	}
	// Recover with two successes.
	s = Update(s, OutcomeSuccess, now)
	s = Update(s, OutcomeSuccess, now)
	if s.ConsecutiveSuccesses != 2 {
		t.Errorf("ConsecutiveSuccesses=%d, want 2 after recovery", s.ConsecutiveSuccesses)
	}
	if s.ConsecutiveFailures != 0 {
		t.Errorf("ConsecutiveFailures=%d, want 0 after success", s.ConsecutiveFailures)
	}
}

func TestExtended_TrueWhenAboveFloor(t *testing.T) {
	if Extended(State{CurrentTTL: FloorTTL}) {
		t.Errorf("Extended at floor should be false")
	}
	if !Extended(State{CurrentTTL: FloorTTL + time.Second}) {
		t.Errorf("Extended above floor should be true")
	}
}

func TestReset_ReturnsFloor(t *testing.T) {
	s := Reset()
	if s.CurrentTTL != FloorTTL {
		t.Errorf("Reset CurrentTTL=%v, want %v", s.CurrentTTL, FloorTTL)
	}
}

func TestUpdate_LogTwoFailuresFromCeilingDropToFloor(t *testing.T) {
	// 60min -> 30 -> 15 -> floor (10). Three failures should land at floor.
	s := State{CurrentTTL: CeilingTTL}
	now := time.Now()
	s = Update(s, OutcomeFailure, now) // 30
	s = Update(s, OutcomeFailure, now) // 15
	s = Update(s, OutcomeFailure, now) // 7.5 -> clamped to floor 10
	if s.CurrentTTL != FloorTTL {
		t.Errorf("after 3 failures from ceiling CurrentTTL=%v, want floor %v", s.CurrentTTL, FloorTTL)
	}
	if s.ConsecutiveFailures != 3 {
		t.Errorf("ConsecutiveFailures=%d, want 3", s.ConsecutiveFailures)
	}
}
