package indicators

import "math"

// VWAP returns the cumulative volume-weighted average price from bar 0, using the
// typical price (High+Low+Close)/3. This is a RUNNING VWAP: VWAP[i] weights bars
// [0, i]. It is NOT session-anchored — intraday daily resets are a caller concern
// (slice per session and concatenate). Bars with zero cumulative volume yield NaN.
func VWAP(high, low, close, volume []float64) []float64 {
	out := make([]float64, len(close))
	var cumPV, cumV float64
	for i := range close {
		tp := (high[i] + low[i] + close[i]) / 3
		cumPV += tp * volume[i]
		cumV += volume[i]
		if cumV != 0 {
			out[i] = cumPV / cumV
		} else {
			out[i] = math.NaN()
		}
	}
	return out
}
