package providers

import (
	"errors"
	"testing"
)

func TestClassifyProviderError_AllCodes(t *testing.T) {
	cases := []struct {
		name     string
		errMsg   string
		wantCode FixHintCode
	}{
		// Rate-limit beats WAF
		{"rate_limit_text", "upstream: rate limit exceeded", FixHintRateLimited},
		{"http_429", "server returned http 429 too many requests", FixHintRateLimited},
		{"too_many_requests", "too many requests from this ip", FixHintRateLimited},

		// Akamai / WAF block
		{"akamai_explicit", "akamai edge server rejected request", FixHintAkamaiBlock},
		{"http_403", "http 403 forbidden", FixHintAkamaiBlock},
		{"http_202", "http 202 challenge page", FixHintAkamaiBlock},
		{"access_denied", "access denied by provider waf", FixHintAkamaiBlock},

		// Cookie / auth failure
		{"cookie_expired", "cookie jar expired, re-login required", FixHintCookieExpired},
		{"csrf_mismatch", "csrf token mismatch on post", FixHintCookieExpired},
		{"http_401", "server returned 401 unauthorized", FixHintCookieExpired},

		// Preflight step failure
		{"preflight_fail", "preflight request to /auth failed", FixHintPreflightFailed},

		// Response shape changed
		{"results_path", "results_path '/data/hotels' not found in response", FixHintResponseShapeChanged},
		{"unmarshal", "cannot unmarshal string into Go value of type []Hotel", FixHintResponseShapeChanged},
		{"unexpected_eof", "unexpected end of json input", FixHintResponseShapeChanged},

		// DNS failure
		{"no_such_host", "dial tcp: lookup booking.com: no such host", FixHintDNSFail},
		{"dns_explicit", "dns resolution failed for provider endpoint", FixHintDNSFail},

		// TLS / connection timeout
		{"tls_handshake", "tls handshake timeout after 10s", FixHintTLSTimeout},
		{"deadline_exceeded", "context deadline exceeded", FixHintTLSTimeout},
		{"timeout_generic", "connection timeout to provider", FixHintTLSTimeout},
		{"connection_refused", "connection refused on port 443", FixHintTLSTimeout},

		// Unclassified
		{"unknown", "something completely unrecognised happened", FixHintUnclassified},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, hint := classifyProviderError(errors.New(tc.errMsg))
			if code != tc.wantCode {
				t.Errorf("classifyProviderError(%q) code = %q, want %q", tc.errMsg, code, tc.wantCode)
			}
			if hint == "" {
				t.Errorf("classifyProviderError(%q) returned empty hint", tc.errMsg)
			}
		})
	}
}

// TestClassifyProviderError_NilError verifies graceful handling of nil.
func TestClassifyProviderError_NilError(t *testing.T) {
	code, hint := classifyProviderError(nil)
	if code != FixHintUnclassified {
		t.Errorf("nil error should classify as UNCLASSIFIED, got %q", code)
	}
	if hint == "" {
		t.Error("nil error should still return a non-empty hint")
	}
}

// TestClassifyProviderError_OrderingPriority pins tie-breaker ordering:
// rate-limit > WAF > cookie.
func TestClassifyProviderError_OrderingPriority(t *testing.T) {
	// A 429 that also contains "403" in the body — rate-limit wins.
	code, _ := classifyProviderError(errors.New("http 429, previously saw http 403"))
	if code != FixHintRateLimited {
		t.Errorf("rate-limit should beat WAF, got %q", code)
	}

	// A 403 that also mentions "cookie" — WAF wins (403 is checked before cookie).
	code, _ = classifyProviderError(errors.New("http 403, cookie header was present"))
	if code != FixHintAkamaiBlock {
		t.Errorf("WAF (403) should beat cookie when both match, got %q", code)
	}
}

// TestProviderFixHint_BackCompat verifies that providerFixHint (the legacy
// wrapper) returns a non-empty string for every error class.
func TestProviderFixHint_BackCompat(t *testing.T) {
	errs := []string{
		"rate limit exceeded",
		"http 403 forbidden",
		"cookie expired",
		"preflight failed",
		"results_path not found",
		"tls handshake timeout",
		"context deadline exceeded",
		"no such host",
		"unrecognised error",
	}
	for _, msg := range errs {
		hint := providerFixHint(errors.New(msg))
		if hint == "" {
			t.Errorf("providerFixHint(%q) returned empty hint", msg)
		}
	}
}
