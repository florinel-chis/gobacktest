package lib

import (
	"math"
	"testing"
	"time"

	backtest "github.com/florinel-chis/gobacktest"
)

func TestTrailingATR(t *testing.T) {
	// 6 bars, period=3. tr[0] is NaN (no previous close); the rolling mean is
	// therefore NaN for every window containing index 0, i.e. indices [0,
	// period-1]=[0,2]; the first valid value is at index==period==3, and
	// indices [0,2] must be backfilled with atr[3].
	high := []float64{10, 11, 12, 13, 14, 15}
	low := []float64{9, 10, 11, 12, 13, 14}
	close := []float64{9.5, 10.5, 11.5, 12.5, 13.5, 14.5}
	atr := TrailingATR(high, low, close, 3)
	if len(atr) != 6 {
		t.Fatalf("len(atr) = %d, want 6", len(atr))
	}
	for i, v := range atr {
		if math.IsNaN(v) {
			t.Fatalf("atr[%d] is NaN, want backfilled/computed value", i)
		}
	}
	// Indices [0,2] must all equal the first valid value (index 3).
	if atr[0] != atr[3] || atr[1] != atr[3] || atr[2] != atr[3] {
		t.Fatalf("leading values not backfilled from index 3: atr=%v", atr)
	}
	// tr[i] = max(hi-lo, |c_prev-hi|, |c_prev-lo|). For i=1..3: hi-lo=1, but
	// |c_prev-hi|=1.5 dominates (c_prev = close[i-1] = hi[i-1]-0.5), so
	// tr[1..3] = 1.5 each and atr[3] = mean(tr[1..3]) = 1.5.
	if math.Abs(atr[3]-1.5) > 1e-9 {
		t.Fatalf("atr[3] = %v, want 1.5", atr[3])
	}
}

func TestTrailingATRShortSeries(t *testing.T) {
	// n < period: every rolling window is NaN, so bfill has nothing to
	// propagate — the whole result stays NaN.
	high := []float64{10, 11}
	low := []float64{9, 10}
	close := []float64{9.5, 10.5}
	atr := TrailingATR(high, low, close, 5)
	for i, v := range atr {
		if !math.IsNaN(v) {
			t.Fatalf("atr[%d] = %v, want NaN (n < period)", i, v)
		}
	}
}

// trailingLongStrategy buys once on the first bar and trails the SL every bar
// via TrailStop; it records the SL after every Next call while a trade is open.
type trailingLongStrategy struct {
	atr    *backtest.Indicator
	bought bool
	slHist []float64
}

func (s *trailingLongStrategy) Init(st *backtest.State) {
	high, low, close := st.Data().High(), st.Data().Low(), st.Data().Close()
	s.atr = st.I("atr", func() []float64 { return TrailingATR(high, low, close, 3) })
}

func (s *trailingLongStrategy) Next(st *backtest.State) {
	if !s.bought {
		st.Buy(backtest.Order{Size: 1})
		s.bought = true
	}
	TrailStop(st, s.atr.Last(), 2)
	if open := st.OpenTrades(); len(open) > 0 {
		s.slHist = append(s.slHist, open[0].SL())
	}
}

// TestTrailStopLongTrailsUpAndExits is a hermetic (no network, no venv)
// functional test: a long trade's SL must trail UP monotonically as price
// rises, and the trade must exit once the trailing SL is breached by a drop.
func TestTrailStopLongTrailsUpAndExits(t *testing.T) {
	base := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)
	// Steady rise for bars 0..8 (entry fills at bar 1's open), then a sharp
	// gap-down drop at bar 9 that gaps through any plausible trailing SL.
	type ohlc struct{ o, h, l, c float64 }
	rows := []ohlc{
		{100, 101, 99, 100},
		{100, 102, 99, 101},
		{101, 103, 100, 102},
		{102, 104, 101, 103},
		{103, 105, 102, 104},
		{104, 106, 103, 105},
		{105, 107, 104, 106},
		{106, 108, 105, 107},
		{107, 109, 106, 108},
		{90, 91, 85, 86}, // gap-down: should trigger the trailing SL
		{85, 86, 83, 84},
		{84, 85, 82, 83},
	}
	n := len(rows)
	times := make([]time.Time, n)
	open := make([]float64, n)
	high := make([]float64, n)
	low := make([]float64, n)
	close := make([]float64, n)
	volume := make([]float64, n)
	for i, r := range rows {
		times[i] = base.AddDate(0, 0, i)
		open[i], high[i], low[i], close[i] = r.o, r.h, r.l, r.c
		volume[i] = 1000
	}
	d, err := backtest.FromOHLCV(times, open, high, low, close, volume)
	if err != nil {
		t.Fatalf("FromOHLCV: %v", err)
	}

	strat := &trailingLongStrategy{}
	bt := backtest.New(d, strat, backtest.Options{Cash: 100000, Margin: 1, FinalizeTrades: true})
	res, err := bt.Run()
	if err != nil {
		t.Fatalf("Run: %v", err)
	}

	if len(res.Trades) != 1 {
		t.Fatalf("len(res.Trades) = %d, want 1 (trailing SL should have closed the only trade)", len(res.Trades))
	}
	tr := res.Trades[0]
	if tr.EntryBar != 1 {
		t.Fatalf("EntryBar = %d, want 1 (order submitted at bar 0 fills at bar 1's open)", tr.EntryBar)
	}
	if tr.EntryPrice != open[1] {
		t.Fatalf("EntryPrice = %v, want %v", tr.EntryPrice, open[1])
	}
	if tr.ExitBar != 9 {
		t.Fatalf("ExitBar = %d, want 9 (the gap-down bar)", tr.ExitBar)
	}
	// The gap-down bar's open (90) is below any plausible trailing SL, so the
	// exit must fill at that bar's open, not at a stale (higher) SL price.
	if tr.ExitPrice != open[9] {
		t.Fatalf("ExitPrice = %v, want %v (gap-down fill at open, not stale SL)", tr.ExitPrice, open[9])
	}

	// SL must never decrease while the trade is open (long trailing SL only
	// tightens: max(currentSL, newSL)).
	for i := 1; i < len(strat.slHist); i++ {
		if strat.slHist[i] < strat.slHist[i-1] {
			t.Fatalf("SL decreased at step %d: %v -> %v (want monotonic non-decreasing)", i, strat.slHist[i-1], strat.slHist[i])
		}
	}
	// And it must have actually trailed up at least once (not stuck at the
	// initial 0/unset value).
	if strat.slHist[0] == 0 || strat.slHist[len(strat.slHist)-1] <= strat.slHist[0] {
		t.Fatalf("SL did not trail up: history=%v", strat.slHist)
	}
}
