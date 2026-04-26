package providers

import (
	"os"
	"strconv"
)

// providerConcurrencyEnv is the environment variable that overrides the
// maximum number of provider goroutines that may run concurrently.
const providerConcurrencyEnv = "TRVL_PROVIDER_CONCURRENCY"

// defaultProviderConcurrency is the cap used when the env var is absent or
// invalid.
const defaultProviderConcurrency = 8

// providerConcurrency returns the configured per-runtime goroutine cap.
// It reads TRVL_PROVIDER_CONCURRENCY; on parse failure or a non-positive
// value it falls back to defaultProviderConcurrency.
func providerConcurrency() int {
	s := os.Getenv(providerConcurrencyEnv)
	if s == "" {
		return defaultProviderConcurrency
	}
	n, err := strconv.Atoi(s)
	if err != nil || n <= 0 {
		return defaultProviderConcurrency
	}
	return n
}
