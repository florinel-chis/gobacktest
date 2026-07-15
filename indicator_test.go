package backtest

import (
	"math"
	"testing"
)

func TestIndicatorAutoSlices(t *testing.T) {
	d := threeBar(1, 1, 1, 1) // 3 bars; n starts at fullLen=3
	ind := &Indicator{full: []float64{10, 20, 30}, data: d}
	d.setLen(2)
	if ind.Last() != 20 {
		t.Fatalf("Last()=%v want 20 at n=2", ind.Last())
	}
	if ind.At(1) != 10 {
		t.Fatalf("At(1)=%v want 10", ind.At(1))
	}
	if ind.Len() != 2 {
		t.Fatalf("Len()=%v want 2", ind.Len())
	}
	d.setLen(3)
	if ind.Last() != 30 {
		t.Fatalf("Last()=%v want 30 at n=3", ind.Last())
	}
}

func TestLeadingNaN(t *testing.T) {
	col := []float64{math.NaN(), math.NaN(), 1, 2}
	if leadingNaN(col) != 2 {
		t.Fatalf("leadingNaN=%d want 2", leadingNaN(col))
	}
}

func TestLeadingNaNEdges(t *testing.T) {
	if got := leadingNaN([]float64{math.NaN(), math.NaN()}); got != 2 {
		t.Fatalf("all-NaN leadingNaN=%d want 2", got)
	}
	if got := leadingNaN(nil); got != 0 {
		t.Fatalf("empty leadingNaN=%d want 0", got)
	}
}

func TestIndicatorAtOutOfRangePanics(t *testing.T) {
	d := threeBar(1, 1, 1, 1)
	d.setLen(2)
	ind := &Indicator{full: []float64{10, 20, 30}, data: d}
	defer func() {
		if recover() == nil {
			t.Fatal("At(2) at n=2 should panic")
		}
	}()
	ind.At(2)
}
