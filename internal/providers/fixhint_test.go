package providers

// MIK-3074: classify provider failures into typed FixHintCode values.

import (
	"errors"
	"net"
	"strings"
	"testing"
)

func TestClassifyProviderError_NilReturnsUnclassified(t *testing.T) {
	code, hint := classifyProviderError(nil)
	if code != FixHintUnclassified {
		t.Errorf("nil err: code = %q, want %q", code, FixHintUnclassified)
	}
	if hint != "" {
		t.Errorf("nil err: hint = %q, want empty", hint)
	}
}

func TestClassifyProviderError_Buckets(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want FixHintCode
	}{
		{"dns_no_such_host", errors.New("dial: lookup foo.invalid: no such host"), FixHintDNSFail},
		{"dns_typed", &net.DNSError{Err: "server misbehaving", Name: "foo.invalid"}, FixHintDNSFail},
		{"rate_limit_429", errors.New("HTTP 429 too many requests"), FixHintRateLimited},
		{"rate_limit_text", errors.New("provider returned rate limit error"), FixHintRateLimited},
		{"tls_handshake", errors.New("tls: handshake failure"), FixHintTLSTimeout},
		{"tls_x509", errors.New("x509: certificate signed by an unrecognised authority"), FixHintTLSTimeout},
		{"akamai_403", errors.New("preflight http 403 (akamai)"), FixHintAkamaiBlock},
		{"booking_202", errors.New("preflight: status 202 challenge"), FixHintAkamaiBlock},
		{"cookie_expired", errors.New("csrf token mismatch"), FixHintCookieExpired},
		{"unauthorized_401", errors.New("HTTP 401 unauthorized"), FixHintCookieExpired},
		{"results_path_drift", errors.New("results_path 'data.hotels' not found"), FixHintResponseShape},
		{"preflight_generic", errors.New("preflight read: EOF"), FixHintPreflightFailed},
		{"unclassified_misc", errors.New("something else entirely"), FixHintUnclassified},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			code, hint := classifyProviderError(tc.err)
			if code != tc.want {
				t.Errorf("err=%q: code = %q, want %q", tc.err.Error(), code, tc.want)
			}
			if hint == "" {
				t.Errorf("err=%q: empty hint for code %q", tc.err.Error(), code)
			}
		})
	}
}

// TestClassifyProviderError_OrderingPriority verifies that more-specific
// signals win over broader ones — e.g. "rate limit" inside an error string
// that also contains "403" should still classify as RATE_LIMITED, not
// AKAMAI_BLOCK. This guards against silent regressions if someone reorders
// the cases in classifyProviderError.
func TestClassifyProviderError_OrderingPriority(t *testing.T) {
	rateInsideWAF := errors.New("HTTP 403 returned: rate limit exceeded")
	if code, _ := classifyProviderError(rateInsideWAF); code != FixHintRateLimited {
		t.Errorf("ambiguous err with both 403 + rate-limit: got %q, want %q", code, FixHintRateLimited)
	}

	wafInsideCookie := errors.New("HTTP 403 forbidden (cookie required)")
	if code, _ := classifyProviderError(wafInsideCookie); code != FixHintAkamaiBlock {
		t.Errorf("ambiguous err with 403 + cookie: got %q, want %q (WAF should win over cookie)", code, FixHintAkamaiBlock)
	}
}

// TestProviderFixHint_DelegatesToClassifier ensures the back-compat wrapper
// returns the same human text the classifier produces, so existing callers
// that only need the string keep working unchanged (MIK-3074).
func TestProviderFixHint_DelegatesToClassifier(t *testing.T) {
	err := errors.New("preflight http 403 akamai")
	got := providerFixHint(err)
	_, want := classifyProviderError(err)
	if got != want {
		t.Errorf("providerFixHint = %q, want %q", got, want)
	}
	if !strings.Contains(got, "WAF") {
		t.Errorf("providerFixHint for WAF err did not mention WAF: %q", got)
	}
}
