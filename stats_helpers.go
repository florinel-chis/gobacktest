package backtest

import (
	"math"
	"sort"
	"time"
)

// dayReturns computes consecutive percentage changes from an equity curve.
// Returns a slice of length len(curve)-1:
//
//	dayReturns[i] = curve[i+1].Equity/curve[i].Equity - 1.
//
// For daily bars this is the per-bar return. Used by annualised-stat helpers.
func dayReturns(curve []EquityPoint) []float64 {
	if len(curve) < 2 {
		return nil
	}
	r := make([]float64, len(curve)-1)
	for i := 1; i < len(curve); i++ {
		prev := curve[i-1].Equity
		if prev == 0 {
			r[i-1] = 0
		} else {
			r[i-1] = curve[i].Equity/prev - 1
		}
	}
	return r
}

// geometricMeanN is like geometricMean but divides the log-sum by n rather
// than by len(returns). Use this when the logical series length (e.g. including
// implicit leading zero-returns from warmup bars) exceeds len(returns).
// backtesting.py's geometric_mean receives day_returns which includes a leading
// NaN (filled with 0), so its len(returns) = d.fullLen() rather than
// d.fullLen()-1. Pass n = d.fullLen() to match.
func geometricMeanN(returns []float64, n int) float64 {
	if n <= 0 || len(returns) == 0 {
		return 0
	}
	sum := 0.0
	for _, r := range returns {
		f := 1 + r
		if f <= 0 {
			return 0
		}
		sum += math.Log(f)
	}
	return math.Exp(sum/float64(n)) - 1
}

// sampleVariance computes the sample variance (ddof=1) of data.
// Matches pandas Series.var(ddof=1, skipna=True) when all elements are non-NaN.
// Returns 0 if len(data) < 2.
func sampleVariance(data []float64) float64 {
	n := len(data)
	if n < 2 {
		return 0
	}
	mean := 0.0
	for _, v := range data {
		mean += v
	}
	mean /= float64(n)
	ss := 0.0
	for _, v := range data {
		d := v - mean
		ss += d * d
	}
	return ss / float64(n-1)
}

// drawdownSeries returns the fractional drawdown at each equity sample.
// dd[i] = 1 - equity[i]/runningMax(equity[:i+1]).
// Values are in [0,1]; 0 means equity is at or above the prior peak.
func drawdownSeries(equity []float64) []float64 {
	dd := make([]float64, len(equity))
	runMax := math.Inf(-1)
	for i, e := range equity {
		if e > runMax {
			runMax = e
		}
		if runMax > 0 {
			dd[i] = 1 - e/runMax
		}
	}
	return dd
}

// drawdownDurationsAndPeaks identifies drawdown spans from the equity curve.
// A span runs from one high-water mark (HWM) until equity reaches a new HWM
// or end-of-data (final unrecovered drawdown). For each span, the function
// returns its calendar duration and its peak (maximum) drawdown fraction.
// Spans with zero drawdown are omitted.
func drawdownDurationsAndPeaks(curve []EquityPoint) (durations []time.Duration, peaks []float64) {
	if len(curve) == 0 {
		return
	}

	// Build the drawdown series.
	equity := make([]float64, len(curve))
	for i, p := range curve {
		equity[i] = p.Equity
	}
	dd := drawdownSeries(equity)

	// Locate all HWM indices (strictly new highs).
	runMax := math.Inf(-1)
	var hwm []int
	for i, e := range equity {
		if e > runMax {
			runMax = e
			hwm = append(hwm, i)
		}
	}
	if len(hwm) == 0 {
		return
	}

	// For each span [hwm[j], hwm[j+1]] (or [hwm[last], len-1]), find the peak
	// drawdown and the calendar duration.
	for j := 0; j < len(hwm); j++ {
		start := hwm[j]
		end := len(curve) - 1
		if j+1 < len(hwm) {
			end = hwm[j+1]
		}
		if end <= start {
			continue
		}

		maxDD := 0.0
		for k := start; k <= end; k++ {
			if dd[k] > maxDD {
				maxDD = dd[k]
			}
		}
		if maxDD > 0 {
			durations = append(durations, curve[end].Time.Sub(curve[start].Time))
			peaks = append(peaks, maxDD)
		}
	}
	return
}

func geometricMean(returns []float64) float64 {
	if len(returns) == 0 {
		return 0
	}
	sum := 0.0
	for _, r := range returns {
		f := 1 + r
		if f <= 0 {
			return 0
		}
		sum += math.Log(f)
	}
	return math.Exp(sum/float64(len(returns))) - 1
}

// sampleCovariance computes the sample covariance (ddof=1) of two equal-length slices.
// Matches np.cov(x, y)[0,1] which uses ddof=1 by default. Returns 0 if n < 2.
func sampleCovariance(x, y []float64) float64 {
	n := len(x)
	if n < 2 || len(y) != n {
		return 0
	}
	meanX, meanY := 0.0, 0.0
	for i := range x {
		meanX += x[i]
		meanY += y[i]
	}
	meanX /= float64(n)
	meanY /= float64(n)
	sum := 0.0
	for i := range x {
		sum += (x[i] - meanX) * (y[i] - meanY)
	}
	return sum / float64(n-1)
}

// roundUpToPeriod ceils a duration up to a whole multiple of period (replicating
// backtesting.py's _round_timedelta). period<=0 or d<=0 returns d unchanged.
func roundUpToPeriod(d, period time.Duration) time.Duration {
	if period <= 0 || d <= 0 {
		return d
	}
	n := (d + period - 1) / period
	return n * period
}

// annualTradingDays infers the annualization factor from the index spacing.
func annualTradingDays(index []time.Time) float64 {
	if len(index) < 2 {
		return 252
	}
	// Median period in days over the last up-to-100 index points, replicating
	// backtesting.py's _data_period: pd.Series(index[-100:]).diff().median().
	// The median makes a single anomalous gap (e.g. a leading holiday week in
	// a daily series) harmless.
	tail := index
	if len(tail) > 100 {
		tail = tail[len(tail)-100:]
	}
	diffs := make([]float64, len(tail)-1)
	for i := 1; i < len(tail); i++ {
		diffs[i-1] = tail[i].Sub(tail[i-1]).Hours() / 24
	}
	sort.Float64s(diffs)
	var periodDays float64
	if n := len(diffs); n%2 == 1 {
		periodDays = diffs[n/2]
	} else {
		periodDays = (diffs[n/2-1] + diffs[n/2]) / 2
	}
	switch {
	case periodDays >= 6 && periodDays <= 8:
		return 52
	case periodDays >= 28 && periodDays <= 31:
		return 12
	case periodDays >= 360:
		return 1
	}
	weekend := 0
	for _, t := range index {
		if wd := t.Weekday(); wd == time.Saturday || wd == time.Sunday {
			weekend++
		}
	}
	if float64(weekend)/float64(len(index)) > 2.0/7.0*0.6 {
		return 365
	}
	return 252
}
