package backtest

import (
	"testing"
	"time"
)

func TestPositionAggregates(t *testing.T) {
	b := &broker{}
	b.trades = []*trade{
		{size: 2, entryPrice: 10},
		{size: -1, entryPrice: 20},
	}
	b.lastClose = 12
	p := &Position{broker: b}
	if p.Size() != 1 { // 2 + (-1)
		t.Fatalf("Size()=%v want 1", p.Size())
	}
	if !p.IsLong() {
		t.Fatal("net long expected")
	}
	// PL: long 2@10 now 12 -> +4 ; short 1@20 now 12 -> +8 ; total +12
	if p.PL() != 12 {
		t.Fatalf("PL()=%v want 12", p.PL())
	}
}

func TestPositionCloseEnqueuesDeferredCloseOrders(t *testing.T) {
	d, err := FromOHLCV(
		[]time.Time{time.Unix(0, 0), time.Unix(1, 0), time.Unix(2, 0)},
		[]float64{10, 20, 10}, []float64{10, 20, 10},
		[]float64{10, 20, 10}, []float64{10, 20, 10}, []float64{0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	b := newBroker(d, Options{Cash: 1000, Margin: 1})
	b.i = 0
	b.lastClose = 10
	b.trades = []*trade{
		{size: 1, entryPrice: 9, entryBar: 0},
		{size: 1, entryPrice: 8, entryBar: 0},
		{size: 1, entryPrice: 7, entryBar: 0},
	}
	p := &Position{broker: b}
	p.Close()
	// Deferred: trades still open, three close orders queued
	if len(b.trades) != 3 {
		t.Fatalf("Close() must be deferred; open trades=%d want 3", len(b.trades))
	}
	if len(b.orders) != 3 {
		t.Fatalf("Close() should enqueue 3 close orders; got %d", len(b.orders))
	}
	// Process next bar (i=1, open=20): all three close at 20
	b.i = 1
	b.lastClose = 10
	b.processOrders()
	if len(b.trades) != 0 {
		t.Fatalf("after processOrders, open trades=%d want 0", len(b.trades))
	}
	if len(b.closedTrades) != 3 {
		t.Fatalf("closed=%d want 3", len(b.closedTrades))
	}
	for _, ct := range b.closedTrades {
		if ct.exitPrice != 20 { // filled at bar1 open
			t.Fatalf("close fill price=%v want 20 (next bar open)", ct.exitPrice)
		}
	}
}
