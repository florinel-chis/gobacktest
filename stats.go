package backtest

import (
	"math"
	"time"
)

// Stats holds the computed performance metrics for a backtest run. Fields are
// added across Plan 3 tasks; unimplemented fields stay zero until their task.
type Stats struct {
	Start            time.Time
	End              time.Time
	Duration         time.Duration
	ExposureTimePct  float64
	EquityFinal      float64
	EquityPeak       float64
	ReturnPct        float64
	BuyHoldReturnPct float64
	NumTrades        int
	// --- added by later tasks (declared now for a stable struct) ---
	ReturnAnnPct     float64
	VolatilityAnnPct float64
	CAGRPct          float64
	SharpeRatio      float64
	SortinoRatio     float64
	CalmarRatio      float64
	AlphaPct         float64
	Beta             float64
	MaxDrawdownPct   float64
	AvgDrawdownPct   float64
	MaxDrawdownDur   time.Duration
	AvgDrawdownDur   time.Duration
	WinRatePct       float64
	BestTradePct     float64
	WorstTradePct    float64
	AvgTradePct      float64
	MaxTradeDur      time.Duration
	AvgTradeDur      time.Duration
	ProfitFactor     float64
	ExpectancyPct    float64
	SQN              float64
	KellyCriterion   float64
	CommissionsTotal float64
}

// Compute derives statistics from a run Result and its Data. riskFreeRate is
// the per-annum rate used by Sharpe/Sortino/Alpha (default 0).
func Compute(r *Result, d *Data, riskFreeRate float64) Stats {
	var s Stats
	if len(r.EquityCurve) == 0 {
		return s
	}
	// Start/End/Duration span the FULL data index (incl. indicator warmup), like
	// backtesting.py — not the equity-curve window which begins at StartBar.
	s.Start = d.timeAt(0)
	s.End = d.timeAt(d.fullLen() - 1)
	s.Duration = s.End.Sub(s.Start)

	// eq0 is equity at StartBar; it equals backtesting.py's equity[0] (initial cash)
	// because no position can open during indicator warmup, so warmup equity is flat.
	eq0 := r.EquityCurve[0].Equity
	s.EquityFinal = r.FinalEquity
	s.EquityPeak = r.EquityCurve[0].Equity
	for _, p := range r.EquityCurve[1:] {
		if p.Equity > s.EquityPeak {
			s.EquityPeak = p.Equity
		}
	}
	if eq0 != 0 {
		s.ReturnPct = (s.EquityFinal/eq0 - 1) * 100
	}
	// Buy & Hold over the same bar window [StartBar, end].
	first := d.closeAt(r.StartBar)
	last := d.closeAt(d.fullLen() - 1)
	if first != 0 {
		s.BuyHoldReturnPct = (last/first - 1) * 100
	}
	s.NumTrades = len(r.Trades)

	// Exposure Time: fraction of ALL bars (incl. warmup) where a position was open.
	// backtesting.py counts bars [EntryBar, ExitBar] inclusive for each closed trade
	// and denominates over the full data length (d.fullLen()).  This matches its
	// "Exposure Time [%]" stat exactly.
	totalBars := d.fullLen()
	if totalBars > 0 && len(r.Trades) > 0 {
		inPos := make(map[int]struct{}, totalBars/2)
		for _, t := range r.Trades {
			for bar := t.EntryBar; bar <= t.ExitBar; bar++ {
				inPos[bar] = struct{}{}
			}
		}
		s.ExposureTimePct = float64(len(inPos)) / float64(totalBars) * 100
	}

	// Drawdown statistics.
	equityVals := make([]float64, len(r.EquityCurve))
	for i, p := range r.EquityCurve {
		equityVals[i] = p.Equity
	}
	ddSeries := drawdownSeries(equityVals)
	maxDD := 0.0
	for _, dv := range ddSeries {
		if dv > maxDD {
			maxDD = dv
		}
	}
	s.MaxDrawdownPct = -maxDD * 100

	ddDurs, ddPeaks := drawdownDurationsAndPeaks(r.EquityCurve)
	if len(ddDurs) > 0 {
		var totalDur time.Duration
		for _, dur := range ddDurs {
			if dur > s.MaxDrawdownDur {
				s.MaxDrawdownDur = dur
			}
			totalDur += dur
		}
		s.AvgDrawdownDur = totalDur / time.Duration(len(ddDurs))
	}
	if len(ddPeaks) > 0 {
		sum := 0.0
		for _, p := range ddPeaks {
			sum += p
		}
		s.AvgDrawdownPct = -(sum / float64(len(ddPeaks))) * 100
	}

	// Round duration stats up to data period (matching backtesting.py's _round_timedelta).
	var period time.Duration
	if d.fullLen() >= 2 {
		period = d.timeAt(1).Sub(d.timeAt(0))
	}
	s.MaxDrawdownDur = roundUpToPeriod(s.MaxDrawdownDur, period)
	s.AvgDrawdownDur = roundUpToPeriod(s.AvgDrawdownDur, period)

	// --- Task 4: annualised return, volatility, CAGR ---
	//
	// backtesting.py's day_returns is a pandas Series of length d.fullLen():
	//   [NaN, equity[1]/equity[0]-1, ..., equity[n-1]/equity[n-2]-1]
	// geometric_mean receives it and does fillna(0) before logging, so len(returns)
	// = d.fullLen() (the NaN at position 0 counts). The warmup bars (0..StartBar-1)
	// all have flat equity → their returns are 0 and contribute 0 to the log-sum.
	// We replicate this by:
	//   1. Computing dayReturns from the equity curve (len(curve)-1 values).
	//   2. Prepending r.StartBar zeros (warmup zero-returns).
	//   3. Dividing the log-sum by d.fullLen() (= len(allRet)+1 matching Python).
	//
	// This gives allRet with d.fullLen()-1 non-NaN elements used for variance and
	// Sortino (pandas var/mean with skipna=True use exactly these d.fullLen()-1 values).

	n := d.fullLen()
	// Build the full data time-index for the annualisation factor.
	allTimes := make([]time.Time, n)
	for i := range allTimes {
		allTimes[i] = d.timeAt(i)
	}
	N := annualTradingDays(allTimes)

	// Consecutive pct-changes of the equity curve.
	dr := dayReturns(r.EquityCurve)
	// Full return series: warmup zeros + equity-curve returns. Length = n-1.
	allRet := make([]float64, r.StartBar+len(dr)) // leading zeros from make
	copy(allRet[r.StartBar:], dr)

	// Geometric mean divides by d.fullLen() (n), not len(allRet) (n-1).
	g := geometricMeanN(allRet, n)
	oneG := 1 + g

	// Return (Ann.) [%] = ((1+g)^N - 1) * 100
	s.ReturnAnnPct = (math.Pow(oneG, N) - 1) * 100

	// Volatility (Ann.) [%]: exact compounded formula matching backtesting.py.
	// Variance uses ddof=1 (sample) over allRet (d.fullLen()-1 elements), matching
	// pandas Series.var(ddof=1, skipna=True) which skips the 1 NaN → same n_eff.
	varR := sampleVariance(allRet)
	s.VolatilityAnnPct = math.Sqrt(math.Pow(varR+oneG*oneG, N)-math.Pow(oneG, 2*N)) * 100

	// CAGR [%]: (EquityFinal/EquityStart)^(1/time_in_years) - 1) * 100
	// backtesting.py time_in_years = (End-Start).days / annual_trading_days (NOT /365.25).
	fullStart := d.timeAt(0)
	fullEnd := d.timeAt(n - 1)
	durDays := fullEnd.Sub(fullStart).Hours() / 24
	timeInYears := durDays / N
	if timeInYears > 0 && eq0 > 0 {
		s.CAGRPct = (math.Pow(s.EquityFinal/eq0, 1/timeInYears) - 1) * 100
	}

	// --- Task 5: Sharpe, Sortino, Calmar ratios ---

	// Sharpe Ratio: both Return(Ann.) and Volatility(Ann.) are in %; the % units
	// cancel so the ratio equals decimal/decimal. Matches backtesting.py exactly:
	//   (Return(Ann.)[%] - rfr*100) / Volatility(Ann.)[%]
	if s.VolatilityAnnPct != 0 {
		s.SharpeRatio = (s.ReturnAnnPct - riskFreeRate*100) / s.VolatilityAnnPct
	} else {
		s.SharpeRatio = math.NaN()
	}

	// Sortino Ratio: downside deviation = sqrt(mean(min(r,0)^2)) * sqrt(N).
	// backtesting.py uses np.mean(day_returns.clip(-inf,0)**2) where np.mean via
	// pandas dispatch applies skipna=True → divides by len(allRet) = n-1, same as
	// our explicit loop. Warmup zero-returns contribute 0 to sum but raise the
	// denominator, matching Python's treatment of those bars.
	if len(allRet) > 0 {
		sumNegSq := 0.0
		for _, ret := range allRet {
			neg := math.Min(ret, 0)
			sumNegSq += neg * neg
		}
		meanNegSq := sumNegSq / float64(len(allRet))
		downsideDev := math.Sqrt(meanNegSq) * math.Sqrt(N)
		if downsideDev > 0 {
			s.SortinoRatio = (s.ReturnAnnPct/100 - riskFreeRate) / downsideDev
		} else {
			s.SortinoRatio = math.NaN()
		}
	} else {
		s.SortinoRatio = math.NaN()
	}

	// Calmar Ratio = annualized_return / max_drawdown_fraction (both positive).
	// backtesting.py: annualized_return / (-max_dd or nan) where max_dd = -dd.max().
	// MaxDrawdownPct is stored negative (e.g. -0.596%); divide by -100 → positive frac.
	maxDDfrac := -s.MaxDrawdownPct / 100
	if maxDDfrac > 0 {
		s.CalmarRatio = (s.ReturnAnnPct / 100) / maxDDfrac
	} else {
		s.CalmarRatio = math.NaN()
	}

	// --- Task 6: Beta and Alpha [%] ---
	//
	// backtesting.py:
	//   equity_log_returns = np.log(equity[1:] / equity[:-1])   # length n-1
	//   market_log_returns = np.log(c[1:] / c[:-1])             # length n-1
	//   cov_matrix = np.cov(equity_log_returns, market_log_returns)  # ddof=1
	//   beta = cov_matrix[0,1] / cov_matrix[1,1]
	//   alpha = Return[%] - rfr*100 - beta*(BuyHold[%] - rfr*100)
	//
	// Go's equity curve starts at r.StartBar; warmup bars (0..StartBar-1) have
	// flat equity → their log-returns are 0, matching Python's equity[0..StartBar].
	equityLogRet := make([]float64, n-1) // leading zeros = warmup zeros
	for i := 1; i < len(r.EquityCurve); i++ {
		prev := r.EquityCurve[i-1].Equity
		curr := r.EquityCurve[i].Equity
		if prev > 0 {
			equityLogRet[r.StartBar+i-1] = math.Log(curr / prev)
		}
	}
	marketLogRet := make([]float64, n-1)
	for i := 1; i < n; i++ {
		prev := d.closeAt(i - 1)
		curr := d.closeAt(i)
		if prev > 0 {
			marketLogRet[i-1] = math.Log(curr / prev)
		}
	}
	if len(equityLogRet) > 1 && len(marketLogRet) > 1 {
		cov := sampleCovariance(equityLogRet, marketLogRet)
		varMarket := sampleVariance(marketLogRet)
		if varMarket != 0 {
			s.Beta = cov / varMarket
		} else {
			s.Beta = math.NaN()
		}
	} else {
		s.Beta = math.NaN()
	}
	// Jensen CAPM Alpha.
	s.AlphaPct = s.ReturnPct - riskFreeRate*100 - s.Beta*(s.BuyHoldReturnPct-riskFreeRate*100)

	// --- Task 7: Trade statistics ---
	//
	// All formulas mirror backtesting.py's compute_stats() exactly:
	//   pl      = trades_df['PnL']
	//   returns = trades_df['ReturnPct']
	//
	// All trade stats are undefined (NaN) when there are no trades — in
	// backtesting.py they are max/min/mean of an empty series, or explicitly
	// np.nan — and ProfitFactor additionally stays NaN when there are no
	// losing trades (denominator `abs(neg.sum()) or np.nan`). Each field is
	// overwritten below only when its formula is defined.
	s.WinRatePct = math.NaN()
	s.BestTradePct = math.NaN()
	s.WorstTradePct = math.NaN()
	s.AvgTradePct = math.NaN()
	s.ProfitFactor = math.NaN()
	s.ExpectancyPct = math.NaN()
	s.SQN = math.NaN()
	s.KellyCriterion = math.NaN()
	if s.NumTrades > 0 {
		pls := make([]float64, s.NumTrades)
		rets := make([]float64, s.NumTrades)
		for i, t := range r.Trades {
			pls[i] = t.PL
			rets[i] = t.ReturnPct
		}

		// Win Rate [%]: (pl > 0).mean() * 100.
		winCount := 0
		for _, pl := range pls {
			if pl > 0 {
				winCount++
			}
		}
		winRate := float64(winCount) / float64(s.NumTrades)
		s.WinRatePct = winRate * 100

		// Best / Worst Trade [%]: max/min(ReturnPct) * 100.
		best, worst := rets[0], rets[0]
		for _, r := range rets[1:] {
			if r > best {
				best = r
			}
			if r < worst {
				worst = r
			}
		}
		s.BestTradePct = best * 100
		s.WorstTradePct = worst * 100

		// Avg. Trade [%]: geometric_mean(ReturnPct) * 100 (matches Python's geometric_mean).
		s.AvgTradePct = geometricMean(rets) * 100

		// Max. / Avg. Trade Duration: ExitTime − EntryTime, ceil'd to data period.
		var maxTradeDur, totalTradeDur time.Duration
		for _, t := range r.Trades {
			dur := t.ExitTime.Sub(t.EntryTime)
			if dur > maxTradeDur {
				maxTradeDur = dur
			}
			totalTradeDur += dur
		}
		avgTradeDur := totalTradeDur / time.Duration(s.NumTrades)
		s.MaxTradeDur = roundUpToPeriod(maxTradeDur, period)
		s.AvgTradeDur = roundUpToPeriod(avgTradeDur, period)

		// Profit Factor: sum(ret > 0) / |sum(ret < 0)|.
		// backtesting.py: returns[returns > 0].sum() / abs(returns[returns < 0].sum())
		var sumPosRet, sumNegRet float64
		for _, r := range rets {
			if r > 0 {
				sumPosRet += r
			} else if r < 0 {
				sumNegRet += r
			}
		}
		if sumNegRet != 0 {
			s.ProfitFactor = sumPosRet / math.Abs(sumNegRet)
		} // else: no losing trades → ProfitFactor stays NaN (set above)

		// Expectancy [%]: arithmetic mean(ReturnPct) * 100.
		// backtesting.py: returns.mean() * 100 (NOT geometric).
		var sumRets float64
		for _, r := range rets {
			sumRets += r
		}
		s.ExpectancyPct = sumRets / float64(s.NumTrades) * 100

		// SQN: sqrt(n) * mean(PL) / std(PL, ddof=1).
		// backtesting.py: pl.std() is pandas default ddof=1.
		var sumPL float64
		for _, pl := range pls {
			sumPL += pl
		}
		meanPL := sumPL / float64(s.NumTrades)
		stdPL := math.Sqrt(sampleVariance(pls))
		if stdPL != 0 {
			s.SQN = math.Sqrt(float64(s.NumTrades)) * meanPL / stdPL
		} // else: std(PL)==0 → SQN stays NaN (set above)

		// Kelly Criterion: win_rate − (1−win_rate) / (mean(pl>0) / −mean(pl<0)).
		// backtesting.py uses PL (not ReturnPct) for the win/loss means.
		var sumWinPL, sumLosePL float64
		nWin, nLose := 0, 0
		for _, pl := range pls {
			if pl > 0 {
				sumWinPL += pl
				nWin++
			} else if pl < 0 {
				sumLosePL += pl // negative accumulator
				nLose++
			}
		}
		if nWin > 0 && nLose > 0 {
			meanWin := sumWinPL / float64(nWin)
			meanLoseAbs := -sumLosePL / float64(nLose) // positive
			if meanWin > 0 && meanLoseAbs > 0 {
				s.KellyCriterion = winRate - (1-winRate)/(meanWin/meanLoseAbs)
			}
		} // else: no losing trades → mean-loss undefined → KellyCriterion stays NaN (set above)
	}

	// CommissionsTotal: sum of Commissions across all closed trades.
	for _, t := range r.Trades {
		s.CommissionsTotal += t.Commissions
	}

	return s
}
