package backtest

import (
	"math"
	"testing"
)

func TestTradePL(t *testing.T) {
	long := &trade{size: 2, entryPrice: 10}
	if long.pl(15) != 10 { // (15-10)*2
		t.Fatalf("long pl=%v want 10", long.pl(15))
	}
	short := &trade{size: -2, entryPrice: 10}
	if short.pl(8) != 4 { // (8-10)*-2
		t.Fatalf("short pl=%v want 4", short.pl(8))
	}
	if long.value(15) != 30 { // |2|*15
		t.Fatalf("value=%v want 30", long.value(15))
	}
	if !long.isLong() || !short.isShort() {
		t.Fatal("direction wrong")
	}
}

func TestTradeSnapshotNetReturn(t *testing.T) {
	// open long, entry 100, entryComm 2, marked to 110, size 1:
	// netPL = (110-100)*1 - 2 = 8; notional = 100*1 = 100; ReturnPct = 0.08
	tr := &trade{size: 1, entryPrice: 100, entryBar: 0, entryComm: 2}
	v := tr.snapshot(110)
	if v.PL != 8 {
		t.Fatalf("snapshot PL=%v want 8", v.PL)
	}
	if v.ReturnPct < 0.0799 || v.ReturnPct > 0.0801 {
		t.Fatalf("snapshot ReturnPct=%v want ~0.08", v.ReturnPct)
	}
}

func TestTradeExport(t *testing.T) {
	tr := &trade{size: 2, entryPrice: 100, exitPrice: 110, entryBar: 1, exitBar: 4, entryComm: 0.5, exitComm: 0.5}
	e := tr.export()
	// PL = (110-100)*2 - (0.5+0.5) = 20 - 1 = 19
	if e.PL != 19 {
		t.Fatalf("export PL=%v want 19", e.PL)
	}
	// ReturnPct = net PnL / (entryPrice * |size|) = 19 / (100*2) = 0.095.
	// This is the commission-adjusted return, matching backtesting.py's formula.
	wantRet := 19.0 / (100.0 * 2)
	if math.Abs(e.ReturnPct-wantRet) > 1e-12 {
		t.Fatalf("export ReturnPct=%v want %v (net PnL / initial position value)", e.ReturnPct, wantRet)
	}
	if e.EntryBar != 1 || e.ExitBar != 4 {
		t.Fatalf("export bars wrong: %d %d", e.EntryBar, e.ExitBar)
	}
	// Commissions = entryComm + exitComm = 0.5 + 0.5 = 1.0.
	if e.Commissions != 1.0 {
		t.Fatalf("export Commissions=%v want 1.0", e.Commissions)
	}
}
