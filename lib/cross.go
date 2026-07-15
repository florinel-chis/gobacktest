// Package lib provides strategy helpers (crossovers) over indicator series.
package lib

import "math"

func last2ok(s []float64) bool {
	n := len(s)
	return n >= 2 && !math.IsNaN(s[n-1]) && !math.IsNaN(s[n-2])
}

// Crossover reports whether a crossed above b on the most recent bar.
// a and b must be time-aligned (equal length); otherwise returns false.
func Crossover(a, b []float64) bool {
	if len(a) != len(b) || !last2ok(a) || !last2ok(b) {
		return false
	}
	na, nb := len(a), len(b)
	return a[na-2] < b[nb-2] && a[na-1] > b[nb-1]
}

// CrossUnder reports whether a crossed below b on the most recent bar.
// a and b must be time-aligned (equal length); otherwise returns false.
func CrossUnder(a, b []float64) bool {
	if len(a) != len(b) || !last2ok(a) || !last2ok(b) {
		return false
	}
	na, nb := len(a), len(b)
	return a[na-2] > b[nb-2] && a[na-1] < b[nb-1]
}
