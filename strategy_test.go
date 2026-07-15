package backtest

import (
	"math"
	"testing"
)

func TestStateIComputesAndValidates(t *testing.T) {
	d := threeBar(1, 1, 1, 1) // fullLen 3
	b := newBroker(d, Options{Cash: 100, Margin: 1})
	s := &State{broker: b}
	ind := s.I("x", func() []float64 { return []float64{1, 2, 3} })
	if ind.Last() != 3 || len(s.indicators) != 1 {
		t.Fatalf("indicator not registered correctly")
	}
}

func TestStateIRejectsWrongLength(t *testing.T) {
	d := threeBar(1, 1, 1, 1)
	b := newBroker(d, Options{Cash: 100, Margin: 1})
	s := &State{broker: b}
	defer func() {
		if recover() == nil {
			t.Fatal("I() with wrong-length column should panic")
		}
	}()
	s.I("bad", func() []float64 { return []float64{1, 2} }) // len 2 != 3
}

func TestBuySellEnqueue(t *testing.T) {
	d := threeBar(1, 1, 1, 1)
	b := newBroker(d, Options{Cash: 100, Margin: 1})
	s := &State{broker: b}
	s.Buy(Order{Size: 0.5, SL: 0.9})
	s.Sell(Order{Size: 2})
	if len(b.orders) != 2 {
		t.Fatalf("orders=%d want 2", len(b.orders))
	}
	if b.orders[0].size != 0.5 || b.orders[0].sl != 0.9 {
		t.Fatalf("buy order wrong: %+v", b.orders[0])
	}
	if b.orders[1].size != -2 {
		t.Fatalf("sell order size=%v want -2", b.orders[1].size)
	}
}

func TestBuyRejectsBadSize(t *testing.T) {
	d := threeBar(1, 1, 1, 1)
	b := newBroker(d, Options{Cash: 100, Margin: 1})
	s := &State{broker: b}
	defer func() {
		if recover() == nil {
			t.Fatal("Buy with Size<=0 should panic")
		}
	}()
	s.Buy(Order{Size: 0})
	_ = math.NaN
}

func TestStateIRejectsDuplicateName(t *testing.T) {
	d := threeBar(1, 1, 1, 1)
	b := newBroker(d, Options{Cash: 100, Margin: 1})
	s := &State{broker: b}
	s.I("x", func() []float64 { return []float64{1, 2, 3} })
	defer func() {
		if recover() == nil {
			t.Fatal("duplicate indicator name should panic")
		}
	}()
	s.I("x", func() []float64 { return []float64{4, 5, 6} })
}

func TestSellRejectsBadSize(t *testing.T) {
	d := threeBar(1, 1, 1, 1)
	b := newBroker(d, Options{Cash: 100, Margin: 1})
	s := &State{broker: b}
	defer func() {
		if recover() == nil {
			t.Fatal("Sell with Size<=0 should panic")
		}
	}()
	s.Sell(Order{Size: 0})
}

func TestStateTradesExportedView(t *testing.T) {
	d := threeBar(10, 10, 10, 10)
	b := newBroker(d, Options{Cash: 1000, Margin: 1})
	b.lastClose = 12
	b.trades = []*trade{{size: 2, entryPrice: 10, entryBar: 0}}
	s := &State{broker: b}
	tv := s.Trades()
	if len(tv) != 1 || tv[0].EntryPrice != 10 || tv[0].PL != 4 { // (12-10)*2
		t.Fatalf("Trades() view wrong: %+v", tv)
	}
}

func TestBuyReturnsCancelableHandle(t *testing.T) {
	d := threeBar(10, 10, 10, 10)
	b := newBroker(d, Options{Cash: 1000, Margin: 1})
	s := &State{broker: b}
	h := s.Buy(Order{Size: 1})
	h.Cancel()
	if !b.orders[0].canceled {
		t.Fatal("Buy handle Cancel() did not cancel the queued order")
	}
}

func TestIndicatorMetadataExposedOnResult(t *testing.T) {
	d := vSeries(10, 11, 12, 13, 14, 15) // helper in engine tests
	bt := New(d, &indMetaStrat{}, Options{Cash: 1000, Margin: 1})
	res, err := bt.Run()
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Indicators) != 2 {
		t.Fatalf("Indicators=%d want 2", len(res.Indicators))
	}
	var fast *IndicatorSeries
	for i := range res.Indicators {
		if res.Indicators[i].Name == "fast" {
			fast = &res.Indicators[i]
		}
	}
	if fast == nil || !fast.Overlay || fast.Color != "#2962FF" {
		t.Fatalf("fast indicator metadata wrong: %+v", fast)
	}
	if len(fast.Values) != d.fullLen() {
		t.Fatalf("fast values len=%d want %d", len(fast.Values), d.fullLen())
	}
}

type indMetaStrat struct{}

func (indMetaStrat) Init(s *State) {
	s.I("fast", func() []float64 { return constSeries(s.Data().Close().Len(), 1) }, Overlay(), Color("#2962FF"))
	s.I("rsi", func() []float64 { return constSeries(s.Data().Close().Len(), 2) }) // non-overlay default
}
func (indMetaStrat) Next(s *State) {}

func constSeries(n int, v float64) []float64 {
	out := make([]float64, n)
	for i := range out {
		out[i] = v
	}
	return out
}
