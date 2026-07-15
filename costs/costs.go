// Package costs computes trading costs the engine does not model natively
// (per-trade holding/financing cost).
package costs

import (
	"math"

	backtest "github.com/florinel-chis/gobacktest"
)

// FinancingPct returns the total holding (overnight financing) cost of the
// closed trades as a percent of starting cash. rate is the annual cost as a
// fraction of notional (e.g. 0.0468 for Oanda EU50_EUR longs); brokers charge
// per calendar day held (Oanda: Mon-Thu x1, Fri x3 covering the weekend, i.e.
// 7 charges per week), so elapsed calendar time is the correct day count.
// Shorts pay on |size| notional the same way.
// FinancingPctSides is FinancingPct with per-direction rates in Oanda's sign
// convention (negative = cost, positive = credit): longs pay -longRate, shorts
// pay -shortRate. Returns the net cost as a percent of cash (negative = the
// position book earned financing).
func FinancingPctSides(trades []backtest.Trade, longRate, shortRate, cash float64) float64 {
	if cash == 0 {
		return 0
	}
	var cost float64
	for _, t := range trades {
		rate := -longRate
		if t.Size < 0 {
			rate = -shortRate
		}
		days := t.ExitTime.Sub(t.EntryTime).Hours() / 24
		cost += math.Abs(t.Size) * t.EntryPrice * rate * days / 365
	}
	return cost / cash * 100
}

func FinancingPct(trades []backtest.Trade, rate, cash float64) float64 {
	if rate == 0 || cash == 0 {
		return 0
	}
	var cost float64
	for _, t := range trades {
		days := t.ExitTime.Sub(t.EntryTime).Hours() / 24
		cost += math.Abs(t.Size) * t.EntryPrice * rate * days / 365
	}
	return cost / cash * 100
}
