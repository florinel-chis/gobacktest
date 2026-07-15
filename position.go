package backtest

// Position is a live view over the broker's open trades.
type Position struct {
	broker *broker
}

// Size is the signed sum of open trade sizes (positive long, negative short).
func (p *Position) Size() float64 {
	var s float64
	for _, t := range p.broker.trades {
		s += t.size
	}
	return s
}

func (p *Position) IsLong() bool  { return p.Size() > 0 }
func (p *Position) IsShort() bool { return p.Size() < 0 }

// PL is the summed mark-to-market profit/loss of open trades, in cash.
func (p *Position) PL() float64 {
	var pl float64
	for _, t := range p.broker.trades {
		pl += t.pl(p.broker.lastClose)
	}
	return pl
}

// Close queue-closes all open trades. Matching backtesting.py, each close is a
// market order filled at the NEXT bar's open (not synchronously), so it never
// introduces look-ahead. The position remains open during the current bar.
func (p *Position) Close() {
	for _, t := range p.broker.trades {
		p.broker.orders = append(p.broker.orders, &order{size: -t.size, parent: t})
	}
}
