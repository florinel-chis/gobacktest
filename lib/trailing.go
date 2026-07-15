package lib

import (
	"math"

	backtest "github.com/florinel-chis/gobacktest"
)

// TrailingATR computes backtesting.py TrailingStrategy's ATR: the SIMPLE moving
// average of true range over `period`, backfilled over the warmup (NOT Wilder's
// ATR — do not use indicators.ATR here). Returns a full-length []float64.
//
// Matches backtesting.py exactly:
//
//	hi, lo, c_prev = data.High, data.Low, pd.Series(data.Close).shift(1)   # c_prev[0] = NaN
//	tr = np.max([hi-lo, (c_prev-hi).abs(), (c_prev-lo).abs()], axis=0)      # tr[0] = NaN
//	atr = pd.Series(tr).rolling(period).mean().bfill().values
//
// tr[0] is NaN (no previous close), so any rolling window containing index 0
// (i.e. every window ending before index == period) is NaN. The first valid
// ATR value is therefore at index == period (window [1, period]), and bfill
// back-fills indices [0, period-1] with that first valid value.
func TrailingATR(high, low, close []float64, period int) []float64 {
	n := len(high)
	tr := make([]float64, n)
	if n > 0 {
		tr[0] = math.NaN()
	}
	for i := 1; i < n; i++ {
		cPrev := close[i-1]
		hi, lo := high[i], low[i]
		tr[i] = max3(hi-lo, math.Abs(cPrev-hi), math.Abs(cPrev-lo))
	}

	atr := make([]float64, n)
	for i := 0; i < n; i++ {
		if i < period-1 {
			atr[i] = math.NaN()
			continue
		}
		sum := 0.0
		valid := true
		for j := i - period + 1; j <= i; j++ {
			if math.IsNaN(tr[j]) {
				valid = false
				break
			}
			sum += tr[j]
		}
		if valid {
			atr[i] = sum / float64(period)
		} else {
			atr[i] = math.NaN()
		}
	}

	// bfill: back-fill leading NaNs with the first valid value.
	firstValid := -1
	for i := 0; i < n; i++ {
		if !math.IsNaN(atr[i]) {
			firstValid = i
			break
		}
	}
	if firstValid > 0 {
		for i := 0; i < firstValid; i++ {
			atr[i] = atr[firstValid]
		}
	}
	return atr
}

func max3(a, b, c float64) float64 {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	return m
}

// TrailStop trails each open trade's stop-loss to (close - nATR*atr) for longs
// and (close + nATR*atr) for shorts, only ever TIGHTENING (max for long, min for
// short). `atr` is the CURRENT bar's TrailingATR value; call it in Next().
func TrailStop(s *backtest.State, atr, nATR float64) {
	close := s.Data().Close().Last()
	for _, t := range s.OpenTrades() {
		switch {
		case t.IsLong():
			newSL := close - nATR*atr
			if t.SL() == 0 {
				t.SetSL(newSL)
			} else {
				t.SetSL(math.Max(t.SL(), newSL))
			}
		case t.IsShort():
			newSL := close + nATR*atr
			if t.SL() == 0 {
				t.SetSL(newSL)
			} else {
				t.SetSL(math.Min(t.SL(), newSL))
			}
		}
	}
}
