package cars

var marketedProviderNames = []string{
	ProviderSkyscanner,
}

// MarketedProviderNames returns the user-facing rental-car provider catalog.
func MarketedProviderNames() []string {
	return append([]string(nil), marketedProviderNames...)
}

// MarketedProviderCount returns the user-facing rental-car provider count.
func MarketedProviderCount() int {
	return len(marketedProviderNames)
}
