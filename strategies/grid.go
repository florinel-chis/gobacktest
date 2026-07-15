package strategies

import (
	"fmt"
	"math"

	backtest "github.com/florinel-chis/gobacktest"
	"github.com/florinel-chis/gobacktest/indicators"
)

// AveragingGrid is a long-only averaging-down grid ("linear martingale"):
//
//   - the FIRST lot opens when Williams %R(WRPeriod) is oversold
//     (<= WRThreshold), size BaseSize;
//   - while the book is open, a new lot is added every time price falls
//     SpacingPct below the LOWEST open entry — with Growing, lot k has size
//     k x BaseSize (1x, 2x, 3x, ...), so the largest lot gets the lowest
//     entry and the book's average entry is pulled toward the bottom;
//   - every lot carries its own take-profit at entry x (1 + TPPct/100); on a
//     bounce the last (largest) lot realizes the largest absolute profit
//     first, releasing capital while earlier lots wait for a deeper recovery;
//   - MaxLevels caps the grid depth. There is NO stop-loss: the known tail
//     risk is a sustained trend that fills every level and keeps falling.
//     Worst-case exposure with Growing is BaseSize x L(L+1)/2 lots' notional —
//     capital grows quadratically with depth. Size it consciously.
//
// Zero values: WRPeriod 21, WRThreshold -80, SpacingPct 0.5, TPPct 0.4,
// MaxLevels 5, BaseSize 1.
type AveragingGrid struct {
	WRPeriod    int
	WRThreshold float64
	SpacingPct  float64 // add a lot when price <= lowest entry x (1 - SpacingPct/100)
	TPPct       float64 // per-lot take-profit above its own entry
	MaxLevels   int
	BaseSize    float64 // units of lot 1 (values in (0,1) would be read by the engine as a fraction — use >= 1)
	Growing     bool    // lot k = k x BaseSize; false: every lot BaseSize

	wr *backtest.Indicator
}

// Init registers the oversold trigger indicator.
func (g *AveragingGrid) Init(st *backtest.State) {
	wrP := g.WRPeriod
	if wrP == 0 {
		wrP = 21
	}
	h, l, c := st.Data().High(), st.Data().Low(), st.Data().Close()
	wr := indicators.WilliamsR(h, l, c, wrP)
	g.wr = st.I(fmt.Sprintf("WilliamsR(%d)", wrP), func() []float64 { return wr }, backtest.Color("#e91e63"))
}

// Next arms per-lot TPs, then evaluates first-entry / grid-add conditions.
func (g *AveragingGrid) Next(st *backtest.State) {
	thr, spacing, tp, maxL, base := g.WRThreshold, g.SpacingPct, g.TPPct, g.MaxLevels, g.BaseSize
	if thr == 0 {
		thr = -80
	}
	if spacing == 0 {
		spacing = 0.5
	}
	if tp == 0 {
		tp = 0.4
	}
	if maxL == 0 {
		maxL = 5
	}
	if base == 0 {
		base = 1
	}

	open := st.OpenTrades()
	lowest := math.Inf(1)
	for _, t := range open {
		if t.TP() == 0 {
			t.SetTP(t.EntryPrice() * (1 + tp/100))
		}
		if t.EntryPrice() < lowest {
			lowest = t.EntryPrice()
		}
	}
	if len(st.Orders()) != 0 || len(open) >= maxL {
		return
	}

	price := st.Data().Close().Last()
	switch {
	case len(open) == 0:
		if v := g.wr.Last(); !math.IsNaN(v) && v <= thr {
			st.Buy(backtest.Order{Size: base})
		}
	case price <= lowest*(1-spacing/100):
		size := base
		if g.Growing {
			size = base * float64(len(open)+1)
		}
		st.Buy(backtest.Order{Size: size})
	}
}
