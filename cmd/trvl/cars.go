package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/cars"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/spf13/cobra"
)

func carsCmd() *cobra.Command {
	var (
		dropoffLocation string
		pickupTime      string
		dropoffTime     string
		currency        string
		provider        string
		passengers      int
		driverAge       int
		maxPrice        float64
		vehicleClass    string
	)

	cmd := &cobra.Command{
		Use:     "cars PICKUP_LOCATION PICKUP_DATE DROPOFF_DATE",
		Aliases: []string{"car", "rentals"},
		Short:   "Search rental car offers",
		Long: `Search rental car offers for a pickup location and date range.

Rental car APIs are commonly partner-gated. By default trvl tries the
Skyscanner Car Hire provider when SKYSCANNER_API_KEY is configured and returns
a typed provider setup status when credentials are missing.

Examples:
  trvl cars HEL 2026-07-01 2026-07-04
  trvl cars "Helsinki Airport" 2026-07-01 2026-07-04 --currency EUR
  trvl cars Prague 2026-07-01 2026-07-04 --dropoff-location Vienna --vehicle-class compact`,
		Args: cobra.ExactArgs(3),
		RunE: func(cmd *cobra.Command, args []string) error {
			opts := cars.SearchOptions{
				PickupLocation:  args[0],
				DropoffLocation: dropoffLocation,
				PickupDate:      args[1],
				DropoffDate:     args[2],
				PickupTime:      pickupTime,
				DropoffTime:     dropoffTime,
				Currency:        currency,
				Passengers:      passengers,
				DriverAge:       driverAge,
				MaxPrice:        maxPrice,
				VehicleClass:    vehicleClass,
			}
			if provider != "" {
				opts.Providers = strings.Split(provider, ",")
			}

			result, err := cars.Search(cmd.Context(), opts)
			if err != nil {
				return err
			}
			if format == "json" {
				return models.FormatJSON(os.Stdout, result)
			}
			printCarsTable(result)
			if openFlag && result.Success && len(result.Offers) > 0 && result.Offers[0].BookingURL != "" {
				_ = openBrowser(result.Offers[0].BookingURL)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&dropoffLocation, "dropoff-location", "", "Dropoff location (defaults to pickup location)")
	cmd.Flags().StringVar(&pickupTime, "pickup-time", "", "Pickup local time HH:MM (default 10:00)")
	cmd.Flags().StringVar(&dropoffTime, "dropoff-time", "", "Dropoff local time HH:MM (default 10:00)")
	cmd.Flags().StringVar(&currency, "currency", "", "Price currency (default EUR)")
	cmd.Flags().StringVar(&provider, "provider", "", "Restrict to provider, currently skyscanner")
	cmd.Flags().IntVar(&passengers, "passengers", 0, "Traveller count for capacity recommendations")
	cmd.Flags().IntVar(&driverAge, "driver-age", 0, "Driver age for age-sensitive offers")
	cmd.Flags().Float64Var(&maxPrice, "max-price", 0, "Maximum total rental price")
	cmd.Flags().StringVar(&vehicleClass, "vehicle-class", "", "Vehicle class filter (economy, compact, SUV, van)")
	return cmd
}

func printCarsTable(result *models.CarSearchResult) {
	if result == nil || !result.Success {
		if result != nil && result.Error != "" {
			_, _ = fmt.Fprintf(os.Stderr, "No rental car offers found: %s\n", result.Error)
		} else {
			_, _ = fmt.Fprintln(os.Stderr, "No rental car offers found.")
		}
		return
	}

	models.Banner(os.Stdout, "🚗", "Rental Cars", fmt.Sprintf("Found %d offers", result.Count))
	fmt.Println()
	headers := []string{"Price", "Supplier", "Vehicle", "Class", "Seats", "Pickup", "Provider"}
	rows := make([][]string, 0, len(result.Offers))
	for _, offer := range result.Offers {
		price := "-"
		if offer.Price > 0 {
			price = fmt.Sprintf("%s %.2f", offer.Currency, offer.Price)
		}
		rows = append(rows, []string{
			models.Green(price),
			offer.Supplier,
			offer.VehicleName,
			offer.VehicleClass,
			intString(offer.Seats),
			offer.Pickup.Location,
			offer.Provider,
		})
	}
	models.FormatTable(os.Stdout, headers, rows)
}

func intString(v int) string {
	if v == 0 {
		return "-"
	}
	return fmt.Sprintf("%d", v)
}
