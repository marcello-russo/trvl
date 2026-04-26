// Package preflightttl computes an adaptive cache TTL for the
// provider preflight cache. The existing implementation pins the
// TTL at 10 minutes regardless of how stable a provider's session
// tokens have proven to be. This package exposes a pure-function
// AIMD-style controller (additive-increase, multiplicative-decrease)
// that callers can wire into the runtime: every successful request
// extends the TTL by a small step, every failure halves it back
// toward the floor.
//
// MIK-3089 (partial). Pure function — no I/O, no clock dependency
// beyond the time.Time inputs the caller supplies — so the math is
// trivially testable. The runtime wiring (mutating the cached TTL
// per-provider on the existing preflightTTL constant) ships as a
// separate change so the controller math can land first.
package preflightttl

import "time"

// Bounds for the controller. Floor matches the conservative WAF
// session expiry; ceiling matches the AC's 60-minute cap. Step is
// the additive growth per successful request — chosen so a fully
// healthy provider reaches the ceiling after about 50 consecutive
// hits, which is the tail length we observe in practice.
const (
	// FloorTTL is the minimum extended TTL the controller will emit.
	// Below this we behave like the legacy 10-minute pin.
	FloorTTL = 10 * time.Minute
	// CeilingTTL caps the extended TTL. Matches the AC.
	CeilingTTL = 60 * time.Minute
	// AdditiveStep is the amount the TTL grows on each consecutive
	// successful preflight reuse.
	AdditiveStep = 1 * time.Minute
	// MultiplicativeFactor is the divisor applied on failure. 2 means
	// a failure halves the TTL — fast enough to drop back to floor in
	// log2((ceiling-floor)/step) failures. From ceiling that is six.
	MultiplicativeFactor = 2
)

// State captures the controller's working memory. Callers persist
// one State per provider, hand it to Update, and store the returned
// value back. Zero-value State means "fresh provider, start at
// floor"; the controller fills LastSuccess and ConsecutiveFailures
// on the first call so callers do not need to seed anything.
type State struct {
	CurrentTTL           time.Duration
	LastSuccess          time.Time
	ConsecutiveSuccesses int
	ConsecutiveFailures  int
}

// Outcome labels the call site so the controller can pick the right
// branch without requiring the caller to import multiple constants.
type Outcome int

const (
	// OutcomeNoOp is the zero value; treated as a no-op so callers
	// can skip Update entirely on cache-hit paths.
	OutcomeNoOp Outcome = iota
	// OutcomeSuccess — the preflight payload survived a real request.
	// Triggers additive increase on CurrentTTL.
	OutcomeSuccess
	// OutcomeFailure — the preflight payload was rejected (token
	// expired, signature invalid, WAF challenge re-issued). Triggers
	// multiplicative decrease.
	OutcomeFailure
)

// Update returns the next State given the current state, the call
// outcome, and the wall-clock time at which the outcome was observed.
// Pure function: no globals, no clock, no I/O. Caller persists the
// returned State and uses State.CurrentTTL on the next preflight.
func Update(s State, outcome Outcome, now time.Time) State {
	if s.CurrentTTL == 0 {
		s.CurrentTTL = FloorTTL
	}
	switch outcome {
	case OutcomeSuccess:
		s.ConsecutiveSuccesses++
		s.ConsecutiveFailures = 0
		s.LastSuccess = now
		next := s.CurrentTTL + AdditiveStep
		if next > CeilingTTL {
			next = CeilingTTL
		}
		s.CurrentTTL = next
	case OutcomeFailure:
		s.ConsecutiveFailures++
		s.ConsecutiveSuccesses = 0
		next := s.CurrentTTL / MultiplicativeFactor
		if next < FloorTTL {
			next = FloorTTL
		}
		s.CurrentTTL = next
	}
	return s
}

// Reset returns the State a caller should use to forget all history
// (e.g. on a configuration change). Equivalent to a zero-value State
// but exposed as a function so future controllers with non-zero
// defaults stay source-compatible.
func Reset() State {
	return State{CurrentTTL: FloorTTL}
}

// Extended reports whether the current TTL is above the floor —
// useful for the structured slog field per the AC ("ttl_extended:
// true" when adaptive math has actually kicked in).
func Extended(s State) bool {
	return s.CurrentTTL > FloorTTL
}
