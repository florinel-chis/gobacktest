package backtest

import (
	"strings"
	"testing"
	"time"
)

// TestStatsString verifies Stats.String() contains key formatted lines.
func TestStatsString(t *testing.T) {
	s := Stats{
		Start:            time.Date(2018, 1, 2, 0, 0, 0, 0, time.UTC),
		End:              time.Date(2018, 12, 28, 0, 0, 0, 0, time.UTC),
		Duration:         360 * 24 * time.Hour,
		ExposureTimePct:  49.60,
		EquityFinal:      10004.10,
		EquityPeak:       10052.12,
		ReturnPct:        0.04,
		BuyHoldReturnPct: -5.03,
		NumTrades:        5,
		WinRatePct:       40.00,
		SharpeRatio:      0.058,
		CommissionsTotal: 12.50,
		MaxDrawdownDur:   158 * 24 * time.Hour,
		AvgDrawdownDur:   74 * 24 * time.Hour,
		MaxTradeDur:      69 * 24 * time.Hour,
		AvgTradeDur:      34 * 24 * time.Hour,
	}
	out := s.String()

	// Verify all expected labels are present.
	labels := []string{
		"Start", "End", "Duration",
		"Exposure Time [%]", "Equity Final [$]", "Equity Peak [$]",
		"Return [%]", "Buy & Hold Return [%]",
		"Return (Ann.) [%]", "Volatility (Ann.) [%]", "CAGR [%]",
		"Sharpe Ratio", "Sortino Ratio", "Calmar Ratio",
		"Alpha [%]", "Beta",
		"Max. Drawdown [%]", "Avg. Drawdown [%]",
		"Max. Drawdown Duration", "Avg. Drawdown Duration",
		"# Trades", "Win Rate [%]",
		"Best Trade [%]", "Worst Trade [%]", "Avg. Trade [%]",
		"Max. Trade Duration", "Avg. Trade Duration",
		"Profit Factor", "Expectancy [%]", "SQN",
		"Kelly Criterion", "Commissions [$]",
	}
	for _, label := range labels {
		if !strings.Contains(out, label) {
			t.Errorf("String() missing label %q", label)
		}
	}

	// Verify specific formatted values.
	checks := []struct {
		desc string
		want string
	}{
		{"Return [%] value", "0.04"},
		{"# Trades value", "5"},
		{"Sharpe Ratio value", "0.058"},
		{"Commissions [$] value", "12.50"},
		{"Win Rate [%] value", "40.00"},
		{"MaxDrawdownDur value", "158 days"},
		{"AvgDrawdownDur value", "74 days"},
		{"MaxTradeDur value", "69 days"},
		{"AvgTradeDur value", "34 days"},
	}
	for _, c := range checks {
		if !strings.Contains(out, c.want) {
			t.Errorf("String() %s: want %q in output\nfull output:\n%s", c.desc, c.want, out)
		}
	}

	// Verify row count: one non-empty line per metric (32 rows).
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if len(lines) != 32 {
		t.Errorf("String() produced %d lines, want 32", len(lines))
	}
}

func TestStatsStringOmitsZeroCommissions(t *testing.T) {
	s := Stats{NumTrades: 3} // CommissionsTotal == 0
	out := s.String()
	if strings.Contains(out, "Commissions [$]") {
		t.Fatal("String() must omit Commissions row when total is 0 (matches backtesting.py)")
	}
}
