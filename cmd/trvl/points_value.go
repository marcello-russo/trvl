package main

import (
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/MikkoParkkola/trvl/internal/hotelarb"
	"github.com/MikkoParkkola/trvl/internal/models"
	"github.com/MikkoParkkola/trvl/internal/points"
	"github.com/spf13/cobra"
)

func pointsValueCmd() *cobra.Command {
	var (
		cashPrice      float64
		pointsRequired int
		program        string
		listPrograms   bool
		format         string
		offers         []string
		currency       string
	)

	cmd := &cobra.Command{
		Use:   "points-value",
		Short: "Calculate whether using points or paying cash is better",
		Long: `Compare the value of redeeming loyalty points against paying cash.

Computes the effective cents-per-point (cpp) for a specific redemption and
compares it against the published floor and ceiling values for the program.

Examples:
  trvl points-value --cash 450 --points 60000 --program finnair-plus
  trvl points-value --cash 1200 --points 50000 --program ana-mileage-club
  trvl points-value --cash 300 --points 30000 --program world-of-hyatt --format json
  trvl points-value --cash 300 --offer world-of-hyatt:12000 --offer hilton-honors:80000
  trvl points-value --list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if listPrograms {
				return printProgramList(format)
			}
			if len(offers) > 0 {
				if cashPrice <= 0 {
					return fmt.Errorf("--cash must be greater than 0")
				}
				parsedOffers, err := parsePointsOffers(offers)
				if err != nil {
					return err
				}
				result, err := hotelarb.ComparePointsArbitrage(hotelarb.PointsArbitrageInput{
					CashPrice: cashPrice,
					Currency:  currency,
					Offers:    parsedOffers,
				})
				if err != nil {
					return err
				}
				if format == "json" {
					return models.FormatJSON(os.Stdout, result)
				}
				printPointsArbitrage(result)
				return nil
			}

			if program == "" {
				return fmt.Errorf("--program is required (use --list to see options)")
			}
			if cashPrice <= 0 {
				return fmt.Errorf("--cash must be greater than 0")
			}
			if pointsRequired <= 0 {
				return fmt.Errorf("--points must be greater than 0")
			}

			rec, err := points.CalculateValue(cashPrice, pointsRequired, program)
			if err != nil {
				return err
			}

			if format == "json" {
				return models.FormatJSON(os.Stdout, rec)
			}

			printRecommendation(rec)
			return nil
		},
	}

	cmd.Flags().Float64Var(&cashPrice, "cash", 0, "Cash price of the flight/hotel (e.g. 450.00)")
	cmd.Flags().IntVar(&pointsRequired, "points", 0, "Points required for the redemption (e.g. 60000)")
	cmd.Flags().StringVar(&program, "program", "", "Loyalty program slug (e.g. finnair-plus)")
	cmd.Flags().BoolVar(&listPrograms, "list", false, "List all supported programs and their valuations")
	cmd.Flags().StringVar(&format, "format", "table", "Output format: table, json")
	cmd.Flags().StringArrayVar(&offers, "offer", nil, "Compare hotel points offer as program:points[:cash_fees]; repeat for multiple programs")
	cmd.Flags().StringVar(&currency, "currency", "USD", "Currency label for hotel points arbitrage output")

	return cmd
}

func parsePointsOffers(raw []string) ([]hotelarb.PointsOffer, error) {
	offers := make([]hotelarb.PointsOffer, 0, len(raw))
	for _, item := range raw {
		parts := strings.Split(item, ":")
		if len(parts) < 2 || len(parts) > 3 {
			return nil, fmt.Errorf("offer %q must use program:points[:cash_fees]", item)
		}
		pointsRequired, err := parsePositiveInt(parts[1])
		if err != nil {
			return nil, fmt.Errorf("offer %q points: %w", item, err)
		}
		offer := hotelarb.PointsOffer{
			Program:        parts[0],
			PointsRequired: pointsRequired,
		}
		if len(parts) == 3 {
			fees, err := parseNonNegativeFloat(parts[2])
			if err != nil {
				return nil, fmt.Errorf("offer %q cash fees: %w", item, err)
			}
			offer.CashFees = fees
		}
		offers = append(offers, offer)
	}
	return offers, nil
}

func parsePositiveInt(raw string) (int, error) {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, err
	}
	if value <= 0 {
		return 0, fmt.Errorf("must be greater than 0")
	}
	return value, nil
}

func parseNonNegativeFloat(raw string) (float64, error) {
	value, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	if value < 0 {
		return 0, fmt.Errorf("must be non-negative")
	}
	return value, nil
}

func printPointsArbitrage(result *hotelarb.PointsArbitrageResult) {
	models.Banner(os.Stdout, "✦", "Hotel Points Arbitrage",
		fmt.Sprintf("Recommendation: %s", strings.ReplaceAll(string(result.Recommendation), "_", " ")),
	)
	fmt.Println()

	headers := []string{"Program", "Points", "CPP", "Floor", "Cost", "Value"}
	rows := make([][]string, 0, len(result.Offers))
	for _, offer := range result.Offers {
		rows = append(rows, []string{
			offer.ProgramName,
			formatPoints(offer.PointsRequired),
			fmt.Sprintf("%.2f¢", offer.CentsPerPoint),
			fmt.Sprintf("%.2f¢", offer.FloorCPP),
			fmt.Sprintf("%.0f %s", offer.OpportunityCost, result.Currency),
			fmt.Sprintf("%.0f %s", offer.SavingsVsCash, result.Currency),
		})
	}
	models.FormatTable(os.Stdout, headers, rows)
	fmt.Println()
	models.Summary(os.Stdout, result.Reason)
}

// printRecommendation renders a single recommendation as a pretty table.
func printRecommendation(r *points.Recommendation) {
	verdictColored := colorizeVerdict(r.Verdict)

	models.Banner(os.Stdout, "✦", "Points vs Cash",
		fmt.Sprintf("Program: %s", r.ProgramName),
	)
	fmt.Println()

	headers := []string{"Metric", "Value"}
	rows := [][]string{
		{"Cash price", fmt.Sprintf("$%.2f", r.CashPrice)},
		{"Points required", formatPoints(r.PointsRequired)},
		{"Effective CPP", fmt.Sprintf("%.2f¢/pt", r.CPP)},
		{"Floor CPP", fmt.Sprintf("%.2f¢/pt", r.FloorCPP)},
		{"Ceiling CPP (sweet spot)", fmt.Sprintf("%.2f¢/pt", r.CeilingCPP)},
		{"Verdict", verdictColored},
	}
	models.FormatTable(os.Stdout, headers, rows)

	fmt.Println()
	models.Summary(os.Stdout, r.Explanation)
}

// printProgramList prints all supported programs grouped by category.
func printProgramList(format string) error {
	if format == "json" {
		return models.FormatJSON(os.Stdout, points.Programs)
	}

	// Group by category.
	grouped := map[string][]points.Program{}
	for _, p := range points.Programs {
		grouped[p.Category] = append(grouped[p.Category], p)
	}

	categories := []string{"airline", "hotel", "transferable"}
	categoryLabels := map[string]string{
		"airline":      "Airline Programs",
		"hotel":        "Hotel Programs",
		"transferable": "Transferable Currencies",
	}

	for _, cat := range categories {
		progs, ok := grouped[cat]
		if !ok || len(progs) == 0 {
			continue
		}
		sort.Slice(progs, func(i, j int) bool { return progs[i].Slug < progs[j].Slug })

		label := categoryLabels[cat]
		models.Banner(os.Stdout, "✦", label)
		fmt.Println()

		headers := []string{"Slug", "Name", "Floor CPP", "Ceiling CPP"}
		var rows [][]string
		for _, p := range progs {
			rows = append(rows, []string{
				p.Slug,
				p.Name,
				fmt.Sprintf("%.2f¢", p.FloorCPP),
				fmt.Sprintf("%.2f¢", p.CeilingCPP),
			})
		}
		models.FormatTable(os.Stdout, headers, rows)
		fmt.Println()
	}

	fmt.Printf("Use --program <slug> to calculate a specific redemption.\n")
	return nil
}

// formatPoints adds thousands separators to a point count.
func formatPoints(n int) string {
	s := fmt.Sprintf("%d", n)
	if len(s) <= 3 {
		return s
	}
	var b strings.Builder
	start := len(s) % 3
	if start == 0 {
		start = 3
	}
	b.WriteString(s[:start])
	for i := start; i < len(s); i += 3 {
		b.WriteByte(',')
		b.WriteString(s[i : i+3])
	}
	return b.String()
}

// colorizeVerdict applies terminal color to the verdict string.
func colorizeVerdict(verdict string) string {
	switch verdict {
	case "use points":
		return models.Green(verdict)
	case "pay cash":
		return models.Red(verdict)
	default:
		return models.Yellow(verdict)
	}
}
