package main

import (
	"fmt"
	"sort"
)

// printMatchBreakdown prints an ASCII bar-chart per-factor breakdown for a
// single result's profile match score. label identifies the result (e.g. "#1 KLM").
// score is 0-100; breakdown maps factor names to per-factor scores in [0,1].
func printMatchBreakdown(label string, score int, breakdown map[string]float64) {
	if len(breakdown) == 0 {
		return
	}
	fmt.Printf("  %s — profile match breakdown (%d%%):\n", label, score)
	factors := make([]string, 0, len(breakdown))
	for k := range breakdown {
		factors = append(factors, k)
	}
	sort.Strings(factors)
	for _, factor := range factors {
		bar := progressBar(breakdown[factor], 20)
		fmt.Printf("    %-35s %s  %.0f%%\n", factor, bar, breakdown[factor]*100)
	}
	fmt.Println()
}
