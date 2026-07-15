package backtest

import (
	"fmt"
	"math"
)

// Indicator is a live handle to a precomputed column that always reflects the
// current bar via the data view length. Strategies store these in Init and read
// them in Next.
type Indicator struct {
	full    []float64
	data    *Data
	name    string
	overlay bool
	color   string
}

// IndicatorOption configures a registered indicator's plot metadata.
type IndicatorOption func(*Indicator)

// Overlay draws the indicator on the price pane (e.g. a moving average).
func Overlay() IndicatorOption { return func(i *Indicator) { i.overlay = true } }

// Color sets the indicator's line color (hex, e.g. "#2962FF").
func Color(hex string) IndicatorOption { return func(i *Indicator) { i.color = hex } }

func (ind *Indicator) Last() float64 {
	if ind.data.n == 0 {
		panic("backtest: Indicator.Last() on empty view")
	}
	return ind.full[ind.data.n-1]
}

func (ind *Indicator) At(i int) float64 {
	if i < 0 || i >= ind.data.n {
		panic(fmt.Sprintf("backtest: Indicator.At(%d) out of range [0,%d)", i, ind.data.n))
	}
	return ind.full[ind.data.n-1-i]
}
func (ind *Indicator) Len() int       { return ind.data.n }
func (ind *Indicator) Series() Series { return ind.full[:ind.data.n] }

// leadingNaN returns how many leading values are NaN (indicator warmup).
func leadingNaN(col []float64) int {
	n := 0
	for _, v := range col {
		if math.IsNaN(v) {
			n++
		} else {
			break
		}
	}
	return n
}
