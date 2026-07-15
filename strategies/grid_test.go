package strategies_test

import (
	"math"
	"sort"
	"testing"
	"time"

	backtest "github.com/florinel-chis/gobacktest"
	"github.com/florinel-chis/gobacktest/strategies"
)

// gridData: a plateau (warms up %R high), a sharp multi-bar decline that
// triggers the first oversold entry and then two grid adds, then a rebound
// past all take-profits.
func gridData(t *testing.T) *backtest.Data {
	t.Helper()
	closes := []float64{100, 100, 100, 100, 99.2, 98.4, 97.6, 96.8, 96.0, 95.2, 96.5, 98, 100, 102, 104}
	n := len(closes)
	times := make([]time.Time, n)
	open := make([]float64, n)
	high := make([]float64, n)
	low := make([]float64, n)
	vol := make([]float64, n)
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, c := range closes {
		times[i] = base.Add(time.Duration(i) * time.Hour)
		open[i] = c
		high[i] = c + 0.3
		low[i] = c - 0.3
		vol[i] = 1000
	}
	d, err := backtest.FromOHLCV(times, open, high, low, closes, vol)
	if err != nil {
		t.Fatalf("FromOHLCV: %v", err)
	}
	return d
}

func runGrid(t *testing.T, g *strategies.AveragingGrid) backtest.Result {
	t.Helper()
	res, err := backtest.New(gridData(t), g, backtest.Options{Cash: 100_000, Margin: 1, FinalizeTrades: true}).Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return *res
}

func TestGridGrowsSizePerLevel(t *testing.T) {
	res := runGrid(t, &strategies.AveragingGrid{
		WRPeriod: 3, WRThreshold: -70, SpacingPct: 0.7, TPPct: 1, MaxLevels: 4, BaseSize: 10, Growing: true,
	})
	if len(res.Trades) < 3 {
		t.Fatalf("got %d trades, want >= 3 (first entry + grid adds)", len(res.Trades))
	}
	trades := append([]backtest.Trade(nil), res.Trades...)
	sort.Slice(trades, func(i, j int) bool { return trades[i].EntryBar < trades[j].EntryBar })
	for i, tr := range trades {
		want := 10 * float64(i+1)
		if math.Abs(tr.Size-want) > 1e-9 {
			t.Errorf("lot %d size = %v, want %v (growing progression)", i+1, tr.Size, want)
		}
		if i > 0 && tr.EntryPrice >= trades[i-1].EntryPrice {
			t.Errorf("lot %d entry %v not below lot %d entry %v", i+1, tr.EntryPrice, i, trades[i-1].EntryPrice)
		}
		if tr.PL <= 0 {
			t.Errorf("lot %d PL = %v, want > 0 (rebound passes all TPs)", i+1, tr.PL)
		}
	}
	// The last (largest) lot must have the largest absolute profit.
	last := trades[len(trades)-1]
	for _, tr := range trades[:len(trades)-1] {
		if last.PL <= tr.PL {
			t.Errorf("last lot PL %v not larger than earlier lot PL %v", last.PL, tr.PL)
		}
	}
}

func TestGridFlatSizing(t *testing.T) {
	res := runGrid(t, &strategies.AveragingGrid{
		WRPeriod: 3, WRThreshold: -70, SpacingPct: 0.7, TPPct: 1, MaxLevels: 4, BaseSize: 10, Growing: false,
	})
	for _, tr := range res.Trades {
		if math.Abs(tr.Size-10) > 1e-9 {
			t.Errorf("flat grid: size = %v, want 10", tr.Size)
		}
	}
}

func TestGridRespectsMaxLevels(t *testing.T) {
	res := runGrid(t, &strategies.AveragingGrid{
		WRPeriod: 3, WRThreshold: -70, SpacingPct: 0.3, TPPct: 5, MaxLevels: 2, BaseSize: 10, Growing: true,
	})
	// With tiny spacing and a far TP the decline would add many lots; the cap
	// must hold the count at 2.
	maxOpen := 0
	type ev struct {
		bar   int
		delta int
	}
	var evs []ev
	for _, tr := range res.Trades {
		evs = append(evs, ev{tr.EntryBar, 1}, ev{tr.ExitBar, -1})
	}
	sort.Slice(evs, func(i, j int) bool {
		if evs[i].bar != evs[j].bar {
			return evs[i].bar < evs[j].bar
		}
		return evs[i].delta < evs[j].delta // exits before entries on same bar
	})
	open := 0
	for _, e := range evs {
		open += e.delta
		if open > maxOpen {
			maxOpen = open
		}
	}
	if maxOpen > 2 {
		t.Errorf("max concurrent lots = %d, want <= 2", maxOpen)
	}
}

func TestGridNoEntryWithoutOversold(t *testing.T) {
	res := runGrid(t, &strategies.AveragingGrid{
		WRPeriod: 3, WRThreshold: -99.9, SpacingPct: 0.7, TPPct: 1, MaxLevels: 4, BaseSize: 10, Growing: true,
	})
	if len(res.Trades) != 0 {
		t.Errorf("strict threshold: got %d trades, want 0", len(res.Trades))
	}
}
