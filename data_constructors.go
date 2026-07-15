package backtest

import (
	"fmt"
	"time"
)

// FromOHLCV builds Data from parallel columns. All slices must be equal length.
// The view starts at full length. FromOHLCV retains (does not copy) the
// provided slices; callers must not mutate them after construction.
func FromOHLCV(t []time.Time, open, high, low, close, volume []float64) (*Data, error) {
	n := len(t)
	for name, col := range map[string]int{"open": len(open), "high": len(high), "low": len(low), "close": len(close), "volume": len(volume)} {
		if col != n {
			return nil, fmt.Errorf("backtest: %s length %d != time length %d", name, col, n)
		}
	}
	return &Data{time: t, open: open, high: high, low: low, close: close, volume: volume, n: n}, nil
}

// FromBars builds Data from an array-of-structs at the caller's boundary.
func FromBars(bars []Bar) *Data {
	n := len(bars)
	d := &Data{
		time:   make([]time.Time, n),
		open:   make([]float64, n),
		high:   make([]float64, n),
		low:    make([]float64, n),
		close:  make([]float64, n),
		volume: make([]float64, n),
		n:      n,
	}
	for i, b := range bars {
		d.time[i], d.open[i], d.high[i], d.low[i], d.close[i], d.volume[i] = b.Time, b.Open, b.High, b.Low, b.Close, b.Volume
	}
	return d
}
