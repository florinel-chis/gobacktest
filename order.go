package backtest

// Order is the public specification passed to State.Buy / State.Sell.
// Size is a fraction in (0,1] of available equity, or absolute units >= 1.
// Zero-valued Limit/Stop/SL/TP mean "unset".
type Order struct {
	Size  float64
	Limit float64
	Stop  float64
	SL    float64
	TP    float64
	Tag   any
}

// order is the broker's internal queued order. size is signed
// (positive long, negative short). contingent marks SL/TP child orders.
type order struct {
	size       float64
	limit      float64
	stop       float64
	sl         float64
	tp         float64
	tag        any
	contingent bool
	parent     *trade
	canceled   bool
}

func (o *order) isLong() bool  { return o.size > 0 }
func (o *order) isShort() bool { return o.size < 0 }

// Cancel marks the order canceled; the broker skips it on the next pass.
func (o *order) Cancel() { o.canceled = true }

// OrderHandle is a cancelable reference to a queued order returned by Buy/Sell.
type OrderHandle struct{ o *order }

// Cancel cancels the queued order; the broker skips it on the next pass.
func (h OrderHandle) Cancel() { h.o.canceled = true }

// Size is the signed size of the queued order (positive long, negative short).
func (h OrderHandle) Size() float64 { return h.o.size }
