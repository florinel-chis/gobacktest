package backtest

import (
	"math"
	"time"
)

// trade is an open or closed position lot. size is signed.
type trade struct {
	size       float64
	entryPrice float64
	exitPrice  float64
	entryBar   int
	exitBar    int
	entryTime  time.Time
	exitTime   time.Time
	sl, tp     float64
	entryComm  float64
	exitComm   float64
	tag        any
}

// pl marks the trade to `price` (use the exit price for a closed trade).
func (t *trade) pl(price float64) float64 { return (price - t.entryPrice) * t.size }

// value is the absolute notional at `price`.
func (t *trade) value(price float64) float64 { return math.Abs(t.size) * price }

func (t *trade) isLong() bool         { return t.size > 0 }
func (t *trade) isShort() bool        { return t.size < 0 }
func (t *trade) commissions() float64 { return t.entryComm + t.exitComm }

// Trade is the public, read-only record of a closed trade in a Result.
type Trade struct {
	Size        float64
	EntryPrice  float64
	ExitPrice   float64
	EntryBar    int
	ExitBar     int
	EntryTime   time.Time
	ExitTime    time.Time
	PL          float64
	ReturnPct   float64
	Commissions float64
	Tag         any
}

// snapshot returns a public read-only view of an OPEN trade marked to price.
func (t *trade) snapshot(price float64) Trade {
	netPL := t.pl(price) - t.entryComm
	notional := t.entryPrice * math.Abs(t.size)
	var retPct float64
	if notional != 0 {
		retPct = netPL / notional
	}
	return Trade{
		Size:       t.size,
		EntryPrice: t.entryPrice,
		EntryBar:   t.entryBar,
		EntryTime:  t.entryTime,
		PL:         netPL,
		ReturnPct:  retPct,
		Tag:        t.tag,
	}
}

// export snapshots a closed trade into its public form.
// ReturnPct matches backtesting.py: net PnL (after commissions) divided by the
// initial position value (entryPrice * abs(size)), so commission costs reduce it.
func (t *trade) export() Trade {
	netPL := t.pl(t.exitPrice) - t.commissions()
	var retPct float64
	if denom := t.entryPrice * math.Abs(t.size); denom != 0 {
		retPct = netPL / denom
	}
	return Trade{
		Size:        t.size,
		EntryPrice:  t.entryPrice,
		ExitPrice:   t.exitPrice,
		EntryBar:    t.entryBar,
		ExitBar:     t.exitBar,
		EntryTime:   t.entryTime,
		ExitTime:    t.exitTime,
		PL:          netPL,
		ReturnPct:   retPct,
		Commissions: t.commissions(),
		Tag:         t.tag,
	}
}
