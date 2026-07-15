// Package indicators wraps github.com/markcheno/go-talib with NaN-warmup and
// composition support (so indicators chain, e.g. EMA of Williams %R).
package indicators

import "math"

func nanSlice(n int) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = math.NaN()
	}
	return s
}

func leadingNaN(s []float64) int {
	n := 0
	for _, v := range s {
		if math.IsNaN(v) {
			n++
		} else {
			break
		}
	}
	return n
}

func maxLeadingNaN(cols ...[]float64) int {
	m := 0
	for _, c := range cols {
		if n := leadingNaN(c); n > m {
			m = n
		}
	}
	return m
}

// apply runs a single-input TA-Lib function with NaN-aware composition.
// `in` may carry leading NaN (output of another indicator). lookback is the
// indicator's warmup length; go-talib zero-fills its first `lookback` outputs,
// which we drop. Returns a full-length NaN-warmup series aligned to the input.
func apply(in []float64, lookback int, fn func([]float64) []float64) []float64 {
	out := nanSlice(len(in))
	if lookback < 0 { // invalid period (e.g. EMA(s, 0)) -> all NaN, never panic
		return out
	}
	start := leadingNaN(in)
	valid := in[start:]
	if len(valid) <= lookback {
		return out
	}
	res := fn(valid)
	for i := lookback; i < len(res); i++ {
		out[start+i] = res[i]
	}
	return out
}

// applyHLC is apply for triple-input (high/low/close) indicators. The three
// columns MUST be length-aligned (raw OHLC always is); if they carried different
// leading-NaN counts, the max offset would discard valid leading data.
func applyHLC(high, low, close []float64, lookback int, fn func(h, l, c []float64) []float64) []float64 {
	out := nanSlice(len(close))
	if lookback < 0 { // invalid period -> all NaN, never panic
		return out
	}
	start := maxLeadingNaN(high, low, close)
	if len(close)-start <= lookback {
		return out
	}
	res := fn(high[start:], low[start:], close[start:])
	for i := lookback; i < len(res); i++ {
		out[start+i] = res[i]
	}
	return out
}
