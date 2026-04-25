package providers

// MIK-3074: classify provider failures into a typed enum so callers (LLMs,
// dashboards) can pick a remediation path without parsing free-form error
// text. The free-text hint is preserved alongside the code for humans.

import (
	"errors"
	"net"
	"strings"
)

// FixHintCode is a stable, machine-readable classification of provider
// failures. Codes are intentionally narrow: each one points at a specific
// remediation. Add new codes rather than overloading existing ones.
type FixHintCode string

const (
	// FixHintAkamaiBlock — WAF rejected the request (Akamai/Booking-style
	// 403/202 challenge). Refresh browser cookies.
	FixHintAkamaiBlock FixHintCode = "AKAMAI_BLOCK"
	// FixHintDNSFail — host did not resolve. Network/config issue.
	FixHintDNSFail FixHintCode = "DNS_FAIL"
	// FixHintTLSTimeout — TLS handshake/certificate failure.
	FixHintTLSTimeout FixHintCode = "TLS_TIMEOUT"
	// FixHintCookieExpired — session state stale; re-auth needed.
	FixHintCookieExpired FixHintCode = "COOKIE_EXPIRED"
	// FixHintRateLimited — provider returned 429 / explicit rate-limit signal.
	FixHintRateLimited FixHintCode = "RATE_LIMITED"
	// FixHintResponseShape — API contract drift (results_path miss).
	FixHintResponseShape FixHintCode = "RESPONSE_SHAPE_CHANGED"
	// FixHintPreflightFailed — generic preflight failure with no narrower signal.
	FixHintPreflightFailed FixHintCode = "PREFLIGHT_FAILED"
	// FixHintUnclassified — error did not match any known pattern.
	FixHintUnclassified FixHintCode = "UNCLASSIFIED"
)

// classifyProviderError inspects err and returns a structured code plus a
// free-text remediation hint. Pure: no I/O, safe to call from any goroutine.
//
// Order matters: more specific signals (rate limit, DNS) are checked before
// broader ones (preflight, WAF) so a 429 is never mis-bucketed as AKAMAI_BLOCK.
func classifyProviderError(err error) (FixHintCode, string) {
	if err == nil {
		return FixHintUnclassified, ""
	}
	msg := strings.ToLower(err.Error())

	// DNS — typed match first to catch wrapped *net.DNSError even when its
	// .Error() text starts with "lookup foo:" rather than "no such host".
	var dnsErr *net.DNSError
	if errors.As(err, &dnsErr) || strings.Contains(msg, "no such host") || strings.Contains(msg, "dns lookup") {
		return FixHintDNSFail, "DNS resolution failed. Check network connectivity and the provider hostname in config."
	}

	// Rate limit — explicit numeric signal beats every WAF heuristic.
	if strings.Contains(msg, "429") || strings.Contains(msg, "rate limit") || strings.Contains(msg, "too many requests") {
		return FixHintRateLimited, "Rate limited. Wait and retry; reduce request frequency in provider config if persistent."
	}

	// TLS / certificate — handshake-level errors before we look at HTTP status.
	if strings.Contains(msg, "tls") || strings.Contains(msg, "handshake") || strings.Contains(msg, "x509") || strings.Contains(msg, "certificate") {
		return FixHintTLSTimeout, "TLS/handshake failure. Check provider TLS config and system clock skew."
	}

	// WAF — Akamai/Booking-style 403/202 challenge.
	if strings.Contains(msg, "akamai") || strings.Contains(msg, "http 403") || strings.Contains(msg, "status 403") || strings.Contains(msg, "http 202") || strings.Contains(msg, "status 202") {
		return FixHintAkamaiBlock, "WAF block detected (Akamai/Booking-style 403/202). Refresh browser cookies via test_provider, then rotate via configure_provider if needed."
	}

	// Cookie / session — runs after WAF so 403+cookie text routes to WAF first.
	if strings.Contains(msg, "cookie") || strings.Contains(msg, "session expired") || strings.Contains(msg, "csrf") || strings.Contains(msg, "401") {
		return FixHintCookieExpired, "Session/cookie expired. Re-run test_provider to refresh auth state."
	}

	// Response shape drift.
	if strings.Contains(msg, "results_path") {
		return FixHintResponseShape, "API response structure changed. Call test_provider to see current shape, then configure_provider to update results_path."
	}

	// Generic preflight failure.
	if strings.Contains(msg, "preflight") {
		return FixHintPreflightFailed, "Preflight failed. Call test_provider to inspect; auth may need refresh."
	}

	return FixHintUnclassified, "Call test_provider with this provider's id to diagnose the issue."
}
