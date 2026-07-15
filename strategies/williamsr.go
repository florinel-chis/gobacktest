// Package strategies provides reusable, parameterized Strategy
// implementations shared by the demo and research commands. Each strategy is
// a plain struct: configure the exported fields (zero values resolve to the
// documented defaults) and pass a pointer to backtest.New or a factory for
// backtest.Optimize.
package strategies

import (
	"fmt"

	backtest "github.com/florinel-chis/gobacktest"
	"github.com/florinel-chis/gobacktest/indicators"
)

// WilliamsROversold is the Williams %R oversold mean-reversion strategy used
// throughout the cmd/ research programs: when flat and %R (optionally
// confirmed by an EMA of %R) is at or under the oversold threshold, buy one
// unit and arm a take-profit off the real entry price. One position at a
// time; no stop-loss.
//
// Zero values resolve to: WRPeriod 21, EMAPeriod 12, WRThreshold -80,
// EMAThreshold -80. (A literal 0 threshold — the top of the %R range — would
// mean "always enter" and is not a useful configuration, so 0 means default.)
// TPAbs (price units) overrides TPPct (percent) when both are set; if both
// are 0 no take-profit is armed.
type WilliamsROversold struct {
	WRPeriod     int     // Williams %R lookback; 0 -> 21
	EMAPeriod    int     // EMA-of-%R lookback (UseEMA only); 0 -> 12
	WRThreshold  float64 // oversold entry threshold in [-100, 0); 0 -> -80
	EMAThreshold float64 // EMA confirmation threshold (UseEMA only); 0 -> -80
	UseEMA       bool    // also require EMA(%R) <= EMAThreshold
	TPPct        float64 // take-profit above entry, percent (10 = +10%)
	TPAbs        float64 // take-profit above entry, price units; overrides TPPct
	Size         float64 // order size per entry; 0 -> 1 unit. Values in (0,1) are a fraction of buying power (engine semantics)

	wr, wrEMA *backtest.Indicator
}

// Init registers Williams %R (and, with UseEMA, its EMA) as subpane
// oscillators. Combined warmup is (WRPeriod-1) + (EMAPeriod-1) bars; the
// engine skips leading-NaN bars so Next never sees NaN.
func (s *WilliamsROversold) Init(st *backtest.State) {
	wrP, emaP := s.WRPeriod, s.EMAPeriod
	if wrP == 0 {
		wrP = 21
	}
	if emaP == 0 {
		emaP = 12
	}

	h, l, c := st.Data().High(), st.Data().Low(), st.Data().Close()
	wr := indicators.WilliamsR(h, l, c, wrP)
	s.wr = st.I(fmt.Sprintf("WilliamsR(%d)", wrP), func() []float64 { return wr }, backtest.Color("#e91e63"))
	if s.UseEMA {
		wrEMA := indicators.EMA(wr, emaP)
		s.wrEMA = st.I(fmt.Sprintf("EMA(%d)", emaP), func() []float64 { return wrEMA }, backtest.Color("#2962FF"))
	}
}

// Next enters when flat and oversold, then arms the take-profit off the real
// entry price (idempotent; orders fill at the NEXT bar's open, so no second
// entry stacks before the fill is visible in Position).
func (s *WilliamsROversold) Next(st *backtest.State) {
	wrThr, emaThr := s.WRThreshold, s.EMAThreshold
	if wrThr == 0 {
		wrThr = -80
	}
	if emaThr == 0 {
		emaThr = -80
	}

	oversold := s.wr.Last() <= wrThr
	if s.UseEMA {
		oversold = oversold && s.wrEMA.Last() <= emaThr
	}
	if st.Position().Size() == 0 && oversold {
		size := s.Size
		if size == 0 {
			size = 1
		}
		st.Buy(backtest.Order{Size: size})
	}

	for _, t := range st.OpenTrades() {
		if t.TP() != 0 {
			continue
		}
		switch {
		case s.TPAbs > 0:
			t.SetTP(t.EntryPrice() + s.TPAbs)
		case s.TPPct > 0:
			t.SetTP(t.EntryPrice() * (1 + s.TPPct/100))
		}
	}
}
