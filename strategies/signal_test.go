package strategies_test

import (
	"testing"
	"time"

	backtest "github.com/florinel-chis/gobacktest"
	"github.com/florinel-chis/gobacktest/strategies"
)

func mkData(prices []float64) *backtest.Data {
	n := len(prices)
	ts := make([]time.Time, n)
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)
	v := make([]float64, n)
	base := time.Unix(0, 0)
	for i, p := range prices {
		ts[i] = base.Add(time.Duration(i) * time.Hour)
		o[i], h[i], l[i], c[i], v[i] = p, p, p, p, 100
	}
	d, err := backtest.FromOHLCV(ts, o, h, l, c, v)
	if err != nil {
		panic(err)
	}
	return d
}

func runSignal(prices, sig []float64, hold int) backtest.Stats {
	d := mkData(prices)
	bt := backtest.New(d, &strategies.SignalStrategy{Signal: sig, Hold: hold, Size: 0.95},
		backtest.Options{Cash: 10000, Margin: 1, FinalizeTrades: true})
	res, err := bt.Run()
	if err != nil {
		panic(err)
	}
	return backtest.Compute(res, d, 0.0)
}

// A single positive signal on a dip that recovers opens exactly one long, holds
// Hold bars, and profits — proving entry timing (fills next bar) and exit timing.
func TestSignalEntryExitAndProfit(t *testing.T) {
	prices := []float64{10, 9, 8, 9, 10, 11, 12, 13, 13}
	sig := make([]float64, len(prices))
	sig[2] = 1 // trigger on the bar priced 8; recovery follows
	s := runSignal(prices, sig, 3)
	if s.NumTrades != 1 {
		t.Fatalf("NumTrades = %d, want 1", s.NumTrades)
	}
	if s.EquityFinal <= 10000 {
		t.Fatalf("EquityFinal = %.2f, want > 10000 (dip recovered)", s.EquityFinal)
	}
}

// No signal means no trades and untouched equity: the strategy trades only on the
// supplied signal, never on its own.
func TestSignalNoTriggerNoTrade(t *testing.T) {
	prices := []float64{10, 9, 8, 9, 10, 11, 12}
	sig := make([]float64, len(prices))
	s := runSignal(prices, sig, 3)
	if s.NumTrades != 0 {
		t.Fatalf("NumTrades = %d, want 0", s.NumTrades)
	}
	if s.EquityFinal != 10000 {
		t.Fatalf("EquityFinal = %.2f, want 10000 (no trades)", s.EquityFinal)
	}
}

// Only one position at a time: a second trigger while in a trade is ignored, and
// the first trade still exits on schedule.
func TestSignalSinglePosition(t *testing.T) {
	prices := []float64{10, 9, 8, 8, 8, 9, 10, 11}
	sig := make([]float64, len(prices))
	sig[2] = 1
	sig[3] = 1 // ignored: still holding the first trade
	s := runSignal(prices, sig, 3)
	if s.NumTrades != 1 {
		t.Fatalf("NumTrades = %d, want 1 (second trigger ignored while in position)", s.NumTrades)
	}
}
