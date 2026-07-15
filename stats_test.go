package backtest

import (
	"errors"
	"math"
	"strings"
	"testing"
	"time"
)

func TestComputeBasics(t *testing.T) {
	// 3 bars, one trade: buy 10 @10 (bar0), mark to 12 (bar2). EquityCurve from a Result.
	d, _ := FromOHLCV(
		[]time.Time{time.Unix(0, 0), time.Unix(86400, 0), time.Unix(172800, 0)},
		[]float64{10, 11, 12}, []float64{10, 11, 12},
		[]float64{10, 11, 12}, []float64{10, 11, 12}, []float64{0, 0, 0})
	r := &Result{
		StartBar: 0,
		EquityCurve: []EquityPoint{
			{Time: d.timeAt(0), Equity: 10000},
			{Time: d.timeAt(1), Equity: 10010},
			{Time: d.timeAt(2), Equity: 10020},
		},
		FinalEquity: 10020,
	}
	s := Compute(r, d, 0)
	if s.EquityFinal != 10020 {
		t.Fatalf("EquityFinal=%v want 10020", s.EquityFinal)
	}
	if s.EquityPeak != 10020 {
		t.Fatalf("EquityPeak=%v want 10020", s.EquityPeak)
	}
	// Return% = (10020/10000 - 1)*100 = 0.2
	if math.Abs(s.ReturnPct-0.2) > 1e-9 {
		t.Fatalf("ReturnPct=%v want 0.2", s.ReturnPct)
	}
	// Buy&Hold = (Close[-1]/Close[0]-1)*100 = (12/10-1)*100 = 20
	if math.Abs(s.BuyHoldReturnPct-20) > 1e-9 {
		t.Fatalf("BuyHoldReturnPct=%v want 20", s.BuyHoldReturnPct)
	}
	if s.NumTrades != 0 {
		t.Fatalf("NumTrades=%d want 0 (no trades in fixture)", s.NumTrades)
	}
}

// TestZeroTradeStatsAreNaN verifies that with zero trades every per-trade
// stat is NaN, matching backtesting.py (max/min/mean of an empty series and
// win_rate with n_trades==0 are all np.nan), instead of the zero-value.
func TestZeroTradeStatsAreNaN(t *testing.T) {
	d, _ := FromOHLCV(
		[]time.Time{time.Unix(0, 0), time.Unix(86400, 0), time.Unix(172800, 0)},
		[]float64{10, 11, 12}, []float64{10, 11, 12},
		[]float64{10, 11, 12}, []float64{10, 11, 12}, []float64{0, 0, 0})
	r := &Result{
		StartBar: 0,
		EquityCurve: []EquityPoint{
			{Time: d.timeAt(0), Equity: 10000},
			{Time: d.timeAt(1), Equity: 10010},
			{Time: d.timeAt(2), Equity: 10020},
		},
		FinalEquity: 10020,
	}
	s := Compute(r, d, 0)
	if s.NumTrades != 0 {
		t.Fatalf("NumTrades=%d want 0 (no trades in fixture)", s.NumTrades)
	}
	cases := map[string]float64{
		"WinRatePct":     s.WinRatePct,
		"BestTradePct":   s.BestTradePct,
		"WorstTradePct":  s.WorstTradePct,
		"AvgTradePct":    s.AvgTradePct,
		"ProfitFactor":   s.ProfitFactor,
		"ExpectancyPct":  s.ExpectancyPct,
		"SQN":            s.SQN,
		"KellyCriterion": s.KellyCriterion,
	}
	for name, v := range cases {
		if !math.IsNaN(v) {
			t.Errorf("%s=%v want NaN (zero trades)", name, v)
		}
	}
}

func TestComputeDrawdown(t *testing.T) {
	// equity [100,120,90,130] → peak 120, dd at 90 = 0.25 → MaxDrawdown -25%
	t0 := time.Unix(0, 0)
	day := 24 * time.Hour
	d, _ := FromOHLCV(
		[]time.Time{t0, t0.Add(day), t0.Add(2 * day), t0.Add(3 * day)},
		[]float64{100, 120, 90, 130},
		[]float64{100, 120, 90, 130},
		[]float64{100, 120, 90, 130},
		[]float64{100, 120, 90, 130},
		[]float64{0, 0, 0, 0},
	)
	r := &Result{
		StartBar: 0,
		EquityCurve: []EquityPoint{
			{Time: t0, Equity: 100},
			{Time: t0.Add(day), Equity: 120},
			{Time: t0.Add(2 * day), Equity: 90},
			{Time: t0.Add(3 * day), Equity: 130},
		},
		FinalEquity: 130,
	}
	s := Compute(r, d, 0)
	if math.Abs(s.MaxDrawdownPct-(-25.0)) > 1e-6 {
		t.Fatalf("MaxDrawdownPct=%v want -25.0", s.MaxDrawdownPct)
	}
	// Only 1 span (peak@120 → peak@130): avg == max
	if math.Abs(s.AvgDrawdownPct-(-25.0)) > 1e-6 {
		t.Fatalf("AvgDrawdownPct=%v want -25.0", s.AvgDrawdownPct)
	}
	if s.MaxDrawdownDur != 2*day {
		t.Fatalf("MaxDrawdownDur=%v want %v", s.MaxDrawdownDur, 2*day)
	}
	if s.AvgDrawdownDur != 2*day {
		t.Fatalf("AvgDrawdownDur=%v want %v", s.AvgDrawdownDur, 2*day)
	}
}

func TestComputeExposure(t *testing.T) {
	// 5-bar dataset. One trade: EntryBar=2, ExitBar=3 → bars {2,3} in position.
	// ExposureTimePct = 2/5 * 100 = 40.  Matches backtesting.py's inclusive
	// [EntryBar, ExitBar] counting over the full data length.
	d, _ := FromOHLCV(
		[]time.Time{time.Unix(0, 0), time.Unix(86400, 0), time.Unix(172800, 0),
			time.Unix(259200, 0), time.Unix(345600, 0)},
		[]float64{10, 11, 12, 13, 14}, []float64{10, 11, 12, 13, 14},
		[]float64{10, 11, 12, 13, 14}, []float64{10, 11, 12, 13, 14},
		[]float64{0, 0, 0, 0, 0})
	r := &Result{
		StartBar: 2,
		EquityCurve: []EquityPoint{
			{Time: d.timeAt(2), Equity: 10000},
			{Time: d.timeAt(3), Equity: 10010},
			{Time: d.timeAt(4), Equity: 10020},
		},
		FinalEquity:    10020,
		InPositionBars: []bool{true, true, false},
		Trades:         []Trade{{EntryBar: 2, ExitBar: 3}},
	}
	s := Compute(r, d, 0)
	// 2 bars in position (2 and 3) / 5 total bars * 100 = 40.0
	if math.Abs(s.ExposureTimePct-40.0) > 1e-9 {
		t.Fatalf("ExposureTimePct=%v want 40.0", s.ExposureTimePct)
	}
}

// TestComputeAnnualisedStats verifies Return(Ann.), Volatility(Ann.) and CAGR
// against hand-computed expected values on a tiny constructed equity curve.
//
// Setup:
//   - 4-bar daily dataset (trading days: Mon-Thu from a known base date).
//   - StartBar = 1 (one warmup bar), equity curve covers bars 1–3.
//   - Equity: [1000, 1010, 1020] (at bars 1, 2, 3).
//   - dayReturns: [1010/1000-1, 1020/1010-1] = [0.01, 0.0099009...]
//   - allRet: prepend StartBar=1 zero → [0, 0.01, 0.0099009...]  (3 values)
//   - g = geometricMeanN(allRet, n=4) = exp(log(1020/1000)/4) - 1
//   - N = 252 (weekday-only trading days → annualTradingDays=252)
//   - Return(Ann.) = ((1+g)^252 - 1) * 100
//   - VolatilityAnn: computed from varR (sample, ddof=1 over 3 values)
//   - CAGR: durDays = bar3.time - bar0.time; timeInYears = durDays / 252
func TestComputeAnnualisedStats(t *testing.T) {
	// 4 consecutive weekdays starting from 2024-01-08 (Mon).
	base := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	times := []time.Time{base, base.Add(day), base.Add(2 * day), base.Add(3 * day)}
	closes := []float64{100, 101, 102, 103}
	d, _ := FromOHLCV(times, closes, closes, closes, closes, []float64{0, 0, 0, 0})
	r := &Result{
		StartBar: 1,
		EquityCurve: []EquityPoint{
			{Time: times[1], Equity: 1000},
			{Time: times[2], Equity: 1010},
			{Time: times[3], Equity: 1020},
		},
		FinalEquity: 1020,
	}
	s := Compute(r, d, 0)

	// g = exp(log(1020/1000)/4) - 1
	n := 4
	N := 252.0
	logRatio := math.Log(1020.0 / 1000.0)
	g := math.Exp(logRatio/float64(n)) - 1
	oneG := 1 + g

	wantRetAnn := (math.Pow(oneG, N) - 1) * 100
	if math.Abs(s.ReturnAnnPct-wantRetAnn) > 1e-9 {
		t.Fatalf("ReturnAnnPct=%v want %v", s.ReturnAnnPct, wantRetAnn)
	}

	// allRet = [0, 0.01, 1020/1010-1] (3 values), sampleVariance ddof=1.
	allRet := []float64{0, 1010.0/1000 - 1, 1020.0/1010 - 1}
	// independent variance (sample, ddof=1) — must NOT call production sampleVariance
	var mean float64
	for _, r := range allRet {
		mean += r
	}
	mean /= float64(len(allRet))
	var ss float64
	for _, r := range allRet {
		ss += (r - mean) * (r - mean)
	}
	varR := ss / float64(len(allRet)-1)
	wantVol := math.Sqrt(math.Pow(varR+oneG*oneG, N)-math.Pow(oneG, 2*N)) * 100
	if math.Abs(s.VolatilityAnnPct-wantVol) > 1e-9 {
		t.Fatalf("VolatilityAnnPct=%v want %v", s.VolatilityAnnPct, wantVol)
	}

	// CAGR: durDays = 3 calendar days (bar0..bar3 inclusive), timeInYears = 3/252.
	durDays := times[3].Sub(times[0]).Hours() / 24 // = 3.0
	timeInYears := durDays / N
	wantCAGR := (math.Pow(1020.0/1000, 1/timeInYears) - 1) * 100
	if math.Abs(s.CAGRPct-wantCAGR) > 1e-9 {
		t.Fatalf("CAGRPct=%v want %v", s.CAGRPct, wantCAGR)
	}
}

// TestComputeAlphaBeta verifies Beta and Alpha [%] on simple constructed series.
//
// Case 1: equity exactly mirrors the market (same log returns) → Beta = 1,
//
//	Alpha = ReturnPct − BuyHoldReturnPct = 0.
//
// Case 2: equity is flat → all equity log returns are 0 → Beta = 0, Alpha = 0.
// Case 3: rfr=5 % with Beta=1 → Alpha still 0 (both sides subtract rfr).
func TestComputeAlphaBeta(t *testing.T) {
	base := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	closes := []float64{100, 105, 98, 110}
	times := []time.Time{base, base.Add(day), base.Add(2 * day), base.Add(3 * day)}
	d, _ := FromOHLCV(times, closes, closes, closes, closes, []float64{0, 0, 0, 0})

	// Case 1: equity = close × 100 → identical log returns → Beta = 1.
	r1 := &Result{
		StartBar: 0,
		EquityCurve: []EquityPoint{
			{Time: times[0], Equity: 10000},
			{Time: times[1], Equity: 10500},
			{Time: times[2], Equity: 9800},
			{Time: times[3], Equity: 11000},
		},
		FinalEquity: 11000,
	}
	s1 := Compute(r1, d, 0)
	if math.Abs(s1.Beta-1.0) > 1e-9 {
		t.Fatalf("case1 Beta=%v want 1.0", s1.Beta)
	}
	// Alpha = ReturnPct − 0 − 1×(BuyHold − 0) = ReturnPct − BuyHold ≈ 0 here.
	wantAlpha1 := s1.ReturnPct - s1.BuyHoldReturnPct
	if math.Abs(s1.AlphaPct-wantAlpha1) > 1e-9 {
		t.Fatalf("case1 AlphaPct=%v want %v", s1.AlphaPct, wantAlpha1)
	}
	if math.Abs(s1.AlphaPct) > 1e-9 {
		t.Fatalf("case1 AlphaPct=%v want ~0 (equity mirrors market)", s1.AlphaPct)
	}

	// Case 2: flat equity → all equity log returns 0 → cov = 0 → Beta = 0.
	r2 := &Result{
		StartBar: 0,
		EquityCurve: []EquityPoint{
			{Time: times[0], Equity: 10000},
			{Time: times[1], Equity: 10000},
			{Time: times[2], Equity: 10000},
			{Time: times[3], Equity: 10000},
		},
		FinalEquity: 10000,
	}
	s2 := Compute(r2, d, 0)
	if math.Abs(s2.Beta) > 1e-9 {
		t.Fatalf("case2 Beta=%v want 0", s2.Beta)
	}
	// ReturnPct=0 and Beta=0 → AlphaPct = 0 − 0 − 0×(BuyHold−0) = 0.
	if math.Abs(s2.AlphaPct) > 1e-9 {
		t.Fatalf("case2 AlphaPct=%v want 0", s2.AlphaPct)
	}

	// Case 3: rfr=5 % (0.05) with equity mirroring market → Beta=1, Alpha=0.
	s3 := Compute(r1, d, 0.05)
	wantAlpha3 := s3.ReturnPct - 0.05*100 - 1.0*(s3.BuyHoldReturnPct-0.05*100)
	if math.Abs(s3.AlphaPct-wantAlpha3) > 1e-9 {
		t.Fatalf("case3 AlphaPct=%v want %v", s3.AlphaPct, wantAlpha3)
	}
	if math.Abs(s3.AlphaPct) > 1e-9 {
		t.Fatalf("case3 AlphaPct=%v want ~0 (equity==market, rfr=5%%)", s3.AlphaPct)
	}
}

// TestComputeRatios verifies Sharpe, Sortino and Calmar on small constructed
// equity curves with hand-computed expected values.
func TestComputeRatios(t *testing.T) {
	base := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	times := []time.Time{base, base.Add(day), base.Add(2 * day), base.Add(3 * day)}
	closes := []float64{100, 101, 102, 103}
	d, _ := FromOHLCV(times, closes, closes, closes, closes, []float64{0, 0, 0, 0})

	// Monotonically rising curve: no drawdown, no negative returns.
	r := &Result{
		StartBar: 1,
		EquityCurve: []EquityPoint{
			{Time: times[1], Equity: 1000},
			{Time: times[2], Equity: 1010},
			{Time: times[3], Equity: 1020},
		},
		FinalEquity: 1020,
	}
	s := Compute(r, d, 0)

	// Sharpe = ReturnAnnPct / VolatilityAnnPct (rfr=0, both in %).
	if s.VolatilityAnnPct != 0 {
		wantSharpe := s.ReturnAnnPct / s.VolatilityAnnPct
		if math.Abs(s.SharpeRatio-wantSharpe) > 1e-9 {
			t.Fatalf("SharpeRatio=%v want %v", s.SharpeRatio, wantSharpe)
		}
	}
	// All-positive allRet → downside deviation = 0 → SortinoRatio is undefined (NaN).
	if !math.IsNaN(s.SortinoRatio) {
		t.Fatalf("SortinoRatio=%v want NaN (no negative returns, downside dev=0)", s.SortinoRatio)
	}
	// No drawdown → CalmarRatio is undefined (NaN).
	if !math.IsNaN(s.CalmarRatio) {
		t.Fatalf("CalmarRatio=%v want NaN (no drawdown, maxDDfrac=0)", s.CalmarRatio)
	}

	// Curve with a dip: [1000, 1020, 990, 1050].
	r2 := &Result{
		StartBar: 0,
		EquityCurve: []EquityPoint{
			{Time: times[0], Equity: 1000},
			{Time: times[1], Equity: 1020},
			{Time: times[2], Equity: 990},
			{Time: times[3], Equity: 1050},
		},
		FinalEquity: 1050,
	}
	s2 := Compute(r2, d, 0)

	// allRet (StartBar=0, no prepended zeros): [0.02, 990/1020-1, 1050/990-1].
	// Only middle value is negative.
	allRet2 := []float64{0.02, 990.0/1020 - 1, 1050.0/990 - 1}
	N := 252.0
	neg := math.Min(allRet2[1], 0)
	meanNegSq := (neg * neg) / 3.0 // sum over 3 values, other two contribute 0
	wantSortino := (s2.ReturnAnnPct / 100) / (math.Sqrt(meanNegSq) * math.Sqrt(N))
	if math.Abs(s2.SortinoRatio-wantSortino) > 1e-9 {
		t.Fatalf("SortinoRatio=%v want %v", s2.SortinoRatio, wantSortino)
	}

	// Calmar: max drawdown fraction = 1 - 990/1020 = 0.029411...
	maxDDfrac := 1 - 990.0/1020
	wantCalmar := (s2.ReturnAnnPct / 100) / maxDDfrac
	if math.Abs(s2.CalmarRatio-wantCalmar) > 1e-9 {
		t.Fatalf("CalmarRatio=%v want %v", s2.CalmarRatio, wantCalmar)
	}
}

// TestComputeTradeStats verifies all trade statistics on a small hand-built
// trades slice. Each expected value is computed from the same formulas used
// by backtesting.py's compute_stats().
//
// 4 trades (PL / ReturnPct):
//
//	t0: PL=−10, ret=−0.05 (loser)
//	t1: PL=−5,  ret=−0.03 (loser)
//	t2: PL=+20, ret=+0.10 (winner)
//	t3: PL=+30, ret=+0.15 (winner)
func TestComputeTradeStats(t *testing.T) {
	base := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour

	// Build a minimal 12-bar daily dataset; only close values matter.
	n := 12
	times := make([]time.Time, n)
	closes := make([]float64, n)
	for i := range times {
		times[i] = base.Add(time.Duration(i) * day)
		closes[i] = 100 + float64(i)
	}
	d, _ := FromOHLCV(times, closes, closes, closes, closes, make([]float64, n))

	// Equity curve: trivially flat for stats computation purposes.
	eq := make([]EquityPoint, n)
	for i := range eq {
		eq[i] = EquityPoint{Time: times[i], Equity: 10000}
	}

	// Four trades, each spanning exactly 2 bars (duration = 2 days).
	trades := []Trade{
		{EntryBar: 0, ExitBar: 2, EntryTime: times[0], ExitTime: times[2], PL: -10, ReturnPct: -0.05},
		{EntryBar: 3, ExitBar: 5, EntryTime: times[3], ExitTime: times[5], PL: -5, ReturnPct: -0.03},
		{EntryBar: 6, ExitBar: 8, EntryTime: times[6], ExitTime: times[8], PL: 20, ReturnPct: 0.10},
		{EntryBar: 9, ExitBar: 11, EntryTime: times[9], ExitTime: times[11], PL: 30, ReturnPct: 0.15},
	}
	r := &Result{
		StartBar:    0,
		EquityCurve: eq,
		FinalEquity: 10000,
		Trades:      trades,
	}
	s := Compute(r, d, 0)

	// Win Rate [%]: (pl > 0).mean() * 100 = 2/4 * 100 = 50.
	if math.Abs(s.WinRatePct-50.0) > 1e-9 {
		t.Fatalf("WinRatePct=%v want 50.0", s.WinRatePct)
	}

	// Best Trade [%]: max(ReturnPct) * 100 = 0.15 * 100 = 15.
	if math.Abs(s.BestTradePct-15.0) > 1e-9 {
		t.Fatalf("BestTradePct=%v want 15.0", s.BestTradePct)
	}

	// Worst Trade [%]: min(ReturnPct) * 100 = −0.05 * 100 = −5.
	if math.Abs(s.WorstTradePct-(-5.0)) > 1e-9 {
		t.Fatalf("WorstTradePct=%v want -5.0", s.WorstTradePct)
	}

	// Avg. Trade [%]: geometric_mean(returns) * 100.
	// returns = [−0.05, −0.03, 0.10, 0.15]
	// factors = [0.95, 0.97, 1.10, 1.15]
	// g = exp((log(0.95)+log(0.97)+log(1.10)+log(1.15))/4) - 1
	logSum := math.Log(0.95) + math.Log(0.97) + math.Log(1.10) + math.Log(1.15)
	wantAvgTrade := (math.Exp(logSum/4) - 1) * 100
	if math.Abs(s.AvgTradePct-wantAvgTrade) > 1e-9 {
		t.Fatalf("AvgTradePct=%v want %v", s.AvgTradePct, wantAvgTrade)
	}

	// Profit Factor: sum(ret>0) / |sum(ret<0)| = (0.10+0.15) / (0.05+0.03) = 0.25/0.08 = 3.125.
	if math.Abs(s.ProfitFactor-3.125) > 1e-9 {
		t.Fatalf("ProfitFactor=%v want 3.125", s.ProfitFactor)
	}

	// Expectancy [%]: arithmetic mean(ReturnPct)*100 = (−0.05−0.03+0.10+0.15)/4*100 = 4.25.
	if math.Abs(s.ExpectancyPct-4.25) > 1e-9 {
		t.Fatalf("ExpectancyPct=%v want 4.25", s.ExpectancyPct)
	}

	// SQN: sqrt(n) * mean(PL) / std(PL, ddof=1).
	// PL = [−10, −5, 20, 30]; mean = 8.75.
	// var = ((−18.75)^2+(−13.75)^2+(11.25)^2+(21.25)^2) / 3
	//      = (351.5625+189.0625+126.5625+451.5625) / 3 = 1118.75/3.
	wantSQN := math.Sqrt(4) * 8.75 / math.Sqrt(1118.75/3)
	if math.Abs(s.SQN-wantSQN) > 1e-9 {
		t.Fatalf("SQN=%v want %v", s.SQN, wantSQN)
	}

	// Kelly Criterion: win_rate − (1−win_rate) / (mean(pl>0) / −mean(pl<0)).
	// win_rate=0.5; mean(pl>0)=(20+30)/2=25; mean(pl<0)=(−10−5)/2=−7.5.
	// Kelly = 0.5 − 0.5 / (25 / 7.5) = 0.5 − 0.5*7.5/25 = 0.5 − 0.15 = 0.35.
	if math.Abs(s.KellyCriterion-0.35) > 1e-9 {
		t.Fatalf("KellyCriterion=%v want 0.35", s.KellyCriterion)
	}

	// Max. Trade Duration: all 4 trades span 2 days; rounded to 1-day period → 2 days.
	if s.MaxTradeDur != 2*day {
		t.Fatalf("MaxTradeDur=%v want %v", s.MaxTradeDur, 2*day)
	}

	// Avg. Trade Duration: mean(2d,2d,2d,2d) = 2d; rounded to period → 2 days.
	if s.AvgTradeDur != 2*day {
		t.Fatalf("AvgTradeDur=%v want %v", s.AvgTradeDur, 2*day)
	}

	// CommissionsTotal: 0 (no commissions set on these trades).
	if s.CommissionsTotal != 0 {
		t.Fatalf("CommissionsTotal=%v want 0", s.CommissionsTotal)
	}
}

// TestComputeTradeCommissionsTotal verifies CommissionsTotal is the sum of
// Trade.Commissions exported from closed trades.
func TestComputeTradeCommissionsTotal(t *testing.T) {
	base := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	n := 4
	times := make([]time.Time, n)
	closes := make([]float64, n)
	for i := range times {
		times[i] = base.Add(time.Duration(i) * day)
		closes[i] = 100
	}
	d, _ := FromOHLCV(times, closes, closes, closes, closes, make([]float64, n))
	eq := []EquityPoint{
		{Time: times[0], Equity: 10000},
		{Time: times[1], Equity: 10000},
		{Time: times[2], Equity: 10000},
		{Time: times[3], Equity: 10000},
	}
	tr1 := &trade{size: 1, entryPrice: 100, exitPrice: 110, entryBar: 0, exitBar: 1,
		entryTime: times[0], exitTime: times[1], entryComm: 1.5, exitComm: 1.5}
	tr2 := &trade{size: 1, entryPrice: 110, exitPrice: 120, entryBar: 2, exitBar: 3,
		entryTime: times[2], exitTime: times[3], entryComm: 2.0, exitComm: 0.5}
	r := &Result{
		StartBar:    0,
		EquityCurve: eq,
		FinalEquity: 10000,
		Trades:      []Trade{tr1.export(), tr2.export()},
	}
	s := Compute(r, d, 0)
	// tr1.commissions() = 3.0, tr2.commissions() = 2.5 → total = 5.5.
	if math.Abs(s.CommissionsTotal-5.5) > 1e-9 {
		t.Fatalf("CommissionsTotal=%v want 5.5", s.CommissionsTotal)
	}
}

// TestDegenerateStatsAreNaN verifies that ratios with an undefined
// denominator render as NaN (roadmap P0.2), matching backtesting.py, instead
// of silently staying at the zero-value.
//
// Setup: a perfectly flat market (zero volatility) and a flat equity curve
// (zero drawdown), plus a single winning trade (no losers → Kelly's mean-loss
// is undefined; a single PL value → std(PL)=0 → SQN is undefined).
func TestDegenerateStatsAreNaN(t *testing.T) {
	base := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	n := 5
	times := make([]time.Time, n)
	closes := make([]float64, n)
	for i := range times {
		times[i] = base.Add(time.Duration(i) * day)
		closes[i] = 100 // flat market → zero volatility, zero Beta denominator
	}
	d, _ := FromOHLCV(times, closes, closes, closes, closes, make([]float64, n))

	eq := make([]EquityPoint, n)
	for i := range eq {
		eq[i] = EquityPoint{Time: times[i], Equity: 10000} // flat equity → zero drawdown
	}

	trades := []Trade{
		{EntryBar: 0, ExitBar: 1, EntryTime: times[0], ExitTime: times[1], PL: 50, ReturnPct: 0.05},
	}
	r := &Result{
		StartBar:    0,
		EquityCurve: eq,
		FinalEquity: 10000,
		Trades:      trades,
	}
	s := Compute(r, d, 0)

	cases := map[string]float64{
		"SharpeRatio":    s.SharpeRatio,
		"SortinoRatio":   s.SortinoRatio,
		"CalmarRatio":    s.CalmarRatio,
		"Beta":           s.Beta,
		"SQN":            s.SQN,
		"KellyCriterion": s.KellyCriterion,
	}
	for name, v := range cases {
		if !math.IsNaN(v) {
			t.Errorf("%s=%v want NaN (degenerate denominator)", name, v)
		}
	}

	// No losing trades → profit factor's denominator is undefined:
	// backtesting.py: pos.sum() / (abs(neg.sum()) or np.nan) → NaN.
	if !math.IsNaN(s.ProfitFactor) {
		t.Errorf("ProfitFactor=%v want NaN (no losing trades)", s.ProfitFactor)
	}
	// The single winning trade still defines the remaining trade stats.
	if s.WinRatePct != 100 {
		t.Errorf("WinRatePct=%v want 100 (single winning trade)", s.WinRatePct)
	}
	defined := map[string]float64{
		"BestTradePct":  s.BestTradePct,
		"WorstTradePct": s.WorstTradePct,
		"AvgTradePct":   s.AvgTradePct,
		"ExpectancyPct": s.ExpectancyPct,
	}
	for name, v := range defined {
		if math.IsNaN(v) {
			t.Errorf("%s=NaN want a real value (one trade exists)", name)
		}
	}
}

// bustStrategy buys a large position on the first bar and holds; combined
// with a subsequent price crash it drives equity to (or below) zero and
// triggers ErrOutOfMoney liquidation.
type bustStrategy struct{ bought bool }

func (s *bustStrategy) Init(st *State) {}
func (s *bustStrategy) Next(st *State) {
	if !s.bought {
		st.Buy(Order{Size: 90})
		s.bought = true
	}
}

// TestFinalEquityZeroOnLiquidation verifies that a run which ends via
// ErrOutOfMoney reports FinalEquity=0 (roadmap P0.2), matching
// backtesting.py's behaviour on ruin, rather than the pre-bust balance.
func TestFinalEquityZeroOnLiquidation(t *testing.T) {
	base := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	times := []time.Time{base, base.Add(day), base.Add(2 * day), base.Add(3 * day)}
	// bar0: price 10, no position yet. bar1: queued order fills at open=10
	// (10x leverage lets the 90-unit order pass the buying-power check);
	// close=10 too, so equity is still ~cash (solvent). bar2: price crashes to
	// 0.01 → the 90-unit long is deeply underwater → equity goes negative →
	// liquidation triggers on this bar.
	opens := []float64{10, 10, 0.01, 0.01}
	highs := []float64{10, 10, 0.01, 0.01}
	lows := []float64{10, 10, 0.01, 0.01}
	closes := []float64{10, 10, 0.01, 0.01}
	vols := []float64{0, 0, 0, 0}
	d, err := FromOHLCV(times, opens, highs, lows, closes, vols)
	if err != nil {
		t.Fatal(err)
	}
	bt := New(d, &bustStrategy{}, Options{Cash: 100, Margin: 0.1})
	res, err := bt.Run()
	if !errors.Is(err, ErrOutOfMoney) {
		t.Fatalf("expected ErrOutOfMoney, got %v", err)
	}
	if res.FinalEquity != 0 {
		t.Fatalf("FinalEquity=%v want 0 on liquidation", res.FinalEquity)
	}
}

// TestRiskFreeRateSharpe exercises the risk-free-rate term in the Sharpe
// Ratio formula (previously untested with a non-zero rfr): Sharpe =
// (ReturnAnn% - rfr*100) / VolatilityAnn%.
func TestRiskFreeRateSharpe(t *testing.T) {
	base := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)
	day := 24 * time.Hour
	times := []time.Time{base, base.Add(day), base.Add(2 * day), base.Add(3 * day)}
	closes := []float64{100, 101, 102, 103}
	d, _ := FromOHLCV(times, closes, closes, closes, closes, []float64{0, 0, 0, 0})
	r := &Result{
		StartBar: 1,
		EquityCurve: []EquityPoint{
			{Time: times[1], Equity: 1000},
			{Time: times[2], Equity: 1010},
			{Time: times[3], Equity: 1020},
		},
		FinalEquity: 1020,
	}
	const rfr = 0.05
	s := Compute(r, d, rfr)
	if s.VolatilityAnnPct == 0 {
		t.Fatal("expected non-zero volatility for this fixture (sanity check)")
	}
	want := (s.ReturnAnnPct - rfr*100) / s.VolatilityAnnPct
	if math.Abs(s.SharpeRatio-want) > 1e-9 {
		t.Fatalf("SharpeRatio=%v want %v", s.SharpeRatio, want)
	}

	// Sanity: a non-zero rfr must actually shift Sharpe from the zero-rfr baseline.
	s0 := Compute(r, d, 0)
	if math.Abs(s.SharpeRatio-s0.SharpeRatio) < 1e-9 {
		t.Fatal("expected SharpeRatio to change with a non-zero risk-free rate")
	}
}

// TestStatsStringRendersNaNAsNA verifies that Stats.String() renders NaN
// metric values as "N/A" (roadmap P0.2) rather than Go's default "NaN" text.
func TestStatsStringRendersNaNAsNA(t *testing.T) {
	s := Stats{
		SharpeRatio:    math.NaN(),
		SortinoRatio:   math.NaN(),
		CalmarRatio:    math.NaN(),
		Beta:           math.NaN(),
		SQN:            math.NaN(),
		KellyCriterion: math.NaN(),
	}
	out := s.String()
	if strings.Contains(out, "NaN") {
		t.Fatalf("String() contains literal \"NaN\":\n%s", out)
	}
	if !strings.Contains(out, "N/A") {
		t.Fatalf("String() does not contain \"N/A\":\n%s", out)
	}
}
