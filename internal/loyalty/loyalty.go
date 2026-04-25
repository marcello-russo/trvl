// Package loyalty tracks airline / hotel program balances and forecasts
// when status renewal deadlines and points-expiry windows are coming
// up. The opportunity-watcher daemon (MIK-3067) consumes Warnings to
// surface "use these points before they vanish" / "one more KL segment
// to hold Gold" messages in the daily digest. (MIK-3082.)
package loyalty

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// Balance is one program's snapshot. Empty fields mean "not tracked"
// and are skipped during warning emission.
type Balance struct {
	// Program is the program label (e.g. "Flying Blue", "Marriott Bonvoy").
	Program string `json:"program"`
	// Points / miles balance. Zero means unspecified.
	Balance int `json:"balance,omitempty"`
	// ExpiresAt is the calendar date the points expire (zero = no
	// known expiry).
	ExpiresAt time.Time `json:"expires_at,omitempty"`
	// StatusTier is the current tier name (e.g. "Gold").
	StatusTier string `json:"status_tier,omitempty"`
	// StatusRenewalDeadline is when the user must hit qualification
	// thresholds to retain StatusTier.
	StatusRenewalDeadline time.Time `json:"status_renewal_deadline,omitempty"`
	// QualSegmentsNeeded is the segments still needed before the
	// renewal deadline to keep StatusTier.
	QualSegmentsNeeded int `json:"qual_segments_needed,omitempty"`
}

// Snapshot is the user's full loyalty surface. UpdatedAt is when the
// snapshot was last refreshed (manually or via webhook).
type Snapshot struct {
	Balances  []Balance `json:"balances,omitempty"`
	UpdatedAt time.Time `json:"updated_at,omitempty"`
}

// WarningKind classifies a Warning so the daemon can format/route it.
type WarningKind string

const (
	// WarningPointsExpiring fires when ExpiresAt is within the lead
	// window and Balance > 0.
	WarningPointsExpiring WarningKind = "points_expiring"
	// WarningStatusRenewal fires when StatusRenewalDeadline is within
	// the lead window and QualSegmentsNeeded > 0.
	WarningStatusRenewal WarningKind = "status_renewal"
)

// Warning is one actionable item produced by Warnings().
type Warning struct {
	Kind     WarningKind
	Program  string
	Deadline time.Time
	DaysLeft int
	Message  string
}

// DefaultLeadDays is the canonical 60-day pre-expiry window per AC.
const DefaultLeadDays = 60

// Warnings inspects every balance in the snapshot and returns the
// subset with deadlines inside [now, now+leadDays]. Sorted ascending
// by deadline so the most urgent appears first.
func Warnings(snap Snapshot, now time.Time, leadDays int) []Warning {
	if leadDays <= 0 {
		return nil
	}
	cutoff := now.AddDate(0, 0, leadDays)
	var out []Warning
	for _, b := range snap.Balances {
		if !b.ExpiresAt.IsZero() && b.Balance > 0 && !b.ExpiresAt.Before(now) && !b.ExpiresAt.After(cutoff) {
			out = append(out, Warning{
				Kind:     WarningPointsExpiring,
				Program:  b.Program,
				Deadline: b.ExpiresAt,
				DaysLeft: daysBetween(now, b.ExpiresAt),
				Message:  fmt.Sprintf("%d %s points expire in %d days (%s)", b.Balance, b.Program, daysBetween(now, b.ExpiresAt), b.ExpiresAt.Format("2006-01-02")),
			})
		}
		if !b.StatusRenewalDeadline.IsZero() && b.QualSegmentsNeeded > 0 && !b.StatusRenewalDeadline.Before(now) && !b.StatusRenewalDeadline.After(cutoff) {
			out = append(out, Warning{
				Kind:     WarningStatusRenewal,
				Program:  b.Program,
				Deadline: b.StatusRenewalDeadline,
				DaysLeft: daysBetween(now, b.StatusRenewalDeadline),
				Message:  fmt.Sprintf("%s %s renewal: %d more segment(s) needed by %s (in %d days)", b.Program, b.StatusTier, b.QualSegmentsNeeded, b.StatusRenewalDeadline.Format("2006-01-02"), daysBetween(now, b.StatusRenewalDeadline)),
			})
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		return out[i].Deadline.Before(out[j].Deadline)
	})
	return out
}

// daysBetween returns the number of full calendar days from a to b
// (UTC), rounded down. Negative when a > b.
func daysBetween(a, b time.Time) int {
	ay, am, ad := a.Date()
	by, bm, bd := b.Date()
	da := time.Date(ay, am, ad, 0, 0, 0, 0, time.UTC)
	db := time.Date(by, bm, bd, 0, 0, 0, 0, time.UTC)
	return int(db.Sub(da).Hours() / 24)
}

// FindByProgram returns the index of the balance whose Program matches
// `name` case-insensitively, or -1. Used by the manual-update CLI path.
func FindByProgram(snap Snapshot, name string) int {
	target := strings.ToLower(strings.TrimSpace(name))
	for i, b := range snap.Balances {
		if strings.EqualFold(strings.TrimSpace(b.Program), target) {
			return i
		}
	}
	return -1
}

// Upsert inserts or updates a balance keyed on Program (case-insensitive).
// Returns the new snapshot — Snapshot is treated as a value, callers
// hold the canonical copy.
func Upsert(snap Snapshot, b Balance, now time.Time) Snapshot {
	if i := FindByProgram(snap, b.Program); i >= 0 {
		snap.Balances[i] = b
	} else {
		snap.Balances = append(snap.Balances, b)
	}
	snap.UpdatedAt = now
	return snap
}
