package backtest

import (
	"fmt"
	"math"
)

// Strategy is implemented by user trading strategies.
type Strategy interface {
	Init(*State)
	Next(*State)
}

// State is the per-run context passed to Init/Next.
type State struct {
	broker     *broker
	indicators []*Indicator
}

func (s *State) Data() *Data { return s.broker.data }

// I registers an indicator: compute runs once (full data view), the result must
// match the data's full length, and the returned handle auto-slices per bar.
// Optional IndicatorOption values (Overlay, Color) attach plot metadata for reports.
func (s *State) I(name string, compute func() []float64, opts ...IndicatorOption) *Indicator {
	col := compute()
	if len(col) != s.broker.data.fullLen() {
		panic(fmt.Sprintf("backtest: indicator %q length %d != data length %d", name, len(col), s.broker.data.fullLen()))
	}
	if _, exists := s.broker.data.Column(name); exists {
		panic(fmt.Sprintf("backtest: indicator %q already registered", name))
	}
	if err := s.broker.data.AddColumn(name, col); err != nil {
		panic(err)
	}
	ind := &Indicator{full: col, data: s.broker.data, name: name}
	for _, o := range opts {
		o(ind)
	}
	s.indicators = append(s.indicators, ind)
	return ind
}

func (s *State) enqueue(size float64, o Order) OrderHandle {
	if size == 0 || math.IsNaN(size) {
		panic(fmt.Sprintf("backtest: invalid order size %v", size))
	}
	ord := &order{size: size, limit: o.Limit, stop: o.Stop, sl: o.SL, tp: o.TP, tag: o.Tag}
	s.broker.orders = append(s.broker.orders, ord)
	return OrderHandle{o: ord}
}

// Buy enqueues a long order. Size must be > 0.
func (s *State) Buy(o Order) OrderHandle {
	if o.Size <= 0 || math.IsNaN(o.Size) {
		panic(fmt.Sprintf("backtest: Buy size must be > 0, got %v", o.Size))
	}
	return s.enqueue(o.Size, o)
}

// Sell enqueues a short order. Size must be > 0 (enqueued as negative).
func (s *State) Sell(o Order) OrderHandle {
	if o.Size <= 0 || math.IsNaN(o.Size) {
		panic(fmt.Sprintf("backtest: Sell size must be > 0, got %v", o.Size))
	}
	return s.enqueue(-o.Size, o)
}

func (s *State) Position() *Position { return s.broker.position }
func (s *State) Equity() float64     { return s.broker.equityNow() }

func (s *State) Trades() []Trade {
	out := make([]Trade, 0, len(s.broker.trades))
	for _, t := range s.broker.trades {
		out = append(out, t.snapshot(s.broker.lastClose))
	}
	return out
}

func (s *State) ClosedTrades() []Trade {
	out := make([]Trade, 0, len(s.broker.closedTrades))
	for _, t := range s.broker.closedTrades {
		out = append(out, t.export())
	}
	return out
}

// OpenTrades returns mutable handles to the currently-open trades.
func (s *State) OpenTrades() []*OpenTrade {
	out := make([]*OpenTrade, 0, len(s.broker.trades))
	for _, t := range s.broker.trades {
		out = append(out, &OpenTrade{t: t})
	}
	return out
}

func (s *State) Orders() []OrderHandle {
	out := make([]OrderHandle, 0, len(s.broker.orders))
	for _, o := range s.broker.orders {
		out = append(out, OrderHandle{o: o})
	}
	return out
}
