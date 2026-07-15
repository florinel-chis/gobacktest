package lib

import (
	"math"
	"testing"
)

func TestCrossover(t *testing.T) {
	// a crosses ABOVE b: prev 1<2, now 3>2
	a := []float64{1, 1, 3}
	b := []float64{2, 2, 2}
	if !Crossover(a, b) {
		t.Fatal("expected crossover")
	}
	if CrossUnder(a, b) {
		t.Fatal("not a cross-under")
	}
	// no cross when touching at prev bar (equal) then above: a[n-2]==b[n-2] -> strict < fails -> no cross
	if Crossover([]float64{2, 3}, []float64{2, 2}) {
		t.Fatal("equal prev values must not count as crossover (strict <)")
	}
	// NaN guard on last two of either series
	if Crossover([]float64{math.NaN(), 3}, []float64{2, 2}) {
		t.Fatal("NaN must yield false")
	}
	// too-short slice
	if Crossover([]float64{3}, []float64{2}) {
		t.Fatal("len<2 must yield false")
	}
	// unequal-length series
	if Crossover([]float64{1, 2, 3}, []float64{1, 2}) {
		t.Fatal("unequal-length series must yield false")
	}
}

func TestCrossUnder(t *testing.T) {
	// a crosses BELOW b: prev 3>2, now 1<2
	a := []float64{3, 3, 1}
	b := []float64{2, 2, 2}
	if !CrossUnder(a, b) {
		t.Fatal("expected cross-under")
	}
	if Crossover(a, b) {
		t.Fatal("not a crossover")
	}
	if CrossUnder([]float64{1, math.NaN()}, []float64{2, 2}) {
		t.Fatal("NaN must yield false")
	}
}
