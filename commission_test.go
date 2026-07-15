package backtest

import (
	"math"
	"testing"
)

func TestCommission(t *testing.T) {
	if got := Pct(0.002).cost(10, 5); math.Abs(got-0.1) > 1e-12 { // |10|*5*0.002
		t.Fatalf("Pct cost=%v want 0.1", got)
	}
	if got := FixedPlusPct(1, 0.001).cost(-10, 5); math.Abs(got-1.05) > 1e-12 { // 1 + 10*5*0.001
		t.Fatalf("FixedPlusPct cost=%v want 1.05", got)
	}
	var c Commission = CommissionFunc(func(size, price float64) float64 { return 7 })
	if c.cost(1, 1) != 7 {
		t.Fatalf("CommissionFunc cost=%v want 7", c.cost(1, 1))
	}
}

func TestAdjustedPrice(t *testing.T) {
	if got := adjustedPrice(0.01, 5, 100); math.Abs(got-101) > 1e-9 { // buy: +1%
		t.Fatalf("buy adjusted=%v want 101", got)
	}
	if got := adjustedPrice(0.01, -5, 100); math.Abs(got-99) > 1e-9 { // sell: -1%
		t.Fatalf("sell adjusted=%v want 99", got)
	}
}
