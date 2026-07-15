package backtest

import (
	"errors"
	"time"
)

// Options configures a Backtest run.
type Options struct {
	Cash            float64
	Spread          float64
	Commission      Commission
	Margin          float64
	TradeOnClose    bool
	Hedging         bool
	ExclusiveOrders bool
	FinalizeTrades  bool
}

// EquityPoint is one bar of the equity curve.
type EquityPoint struct {
	Time   time.Time
	Equity float64
}

// IndicatorSeries is a registered indicator's values + plot metadata (for reports).
type IndicatorSeries struct {
	Name    string
	Overlay bool
	Color   string
	Values  []float64
}

// Result is the raw timeline produced by a run. Statistics (Plan 3) are
// computed from this.
type Result struct {
	EquityCurve []EquityPoint
	Trades      []Trade
	FinalEquity float64
	StartBar    int
	// InPositionBars[k] is whether a position was open at equity-curve point k,
	// recorded after strategy.Next (a same-bar Buy is not yet filled). Note: the
	// ExitBar of a deferred close reads false here; exposure is derived from trades.
	InPositionBars []bool
	// Indicators holds each registered indicator's values and plot metadata.
	Indicators []IndicatorSeries
}

// ErrNoBars is returned by Backtest.Run when the data contains zero bars
// (e.g. a Data built from empty slices), so there is nothing to run on.
var ErrNoBars = errors.New("backtest: data has no bars")

// Backtest binds data + strategy + options.
type Backtest struct {
	data     *Data
	strategy Strategy
	opts     Options
}

// New creates a Backtest.
func New(data *Data, strategy Strategy, opts Options) *Backtest {
	return &Backtest{data: data, strategy: strategy, opts: opts}
}

func maxLeadingNaN(inds []*Indicator) int {
	m := 0
	for _, ind := range inds {
		if n := leadingNaN(ind.full); n > m {
			m = n
		}
	}
	return m
}

// Run executes the strategy bar-by-bar and returns the timeline.
func (bt *Backtest) Run() (*Result, error) {
	if bt.data.fullLen() == 0 {
		return nil, ErrNoBars
	}
	b := newBroker(bt.data, bt.opts)
	s := &State{broker: b}

	bt.data.setLen(bt.data.fullLen())
	bt.strategy.Init(s)

	start := maxLeadingNaN(s.indicators)
	var runErr error
	n := bt.data.fullLen()

	inPos := make([]bool, 0, n-start)
	for i := start; i < n; i++ {
		b.i = i
		bt.data.setLen(i + 1)

		prevClose := bt.data.closeAt(i)
		if i > 0 {
			prevClose = bt.data.closeAt(i - 1)
		}
		b.lastClose = prevClose
		b.processOrders()

		b.lastClose = bt.data.closeAt(i)
		b.processContingent()

		if err := b.next(); err != nil {
			runErr = err
			break
		}
		bt.strategy.Next(s)
		inPos = append(inPos, len(b.trades) > 0)
	}

	if runErr == nil && bt.opts.FinalizeTrades {
		// Close leftover open trades at the LAST BAR'S OPEN, matching
		// backtesting.py's finalize semantics: orders fill at that bar's open,
		// not its close.
		finalOpen := b.data.openAt(n - 1)
		for _, t := range append([]*trade{}, b.trades...) {
			b.closeTradeAt(t, finalOpen)
		}
		b.i = n - 1
		b.recordEquity()
	}

	res := &Result{StartBar: start, InPositionBars: inPos}
	last := n - 1
	for j := start; j < n; j++ {
		res.EquityCurve = append(res.EquityCurve, EquityPoint{Time: bt.data.timeAt(j), Equity: b.equity[j]})
		if b.equity[j] != 0 {
			last = j
		}
	}
	for _, t := range b.closedTrades {
		res.Trades = append(res.Trades, t.export())
	}
	res.FinalEquity = b.equity[last]
	if errors.Is(runErr, ErrOutOfMoney) {
		// The run was liquidated: backtesting.py reports 0 final equity on ruin,
		// even though the equity curve's last non-zero point (tracked by `last`
		// above) is the pre-liquidation balance.
		res.FinalEquity = 0
	}
	for _, ind := range s.indicators {
		res.Indicators = append(res.Indicators, IndicatorSeries{
			Name: ind.name, Overlay: ind.overlay, Color: ind.color, Values: ind.full,
		})
	}
	return res, runErr
}
