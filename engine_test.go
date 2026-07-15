package backtest

import (
	"errors"
	"math"
	"testing"
	"time"
)

// buyAndHold buys once on the first Next and holds.
type buyAndHold struct{ bought bool }

func (s *buyAndHold) Init(st *State) {}
func (s *buyAndHold) Next(st *State) {
	if !s.bought {
		st.Buy(Order{Size: 1}) // 1 unit
		s.bought = true
	}
}

func ramp(n int) *Data {
	tm := make([]time.Time, n)
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)
	v := make([]float64, n)
	base := time.Unix(0, 0)
	for i := 0; i < n; i++ {
		tm[i] = base.AddDate(0, 0, i)
		price := float64(10 + i) // 10,11,12,...
		o[i], h[i], l[i], c[i], v[i] = price, price, price, price, 0
	}
	d, _ := FromOHLCV(tm, o, h, l, c, v)
	return d
}

// TestRunEmptyDataErrors locks that Run returns ErrNoBars — instead of
// panicking with index out of range — when the data has zero bars, both on
// the plain path and with FinalizeTrades (which indexes the last bar's open).
func TestRunEmptyDataErrors(t *testing.T) {
	for _, opts := range []Options{
		{Cash: 1000, Margin: 1},
		{Cash: 1000, Margin: 1, FinalizeTrades: true},
	} {
		bt := New(FromBars(nil), &buyAndHold{}, opts)
		res, err := bt.Run()
		if !errors.Is(err, ErrNoBars) {
			t.Fatalf("FinalizeTrades=%v: err=%v, want ErrNoBars", opts.FinalizeTrades, err)
		}
		if res != nil {
			t.Fatalf("FinalizeTrades=%v: res=%+v, want nil", opts.FinalizeTrades, res)
		}
	}
}

func TestBuyAndHoldProfits(t *testing.T) {
	d := ramp(5) // prices 10..14
	bt := New(d, &buyAndHold{}, Options{Cash: 1000, Margin: 1})
	res, err := bt.Run()
	if err != nil {
		t.Fatalf("Run error: %v", err)
	}
	// Buy queued on bar0's Next -> fills at bar1 open=11; holds to last close=14.
	// 1 unit: equity = cash(1000 - 11 spent? no: cash only changes by commission;
	// open pl marks to close) -> final equity = 1000 + (14-11)*1 = 1003.
	if math.Abs(res.FinalEquity-1003) > 1e-6 {
		t.Fatalf("FinalEquity=%v want 1003", res.FinalEquity)
	}
	if len(res.EquityCurve) != 5 {
		t.Fatalf("equity curve len=%d want 5", len(res.EquityCurve))
	}
}

func TestFinalizeTradesClosesOpen(t *testing.T) {
	d := ramp(4)
	bt := New(d, &buyAndHold{}, Options{Cash: 1000, Margin: 1, FinalizeTrades: true})
	res, err := bt.Run()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Trades) != 1 {
		t.Fatalf("expected 1 finalized closed trade, got %d", len(res.Trades))
	}
}

// TestFinalizeTradesUsesLastOpen locks the finalize exit price to the LAST BAR'S
// OPEN (not its close), matching backtesting.py. ramp() can't verify this
// because it sets open==close, so use a last bar where they differ.
func TestFinalizeTradesUsesLastOpen(t *testing.T) {
	tm := []time.Time{
		time.Unix(0, 0), time.Unix(0, 0).AddDate(0, 0, 1), time.Unix(0, 0).AddDate(0, 0, 2),
	}
	// bar1 fills the buy at open=12; last bar open=20 differs from close=30.
	o := []float64{10, 12, 20}
	h := []float64{10, 12, 30}
	l := []float64{10, 12, 20}
	c := []float64{10, 12, 30}
	v := []float64{0, 0, 0}
	d, err := FromOHLCV(tm, o, h, l, c, v)
	if err != nil {
		t.Fatal(err)
	}
	bt := New(d, &buyAndHold{}, Options{Cash: 1000, Margin: 1, FinalizeTrades: true})
	res, err := bt.Run()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Trades) != 1 {
		t.Fatalf("expected 1 finalized trade, got %d", len(res.Trades))
	}
	if got := res.Trades[0].ExitPrice; math.Abs(got-20) > 1e-9 {
		t.Fatalf("finalize ExitPrice=%v want 20 (last bar OPEN, not close=30)", got)
	}
}
