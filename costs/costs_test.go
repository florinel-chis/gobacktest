package costs

import (
	"math"
	"testing"
	"time"

	backtest "github.com/florinel-chis/gobacktest"
)

func TestFinancingPct(t *testing.T) {
	day := 24 * time.Hour
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	trades := []backtest.Trade{
		// 1 unit @ 1000 held 365 days at 4.68%/yr -> 46.80 cost
		{Size: 1, EntryPrice: 1000, EntryTime: t0, ExitTime: t0.Add(365 * day)},
		// 0.5 units @ 2000 held 73 days (0.2y) -> 1000 * 0.0468 * 0.2 = 9.36
		{Size: 0.5, EntryPrice: 2000, EntryTime: t0, ExitTime: t0.Add(73 * day)},
	}
	got := FinancingPct(trades, 0.0468, 10_000)
	want := (46.80 + 9.36) / 10_000 * 100
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("FinancingPct = %v, want %v", got, want)
	}
}

func TestFinancingPctShortsUseAbsNotional(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	long := []backtest.Trade{{Size: 1, EntryPrice: 1000, EntryTime: t0, ExitTime: t0.AddDate(1, 0, 0)}}
	short := []backtest.Trade{{Size: -1, EntryPrice: 1000, EntryTime: t0, ExitTime: t0.AddDate(1, 0, 0)}}
	if l, s := FinancingPct(long, 0.05, 1000), FinancingPct(short, 0.05, 1000); l != s {
		t.Errorf("long %v != short %v, want same magnitude on |size|", l, s)
	}
}

func TestFinancingPctZeroInputs(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	trades := []backtest.Trade{{Size: 1, EntryPrice: 1000, EntryTime: t0, ExitTime: t0.AddDate(0, 1, 0)}}
	if got := FinancingPct(trades, 0, 10_000); got != 0 {
		t.Errorf("zero rate: %v, want 0", got)
	}
	if got := FinancingPct(trades, 0.05, 0); got != 0 {
		t.Errorf("zero cash: %v, want 0", got)
	}
	if got := FinancingPct(nil, 0.05, 10_000); got != 0 {
		t.Errorf("no trades: %v, want 0", got)
	}
}

func TestFinancingPctSides(t *testing.T) {
	t0 := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	yr := t0.AddDate(1, 0, 0)
	trades := []backtest.Trade{
		{Size: 1, EntryPrice: 1000, EntryTime: t0, ExitTime: yr},  // long: 4.68% cost -> 46.80
		{Size: -1, EntryPrice: 1000, EntryTime: t0, ExitTime: yr}, // short: 0.32% cost -> 3.20
	}
	// Oanda-signed rates: negative = cost.
	got := FinancingPctSides(trades, -0.0468, -0.0032, 10_000)
	want := (46.80 + 3.20) / 10_000 * 100
	if math.Abs(got-want) > 1e-9 {
		t.Errorf("FinancingPctSides = %v, want %v", got, want)
	}
	// A positive (credit) rate reduces the cost.
	credit := FinancingPctSides(trades[:1], 0.01, -0.0032, 10_000)
	if credit >= 0 {
		t.Errorf("positive longRate should be a credit, got %v", credit)
	}
}
