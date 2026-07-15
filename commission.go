package backtest

import "math"

// Commission computes the cash cost of trading `size` units at `price`.
type Commission interface {
	cost(size, price float64) float64
}

type pctComm float64

func (c pctComm) cost(size, price float64) float64 { return math.Abs(size) * price * float64(c) }

// Pct charges a relative commission: |size| * price * rate.
func Pct(rate float64) Commission { return pctComm(rate) }

type fixedPlusPct struct{ fixed, rate float64 }

func (c fixedPlusPct) cost(size, price float64) float64 {
	return c.fixed + math.Abs(size)*price*c.rate
}

// FixedPlusPct charges a fixed amount plus a relative commission.
func FixedPlusPct(fixed, rate float64) Commission { return fixedPlusPct{fixed, rate} }

// CommissionFunc adapts a function to the Commission interface.
type CommissionFunc func(size, price float64) float64

func (f CommissionFunc) cost(size, price float64) float64 { return f(size, price) }

// adjustedPrice applies the bid/ask spread directionally to a fill price.
func adjustedPrice(spread, size, price float64) float64 {
	return price * (1 + math.Copysign(spread, size))
}
