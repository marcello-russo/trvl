package models

// CarSearchResult holds the results from searching rental car providers.
type CarSearchResult struct {
	Success          bool             `json:"success"`
	Count            int              `json:"count"`
	Offers           []CarOffer       `json:"offers"`
	ProviderStatuses []ProviderStatus `json:"provider_statuses,omitempty"`
	Error            string           `json:"error,omitempty"`
}

// CarOffer represents one rental car offer.
type CarOffer struct {
	Provider         string      `json:"provider"`
	Supplier         string      `json:"supplier,omitempty"`
	VehicleClass     string      `json:"vehicle_class,omitempty"`
	VehicleName      string      `json:"vehicle_name,omitempty"`
	Transmission     string      `json:"transmission,omitempty"`
	FuelPolicy       string      `json:"fuel_policy,omitempty"`
	Seats            int         `json:"seats,omitempty"`
	Bags             int         `json:"bags,omitempty"`
	Doors            int         `json:"doors,omitempty"`
	Passengers       int         `json:"passengers,omitempty"`
	Pickup           CarEndpoint `json:"pickup"`
	Dropoff          CarEndpoint `json:"dropoff"`
	Price            float64     `json:"price"`
	Currency         string      `json:"currency"`
	TaxesAndFees     float64     `json:"taxes_and_fees,omitempty"`
	FreeCancellation *bool       `json:"free_cancellation,omitempty"`
	UnlimitedMileage *bool       `json:"unlimited_mileage,omitempty"`
	BookingURL       string      `json:"booking_url,omitempty"`
	Freshness        string      `json:"freshness,omitempty"`
}

// CarEndpoint describes a pickup or dropoff point and time.
type CarEndpoint struct {
	Location string  `json:"location"`
	Code     string  `json:"code,omitempty"`
	Time     string  `json:"time,omitempty"`
	Address  string  `json:"address,omitempty"`
	Lat      float64 `json:"lat,omitempty"`
	Lon      float64 `json:"lon,omitempty"`
}
