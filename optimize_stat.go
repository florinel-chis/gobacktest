package backtest

// statValue extracts the named statistic from s, returning (value, true) when
// the name is recognised or (0, false) otherwise. Names match the labels used by
// Stats.String() (i.e. backtesting.py parity keys).
func statValue(s Stats, name string) (float64, bool) {
	switch name {
	case "SQN":
		return s.SQN, true
	case "Sharpe Ratio":
		return s.SharpeRatio, true
	case "Sortino Ratio":
		return s.SortinoRatio, true
	case "Calmar Ratio":
		return s.CalmarRatio, true
	case "Return [%]":
		return s.ReturnPct, true
	case "Equity Final [$]":
		return s.EquityFinal, true
	case "Profit Factor":
		return s.ProfitFactor, true
	case "Win Rate [%]":
		return s.WinRatePct, true
	case "Expectancy [%]":
		return s.ExpectancyPct, true
	case "Max. Drawdown [%]":
		return s.MaxDrawdownPct, true
	default:
		return 0, false
	}
}
