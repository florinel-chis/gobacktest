package backtest

// OpenTrade is a live, mutable handle to a currently-open trade (for trailing
// stops and other dynamic SL/TP logic). Obtain via State.OpenTrades() and use it
// within the same bar — once the underlying trade closes, the handle becomes inert
// (the broker no longer reads its SL/TP). Re-fetch OpenTrades() each Next().
type OpenTrade struct{ t *trade }

func (o *OpenTrade) IsLong() bool        { return o.t.isLong() }
func (o *OpenTrade) IsShort() bool       { return o.t.isShort() }
func (o *OpenTrade) Size() float64       { return o.t.size }
func (o *OpenTrade) EntryPrice() float64 { return o.t.entryPrice }
func (o *OpenTrade) SL() float64         { return o.t.sl }
func (o *OpenTrade) SetSL(price float64) { o.t.sl = price }
func (o *OpenTrade) TP() float64         { return o.t.tp }
func (o *OpenTrade) SetTP(price float64) { o.t.tp = price }
