package backtest

import (
	"errors"
	"math"
)

// ErrOutOfMoney is returned by Backtest.Run when available equity goes to zero
// (the account is liquidated). On this path the equity curve ends at 0 and
// Stats.EquityFinal is 0.
var ErrOutOfMoney = errors.New("backtest: out of money")

type broker struct {
	data            *Data
	cash            float64
	spread          float64
	commission      Commission
	margin          float64
	tradeOnClose    bool
	hedging         bool
	exclusiveOrders bool
	orders          []*order
	trades          []*trade
	closedTrades    []*trade
	position        *Position
	equity          []float64
	lastClose       float64
	i               int
	// fillBar is the bar a fill being processed is stamped with, or -1 for the
	// current bar. backtesting.py stamps a market order filled under
	// trade_on_close with the bar that placed it (time_index = i - 1), the bar
	// whose close is the fill price.
	fillBar       int
	contingentBuf []*trade // reused snapshot buffer for processContingent (per-bar hot path)
}

func newBroker(data *Data, opts Options) *broker {
	margin := opts.Margin
	if margin == 0 {
		margin = 1
	}
	cash := opts.Cash
	if cash == 0 {
		cash = 10000
	}
	b := &broker{
		data:            data,
		cash:            cash,
		spread:          opts.Spread,
		commission:      opts.Commission,
		margin:          margin,
		tradeOnClose:    opts.TradeOnClose,
		hedging:         opts.Hedging,
		exclusiveOrders: opts.ExclusiveOrders,
		equity:          make([]float64, data.fullLen()),
		fillBar:         -1,
	}
	b.position = &Position{broker: b}
	return b
}

func (b *broker) commissionCost(size, price float64) float64 {
	if b.commission == nil {
		return 0
	}
	return b.commission.cost(size, price)
}

func (b *broker) equityNow() float64 {
	eq := b.cash
	for _, t := range b.trades {
		eq += t.pl(b.lastClose)
	}
	return eq
}

func (b *broker) leverage() float64 { return 1 / b.margin }

func (b *broker) marginUsed() float64 {
	var m float64
	for _, t := range b.trades {
		m += t.value(b.lastClose) / b.leverage()
	}
	return m
}

func (b *broker) marginAvailable() float64 {
	return math.Max(0, b.equityNow()-b.marginUsed())
}

// closeTrade closes `portion` (0..1) of a trade at lastClose. Minimal version;
// fill-price nuances handled by SL/TP processing in later tasks.

// isOpen reports whether t is still an open trade.
func (b *broker) isOpen(t *trade) bool {
	for _, x := range b.trades {
		if x == t {
			return true
		}
	}
	return false
}

func (b *broker) removeTrade(t *trade) {
	out := b.trades[:0]
	for _, x := range b.trades {
		if x != t {
			out = append(out, x)
		}
	}
	b.trades = out
}

// perUnitPrice is the fill price plus the per-unit commission, mirroring
// backtesting.py's adjusted_price_plus_commission = adjPrice + commission(size, rawPrice)/|size|.
func (b *broker) perUnitPrice(o *order, adjPrice, rawPrice float64) float64 {
	p := adjPrice
	if b.commission != nil {
		p += b.commission.cost(o.size, rawPrice) / math.Abs(o.size)
	}
	return p
}

// resolveSize converts an order's size into whole units. Absolute sizes (|size|>=1)
// truncate to units; fractional sizes (0<|size|<1) are a fraction of BUYING POWER
// (equity available for margin × leverage), matching backtesting.py:
//
//	units = floor( (marginAvailable * leverage * |frac|) / (adjPrice + commission/|frac|) )
func (b *broker) resolveSize(o *order, adjPrice, rawPrice float64) float64 {
	if math.Abs(o.size) >= 1 {
		return math.Trunc(o.size) // absolute units, already signed
	}
	buyingPower := b.marginAvailable() * b.leverage()
	units := math.Floor(buyingPower * math.Abs(o.size) / b.perUnitPrice(o, adjPrice, rawPrice))
	return math.Copysign(units, o.size)
}

func (b *broker) openTrade(size, price float64, sl, tp float64, tag any) {
	t := &trade{
		size:       size,
		entryPrice: price,
		entryBar:   b.stampBar(),
		entryTime:  b.data.timeAt(b.stampBar()),
		sl:         sl,
		tp:         tp,
		tag:        tag,
		entryComm:  b.commissionCost(size, price),
	}
	b.cash -= t.entryComm
	b.trades = append(b.trades, t)
}

// stampBar is the bar index a fill is recorded at (see fillBar).
func (b *broker) stampBar() int {
	if b.fillBar >= 0 {
		return b.fillBar
	}
	return b.i
}

// closeTradeAt closes a full trade at an explicit price/bar (for SL/TP fills).
func (b *broker) closeTradeAt(t *trade, price float64) {
	t.exitPrice = price
	t.exitBar = b.stampBar()
	t.exitTime = b.data.timeAt(t.exitBar)
	t.exitComm = b.commissionCost(t.size, price)
	b.cash += t.pl(price) - t.exitComm
	b.closedTrades = append(b.closedTrades, t)
	b.removeTrade(t)
}

// processContingent fills a trade's stop-loss/take-profit intrabar. Matching
// backtesting.py's _process_orders: a triggered SL/TP is a market order once
// hit, so on a gap through the trigger price it fills at the bar's open (not
// the stale sl/tp price) — min(open, sl) long / max(open, sl) short for SL,
// max(open, tp) long / min(open, tp) short for TP.
func (b *broker) processContingent() {
	open := b.data.openAt(b.i)
	high := b.data.highAt(b.i)
	low := b.data.lowAt(b.i)
	// Snapshot into a reused buffer (we mutate b.trades via closeTradeAt below).
	b.contingentBuf = append(b.contingentBuf[:0], b.trades...)
	for _, t := range b.contingentBuf {
		slHit := t.sl != 0 && ((t.isLong() && low <= t.sl) || (t.isShort() && high >= t.sl))
		tpHit := t.tp != 0 && ((t.isLong() && high >= t.tp) || (t.isShort() && low <= t.tp))
		switch {
		case slHit: // stop-loss takes priority when both hit
			price := math.Max(open, t.sl)
			if t.isLong() {
				price = math.Min(open, t.sl)
			}
			b.closeTradeAt(t, price)
		case tpHit:
			price := math.Min(open, t.tp)
			if t.isLong() {
				price = math.Max(open, t.tp)
			}
			b.closeTradeAt(t, price)
		}
	}
}

func (b *broker) processOrders() {
	open := b.data.openAt(b.i)
	high := b.data.highAt(b.i)
	low := b.data.lowAt(b.i)
	prevClose := b.lastClose // caller sets lastClose to previous bar's close before processOrders

	// Iterate a stable snapshot; build a fresh remaining list. Neither aliases
	// b.orders, so mutations during exclusive handling cannot clobber iteration.
	defer func() { b.fillBar = -1 }()

	pending := append([]*order(nil), b.orders...)
	remaining := make([]*order, 0, len(pending))
	for idx, o := range pending {
		if o.canceled {
			continue
		}
		stopPrice := 0.0
		if o.stop != 0 {
			triggered := (o.isLong() && high >= o.stop) || (o.isShort() && low <= o.stop)
			if !triggered {
				remaining = append(remaining, o)
				continue
			}
			stopPrice = o.stop
			o.stop = 0 // latch: once triggered, behave as market/limit on this and later bars
		}
		if o.limit != 0 {
			hit := (o.isLong() && low <= o.limit) || (o.isShort() && high >= o.limit)
			if !hit {
				remaining = append(remaining, o)
				continue
			}
		}
		// Market orders filled at the previous close are stamped with that bar.
		b.fillBar = -1
		if b.tradeOnClose && !o.contingent && o.limit == 0 && stopPrice == 0 && b.i > 0 {
			b.fillBar = b.i - 1
		}

		var price float64
		switch {
		case o.limit != 0:
			base := open
			if stopPrice != 0 {
				base = stopPrice
			}
			if o.isLong() {
				price = math.Min(base, o.limit)
			} else {
				price = math.Max(base, o.limit)
			}
		case b.tradeOnClose && !o.contingent:
			price = prevClose
		case stopPrice != 0:
			price = stopPrice
		default:
			price = open
		}

		rawPrice := price
		price = adjustedPrice(b.spread, o.size, rawPrice)
		if o.parent != nil {
			// A close order: close its specific parent trade at this fill price
			// (skip if the trade already closed, e.g. via SL/TP, between queueing
			// and now). Does not go through netting/openTrade.
			if b.isOpen(o.parent) {
				b.closeTradeAt(o.parent, price)
			}
			continue
		}
		if b.exclusiveOrders && !o.contingent {
			// Exclusive: close all open trades and cancel every other pending
			// non-contingent order (already-kept this pass and still-ahead).
			b.closeAllTrades(price)
			kept := remaining[:0]
			for _, r := range remaining {
				if r.contingent {
					kept = append(kept, r)
				}
			}
			remaining = kept
			for _, later := range pending[idx+1:] {
				if !later.contingent {
					later.canceled = true
				}
			}
		}
		size := b.resolveSize(o, price, rawPrice)
		if size == 0 {
			continue // insufficient buying power for even one unit — cancel
		}
		if !b.hedging {
			size = b.net(size, price)
			if size == 0 {
				continue
			}
		}
		// Liquidity check (backtesting.py): cancel if the residual can't be covered
		// by available buying power (margin × leverage), incl. per-unit commission.
		if math.Abs(size)*b.perUnitPrice(o, price, rawPrice) > b.marginAvailable()*b.leverage() {
			continue
		}
		b.openTrade(size, price, o.sl, o.tp, o.tag)
	}
	b.orders = remaining
}

// net reduces opposite-direction open trades FIFO against an incoming `size`.
// Returns the residual size still to be opened (same sign as `size`), or 0.
func (b *broker) net(size, price float64) float64 {
	residual := size
	for len(b.trades) > 0 && residual != 0 {
		t := b.trades[0] // FIFO
		// only net against opposite direction
		if (residual > 0) == (t.size > 0) {
			break
		}
		if math.Abs(residual) >= math.Abs(t.size) {
			residual += t.size // shrink residual by the closed lot
			b.closeTradeAt(t, price)
		} else {
			// partial close of the front trade
			portion := math.Abs(residual) / math.Abs(t.size)
			b.closePortionAt(t, portion, price)
			residual = 0
		}
	}
	return residual
}

// closePortionAt closes `portion` of t at price, leaving the remainder open.
func (b *broker) closePortionAt(t *trade, portion, price float64) {
	closing := &trade{
		size:       t.size * portion,
		entryPrice: t.entryPrice,
		entryBar:   t.entryBar,
		entryTime:  t.entryTime,
		entryComm:  t.entryComm * portion,
		exitPrice:  price,
		exitBar:    b.stampBar(),
		exitTime:   b.data.timeAt(b.stampBar()),
	}
	closing.exitComm = b.commissionCost(closing.size, price)
	b.cash += closing.pl(price) - closing.exitComm
	b.closedTrades = append(b.closedTrades, closing)
	t.size -= closing.size
	t.entryComm -= closing.entryComm
}

// closeAllTrades closes every open trade at price (snapshot iteration).
func (b *broker) closeAllTrades(price float64) {
	for _, t := range append([]*trade{}, b.trades...) {
		b.closeTradeAt(t, price)
	}
}

func (b *broker) recordEquity() { b.equity[b.i] = b.equityNow() }

// next records equity for the current bar and liquidates if broke.
func (b *broker) next() error {
	if b.equityNow() <= 0 {
		b.closeAllTrades(b.lastClose)
		b.cash = 0
		for j := b.i; j < len(b.equity); j++ {
			b.equity[j] = 0
		}
		return ErrOutOfMoney
	}
	b.recordEquity()
	return nil
}
