package strategies_test

import (
	"testing"
	"time"

	backtest "github.com/florinel-chis/gobacktest"
	"github.com/florinel-chis/gobacktest/strategies"
)

// synthData builds a deterministic OHLCV series whose Williams %R(3) drops
// into oversold (near -100 at the trough) then rebounds strongly, so a loose
// threshold trades profitably and a strict one never triggers.
func synthData(t *testing.T) *backtest.Data {
	t.Helper()
	closes := []float64{100, 100, 100, 92, 90, 95, 100, 104, 108}
	n := len(closes)
	times := make([]time.Time, n)
	open := make([]float64, n)
	high := make([]float64, n)
	low := make([]float64, n)
	vol := make([]float64, n)
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	for i, c := range closes {
		times[i] = base.AddDate(0, 0, i)
		open[i] = c
		high[i] = c + 1
		low[i] = c - 1
		vol[i] = 1000
	}
	d, err := backtest.FromOHLCV(times, open, high, low, closes, vol)
	if err != nil {
		t.Fatalf("FromOHLCV: %v", err)
	}
	return d
}

func run(t *testing.T, d *backtest.Data, s *strategies.WilliamsROversold) backtest.Stats {
	t.Helper()
	res, err := backtest.New(d, s, backtest.Options{Cash: 10_000, Margin: 1, FinalizeTrades: true}).Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return backtest.Compute(res, d, 0)
}

func TestLooseThresholdTrades(t *testing.T) {
	st := run(t, synthData(t), &strategies.WilliamsROversold{
		WRPeriod: 3, EMAPeriod: 2, WRThreshold: -70, EMAThreshold: -70,
		UseEMA: true, TPPct: 10,
	})
	if st.NumTrades == 0 {
		t.Fatal("loose threshold: want at least one trade")
	}
	if st.ReturnPct <= 0 {
		t.Errorf("Return%% = %v, want > 0 on the rebound path", st.ReturnPct)
	}
}

func TestStrictThresholdNeverTrades(t *testing.T) {
	st := run(t, synthData(t), &strategies.WilliamsROversold{
		WRPeriod: 3, EMAPeriod: 2, WRThreshold: -99.9, EMAThreshold: -99.9,
		UseEMA: true, TPPct: 10,
	})
	if st.NumTrades != 0 {
		t.Errorf("strict threshold: got %d trades, want 0", st.NumTrades)
	}
}

func TestWROnlyVariantTradesMore(t *testing.T) {
	d1, d2 := synthData(t), synthData(t)
	wrOnly := run(t, d1, &strategies.WilliamsROversold{WRPeriod: 3, WRThreshold: -70, TPPct: 10})
	withEMA := run(t, d2, &strategies.WilliamsROversold{
		WRPeriod: 3, EMAPeriod: 2, WRThreshold: -70, EMAThreshold: -95, UseEMA: true, TPPct: 10,
	})
	if wrOnly.NumTrades < withEMA.NumTrades {
		t.Errorf("WR-only trades (%d) < stricter EMA-filtered trades (%d)", wrOnly.NumTrades, withEMA.NumTrades)
	}
}

func TestAbsoluteTPOverridesPct(t *testing.T) {
	// TPAbs 2 arms a much nearer target than TPPct 10 on a ~90 entry, so the
	// same rebound path must exit earlier (smaller best trade) when both are
	// set. (Exact fill prices gap through the TP at the next open, so compare
	// the two runs rather than asserting the raw TP distance.)
	abs := run(t, synthData(t), &strategies.WilliamsROversold{
		WRPeriod: 3, WRThreshold: -70, TPPct: 10, TPAbs: 2,
	})
	pct := run(t, synthData(t), &strategies.WilliamsROversold{
		WRPeriod: 3, WRThreshold: -70, TPPct: 10,
	})
	if abs.NumTrades == 0 || pct.NumTrades == 0 {
		t.Fatal("want at least one trade in both runs")
	}
	if abs.BestTradePct >= pct.BestTradePct {
		t.Errorf("TPAbs best trade %v%% >= TPPct best trade %v%%, want earlier exit", abs.BestTradePct, pct.BestTradePct)
	}
}

func TestDefaultsApplied(t *testing.T) {
	// Zero-value periods/thresholds resolve to 21/12 and -80: on a series
	// shorter than the warmup the strategy must simply never trade (and,
	// critically, never panic on NaN).
	st := run(t, synthData(t), &strategies.WilliamsROversold{UseEMA: true, TPPct: 10})
	if st.NumTrades != 0 {
		t.Errorf("9 bars < WR(21)+EMA(12) warmup: got %d trades, want 0", st.NumTrades)
	}
}
