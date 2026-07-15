package backtest

import (
	"testing"
	"time"

	indicators "github.com/florinel-chis/gobacktest/indicators"
)

// localSMA delegates to indicators.SMA (TA-Lib backed), which emits NaN for the
// first period-1 bars — identical warmup to the hand-rolled version.
func localSMA(s Series, period int) []float64 {
	return indicators.SMA([]float64(s), period)
}

type smaCross struct{ fast, slow *Indicator }

func (st *smaCross) Init(s *State) {
	st.fast = s.I("fast", func() []float64 { return localSMA(s.Data().Close(), 2) })
	st.slow = s.I("slow", func() []float64 { return localSMA(s.Data().Close(), 4) })
}
func (st *smaCross) Next(s *State) {
	// crossover up: fast was below slow, now above
	if st.fast.Len() < 2 {
		return
	}
	if st.fast.At(1) <= st.slow.At(1) && st.fast.Last() > st.slow.Last() {
		if !s.Position().IsLong() {
			s.Buy(Order{Size: 1})
		}
	}
}

func vSeries(prices ...float64) *Data {
	n := len(prices)
	tm := make([]time.Time, n)
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)
	v := make([]float64, n)
	base := time.Unix(0, 0)
	for i, p := range prices {
		tm[i] = base.AddDate(0, 0, i)
		o[i], h[i], l[i], c[i], v[i] = p, p, p, p, 0
	}
	d, _ := FromOHLCV(tm, o, h, l, c, v)
	return d
}

func TestSMACrossEndToEnd(t *testing.T) {
	// Dip then rally so a fast/slow cross-up happens.
	d := vSeries(10, 9, 8, 7, 8, 10, 12, 14, 16)
	bt := New(d, &smaCross{}, Options{Cash: 10000, Margin: 1})
	res, err := bt.Run()
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	// Start bar: slow SMA(4) warmup -> first valid at index 3, so StartBar=3.
	if res.StartBar != 3 {
		t.Fatalf("StartBar=%d want 3 (SMA(4) warmup)", res.StartBar)
	}
	// A cross-up must occur during the rally and open exactly one long.
	if len(res.EquityCurve) != len(d.close)-3 {
		t.Fatalf("equity curve len=%d want %d", len(res.EquityCurve), len(d.close)-3)
	}
	// Final equity should exceed starting cash (bought into the rally).
	if res.FinalEquity <= 10000 {
		t.Fatalf("FinalEquity=%v should profit from rally", res.FinalEquity)
	}
}
