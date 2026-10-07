package backtest

import (
	"testing"
	"time"
)

// scriptStrat buys on bar buyAt and closes the position on bar closeAt
// (bar indices as seen in Next), optionally with a limit price.
type scriptStrat struct {
	buyAt, closeAt int
	limit          float64
}

func (s *scriptStrat) Init(*State) {}
func (s *scriptStrat) Next(st *State) {
	switch i := len(st.Data().Close()) - 1; i {
	case s.buyAt:
		st.Buy(Order{Size: 1, Limit: s.limit})
	case s.closeAt:
		st.Position().Close()
	}
}

func tocBars() []Bar {
	t0 := time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC)
	var bars []Bar
	for i := 0; i < 6; i++ {
		px := 10 + float64(i)
		bars = append(bars, Bar{Time: t0.AddDate(0, 0, i), Open: px + 0.25, High: px + 1, Low: px - 1, Close: px, Volume: 1})
	}
	return bars
}

func runScript(t *testing.T, s *scriptStrat, tradeOnClose bool) Trade {
	t.Helper()
	res, err := New(FromBars(tocBars()), s, Options{Cash: 1000, Margin: 1, TradeOnClose: tradeOnClose}).Run()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Trades) != 1 {
		t.Fatalf("trades = %d, want 1", len(res.Trades))
	}
	return res.Trades[0]
}

// backtesting.py (_process_orders): for a market order under trade_on_close the
// fill is at the previous close AND the trade's entry/exit bar is that previous
// bar (time_index = i - 1) — the bar the order was placed on.
func TestTradeOnCloseStampsTheSignalBar(t *testing.T) {
	bars := tocBars()
	tr := runScript(t, &scriptStrat{buyAt: 1, closeAt: 3}, true)
	if tr.EntryPrice != bars[1].Close || tr.EntryBar != 1 || !tr.EntryTime.Equal(bars[1].Time) {
		t.Errorf("entry = %v @ bar %d (%s), want %v @ bar 1 (%s)",
			tr.EntryPrice, tr.EntryBar, tr.EntryTime.Format("01-02"), bars[1].Close, bars[1].Time.Format("01-02"))
	}
	if tr.ExitPrice != bars[3].Close || tr.ExitBar != 3 || !tr.ExitTime.Equal(bars[3].Time) {
		t.Errorf("exit = %v @ bar %d (%s), want %v @ bar 3 (%s)",
			tr.ExitPrice, tr.ExitBar, tr.ExitTime.Format("01-02"), bars[3].Close, bars[3].Time.Format("01-02"))
	}
}

// Without trade_on_close a market order fills at the next bar's open, stamped
// with that next bar (unchanged behaviour).
func TestDefaultFillStampsTheFillBar(t *testing.T) {
	bars := tocBars()
	tr := runScript(t, &scriptStrat{buyAt: 1, closeAt: 3}, false)
	if tr.EntryPrice != bars[2].Open || tr.EntryBar != 2 || tr.ExitPrice != bars[4].Open || tr.ExitBar != 4 {
		t.Errorf("entry %v@%d exit %v@%d, want %v@2 and %v@4", tr.EntryPrice, tr.EntryBar, tr.ExitPrice, tr.ExitBar, bars[2].Open, bars[4].Open)
	}
}

// A limit order is not a market order: even under trade_on_close it is
// stamped with the bar it fills on (backtesting.py: is_market_order is false).
func TestTradeOnCloseLimitOrderStampsTheFillBar(t *testing.T) {
	bars := tocBars()
	tr := runScript(t, &scriptStrat{buyAt: 1, closeAt: 3, limit: 100}, true)
	if tr.EntryBar != 2 || !tr.EntryTime.Equal(bars[2].Time) {
		t.Errorf("limit entry @ bar %d (%s), want bar 2", tr.EntryBar, tr.EntryTime.Format("01-02"))
	}
}
