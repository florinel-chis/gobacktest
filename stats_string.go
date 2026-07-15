package backtest

import (
	"fmt"
	"math"
	"strings"
	"time"
)

// fmtStat formats a metric value with format, rendering NaN as "N/A" (a
// degenerate/undefined ratio, e.g. Sharpe when volatility is 0) instead of
// Go's default "NaN" text.
func fmtStat(v float64, format string) string {
	if math.IsNaN(v) {
		return "N/A"
	}
	return fmt.Sprintf(format, v)
}

// String returns a formatted statistics table mirroring backtesting.py's
// print(stats) output: label left-aligned, value right-aligned.
func (s Stats) String() string {
	var b strings.Builder

	row := func(label, value string) {
		fmt.Fprintf(&b, "%-32s %14s\n", label, value)
	}

	fmtDur := func(d time.Duration) string {
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%d days", days)
	}

	row("Start", s.Start.Format("2006-01-02 15:04:05"))
	row("End", s.End.Format("2006-01-02 15:04:05"))
	row("Duration", fmtDur(s.Duration))
	row("Exposure Time [%]", fmtStat(s.ExposureTimePct, "%.2f"))
	row("Equity Final [$]", fmtStat(s.EquityFinal, "%.2f"))
	row("Equity Peak [$]", fmtStat(s.EquityPeak, "%.2f"))
	row("Return [%]", fmtStat(s.ReturnPct, "%.2f"))
	row("Buy & Hold Return [%]", fmtStat(s.BuyHoldReturnPct, "%.2f"))
	row("Return (Ann.) [%]", fmtStat(s.ReturnAnnPct, "%.2f"))
	row("Volatility (Ann.) [%]", fmtStat(s.VolatilityAnnPct, "%.2f"))
	row("CAGR [%]", fmtStat(s.CAGRPct, "%.2f"))
	row("Sharpe Ratio", fmtStat(s.SharpeRatio, "%.3f"))
	row("Sortino Ratio", fmtStat(s.SortinoRatio, "%.3f"))
	row("Calmar Ratio", fmtStat(s.CalmarRatio, "%.3f"))
	row("Alpha [%]", fmtStat(s.AlphaPct, "%.2f"))
	row("Beta", fmtStat(s.Beta, "%.4f"))
	row("Max. Drawdown [%]", fmtStat(s.MaxDrawdownPct, "%.2f"))
	row("Avg. Drawdown [%]", fmtStat(s.AvgDrawdownPct, "%.2f"))
	row("Max. Drawdown Duration", fmtDur(s.MaxDrawdownDur))
	row("Avg. Drawdown Duration", fmtDur(s.AvgDrawdownDur))
	row("# Trades", fmt.Sprintf("%d", s.NumTrades))
	row("Win Rate [%]", fmtStat(s.WinRatePct, "%.2f"))
	row("Best Trade [%]", fmtStat(s.BestTradePct, "%.2f"))
	row("Worst Trade [%]", fmtStat(s.WorstTradePct, "%.2f"))
	row("Avg. Trade [%]", fmtStat(s.AvgTradePct, "%.2f"))
	row("Max. Trade Duration", fmtDur(s.MaxTradeDur))
	row("Avg. Trade Duration", fmtDur(s.AvgTradeDur))
	row("Profit Factor", fmtStat(s.ProfitFactor, "%.3f"))
	row("Expectancy [%]", fmtStat(s.ExpectancyPct, "%.2f"))
	row("SQN", fmtStat(s.SQN, "%.3f"))
	row("Kelly Criterion", fmtStat(s.KellyCriterion, "%.4f"))
	if s.CommissionsTotal != 0 {
		row("Commissions [$]", fmtStat(s.CommissionsTotal, "%.2f"))
	}

	return b.String()
}
