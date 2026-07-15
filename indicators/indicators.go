package indicators

import talib "github.com/markcheno/go-talib"

// RSI (warmup = period).
func RSI(s []float64, period int) []float64 {
	return apply(s, period, func(v []float64) []float64 { return talib.Rsi(v, period) })
}

// ATR (warmup = period).
func ATR(high, low, close []float64, period int) []float64 {
	return applyHLC(high, low, close, period, func(h, l, c []float64) []float64 {
		return talib.Atr(h, l, c, period)
	})
}

// WMA (warmup = period-1).
func WMA(s []float64, period int) []float64 {
	return apply(s, period-1, func(v []float64) []float64 { return talib.Wma(v, period) })
}

// ROC rate-of-change percent (warmup = period).
func ROC(s []float64, period int) []float64 {
	return apply(s, period, func(v []float64) []float64 { return talib.Roc(v, period) })
}

// MACD returns the MACD line, signal line, and histogram (warmup = slow+signal-2).
func MACD(s []float64, fast, slow, signal int) (macd, sig, hist []float64) {
	lookback := slow + signal - 2
	macd = nanSlice(len(s))
	sig = nanSlice(len(s))
	hist = nanSlice(len(s))
	start := leadingNaN(s)
	if len(s)-start <= lookback {
		return
	}
	rm, rs, rh := talib.Macd(s[start:], fast, slow, signal)
	for i := lookback; i < len(rm); i++ {
		macd[start+i] = rm[i]
		sig[start+i] = rs[i]
		hist[start+i] = rh[i]
	}
	return
}

// Bollinger returns upper, middle, lower bands (warmup = period-1).
func Bollinger(s []float64, period int, dev float64) (upper, mid, lower []float64) {
	upper = nanSlice(len(s))
	mid = nanSlice(len(s))
	lower = nanSlice(len(s))
	start := leadingNaN(s)
	lookback := period - 1
	if len(s)-start <= lookback {
		return
	}
	ru, rm, rl := talib.BBands(s[start:], period, dev, dev, talib.SMA)
	for i := lookback; i < len(ru); i++ {
		upper[start+i] = ru[i]
		mid[start+i] = rm[i]
		lower[start+i] = rl[i]
	}
	return
}

// SMA is the simple moving average (warmup period-1).
func SMA(s []float64, period int) []float64 {
	return apply(s, period-1, func(v []float64) []float64 { return talib.Sma(v, period) })
}

// EMA is the exponential moving average (warmup period-1). Accepts a NaN-prefixed
// input so it composes (e.g. EMA(WilliamsR(...), 9)).
func EMA(s []float64, period int) []float64 {
	return apply(s, period-1, func(v []float64) []float64 { return talib.Ema(v, period) })
}

// WilliamsR is the Williams %R oscillator in [-100, 0] (warmup period-1).
func WilliamsR(high, low, close []float64, period int) []float64 {
	return applyHLC(high, low, close, period-1, func(h, l, c []float64) []float64 {
		return talib.WillR(h, l, c, period)
	})
}

// Stochastic returns the slow %K and %D lines. Lookback = (fastK-1)+(slowK-1)+(slowD-1).
func Stochastic(high, low, close []float64, fastK, slowK, slowD int) (k, d []float64) {
	lookback := (fastK - 1) + (slowK - 1) + (slowD - 1)
	out := nanSlice(len(close))
	dOut := nanSlice(len(close))
	start := maxLeadingNaN(high, low, close)
	if len(close)-start <= lookback {
		return out, dOut
	}
	rk, rd := talib.Stoch(high[start:], low[start:], close[start:], fastK, slowK, talib.SMA, slowD, talib.SMA)
	for i := lookback; i < len(rk); i++ {
		out[start+i] = rk[i]
		dOut[start+i] = rd[i]
	}
	return out, dOut
}

// ADX is the Average Directional Movement Index in [0, 100]. It is
// double-smoothed (DM smoothing, then DX smoothing), so its warmup is
// 2*period-1, not period-1 like a single-smoothed indicator.
func ADX(high, low, close []float64, period int) []float64 {
	return applyHLC(high, low, close, 2*period-1, func(h, l, c []float64) []float64 {
		return talib.Adx(h, l, c, period)
	})
}

// CCI is the Commodity Channel Index (warmup = period-1). Unbounded, so no
// range assertion applies to it.
func CCI(high, low, close []float64, period int) []float64 {
	return applyHLC(high, low, close, period-1, func(h, l, c []float64) []float64 {
		return talib.Cci(h, l, c, period)
	})
}

// OBV is On-Balance Volume, a cumulative running total defined from the
// first bar (lookback 0). close and volume MUST be length-aligned.
func OBV(close, volume []float64) []float64 {
	out := nanSlice(len(close))
	start := maxLeadingNaN(close, volume)
	if len(close)-start < 1 {
		return out
	}
	res := talib.Obv(close[start:], volume[start:])
	for i := 0; i < len(res); i++ {
		out[start+i] = res[i]
	}
	return out
}

// Donchian returns the upper, mid, and lower Donchian channel bands
// (warmup = period-1). upper = highest high over period, lower = lowest low
// over period, mid = midpoint of the two.
func Donchian(high, low []float64, period int) (upper, mid, lower []float64) {
	upper = nanSlice(len(high))
	mid = nanSlice(len(high))
	lower = nanSlice(len(high))
	start := maxLeadingNaN(high, low)
	lookback := period - 1
	if len(high)-start <= lookback {
		return
	}
	ru := talib.Max(high[start:], period)
	rl := talib.Min(low[start:], period)
	for i := lookback; i < len(ru); i++ {
		upper[start+i] = ru[i]
		lower[start+i] = rl[i]
		mid[start+i] = (ru[i] + rl[i]) / 2
	}
	return
}
