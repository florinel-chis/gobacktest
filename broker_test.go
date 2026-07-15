package backtest

import (
	"errors"
	"math"
	"testing"
	"time"
)

func threeBar(o, h, l, c float64) *Data {
	d, _ := FromOHLCV(
		[]time.Time{time.Unix(0, 0), time.Unix(86400, 0), time.Unix(172800, 0)},
		[]float64{o, o, o}, []float64{h, h, h},
		[]float64{l, l, l}, []float64{c, c, c}, []float64{0, 0, 0})
	return d
}

func TestMarketFillAtOpen(t *testing.T) {
	b := newBroker(threeBar(10, 12, 9, 11), Options{Cash: 1000, Margin: 1})
	b.i = 1
	b.lastClose = 11
	b.orders = []*order{{size: 10}} // market buy 10 units
	b.processOrders()
	if len(b.trades) != 1 {
		t.Fatalf("trades=%d want 1", len(b.trades))
	}
	if b.trades[0].entryPrice != 10 { // bar1 open
		t.Fatalf("entryPrice=%v want 10", b.trades[0].entryPrice)
	}
	if len(b.orders) != 0 {
		t.Fatalf("orders should be consumed, got %d", len(b.orders))
	}
}

func TestLimitBuyOnlyFillsWhenLowCrosses(t *testing.T) {
	// limit buy at 9.5; bar low is 9 -> fills at limit 9.5 (better of open/limit: min(10,9.5)=9.5)
	b := newBroker(threeBar(10, 12, 9, 11), Options{Cash: 1000, Margin: 1})
	b.i = 1
	b.lastClose = 11
	b.orders = []*order{{size: 5, limit: 9.5}}
	b.processOrders()
	if len(b.trades) != 1 || math.Abs(b.trades[0].entryPrice-9.5) > 1e-9 {
		t.Fatalf("limit fill price=%v want 9.5 (trades=%d)", priceOf(b), len(b.trades))
	}
}

func TestLimitBuyDoesNotFillAboveLimit(t *testing.T) {
	// limit buy at 8; bar low is 9 -> never reached -> no fill, order remains
	b := newBroker(threeBar(10, 12, 9, 11), Options{Cash: 1000, Margin: 1})
	b.i = 1
	b.lastClose = 11
	b.orders = []*order{{size: 5, limit: 8}}
	b.processOrders()
	if len(b.trades) != 0 || len(b.orders) != 1 {
		t.Fatalf("should not fill; trades=%d orders=%d", len(b.trades), len(b.orders))
	}
}

func priceOf(b *broker) float64 {
	if len(b.trades) == 0 {
		return math.NaN()
	}
	return b.trades[0].entryPrice
}

func brokerWithData(t *testing.T) *broker {
	t.Helper()
	d, err := FromOHLCV(
		[]time.Time{{}, {}, {}},
		[]float64{10, 11, 12}, []float64{10, 11, 12},
		[]float64{10, 11, 12}, []float64{10, 11, 12},
		[]float64{0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	return newBroker(d, Options{Cash: 1000, Margin: 1})
}

func TestStopMarketTriggersAndFills(t *testing.T) {
	// stop buy 10.5; bar high 11 >= 10.5 triggers; no limit -> fill at stopPrice 10.5
	b := newBroker(threeBar(10, 11, 9, 10), Options{Cash: 10000, Margin: 1})
	b.i = 1
	b.lastClose = 10
	b.orders = []*order{{size: 5, stop: 10.5}}
	b.processOrders()
	if len(b.trades) != 1 || math.Abs(priceOf(b)-10.5) > 1e-9 {
		t.Fatalf("stop-market fill price=%v want 10.5 (trades=%d)", priceOf(b), len(b.trades))
	}
}

func TestStopNotTriggeredStaysPending(t *testing.T) {
	// stop buy 12; bar high 11 < 12 -> not triggered, stays pending
	b := newBroker(threeBar(10, 11, 9, 10), Options{Cash: 10000, Margin: 1})
	b.i = 1
	b.lastClose = 10
	b.orders = []*order{{size: 5, stop: 12}}
	b.processOrders()
	if len(b.trades) != 0 || len(b.orders) != 1 {
		t.Fatalf("untriggered stop: trades=%d orders=%d (want 0,1)", len(b.trades), len(b.orders))
	}
}

func TestStopLimitSameBarFill(t *testing.T) {
	// stop 10.5 triggers (high 11), limit 9.8, low 9 hits limit -> price min(stopPrice 10.5, limit 9.8)=9.8
	b := newBroker(threeBar(10, 11, 9, 10), Options{Cash: 10000, Margin: 1})
	b.i = 1
	b.lastClose = 10
	b.orders = []*order{{size: 5, stop: 10.5, limit: 9.8}}
	b.processOrders()
	if len(b.trades) != 1 || math.Abs(priceOf(b)-9.8) > 1e-9 {
		t.Fatalf("stop-limit same-bar fill price=%v want 9.8 (trades=%d)", priceOf(b), len(b.trades))
	}
}

func TestStopLimitDeferredToNextBarLatches(t *testing.T) {
	// bar1 triggers stop (high 11) but limit 9.8 not hit (low 10). Order must stay pending with stop CLEARED.
	// bar2: high 9.9 would NOT re-trigger stop 10.5, but order is now a pure limit; low 9.5 <= 9.8 fills at min(open 9.7, 9.8)=9.7.
	d, err := FromOHLCV(
		[]time.Time{time.Unix(0, 0), time.Unix(1, 0), time.Unix(2, 0)},
		[]float64{10, 10, 9.7},
		[]float64{10, 11, 9.9},
		[]float64{10, 10, 9.5},
		[]float64{10, 10, 9.8},
		[]float64{0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	b := newBroker(d, Options{Cash: 10000, Margin: 1})
	b.i = 1
	b.lastClose = 10
	b.orders = []*order{{size: 5, stop: 10.5, limit: 9.8}}
	b.processOrders()
	if len(b.trades) != 0 || len(b.orders) != 1 {
		t.Fatalf("bar1: stop hit, limit missed -> pending; trades=%d orders=%d", len(b.trades), len(b.orders))
	}
	if b.orders[0].stop != 0 {
		t.Fatalf("bar1: stop must be latched/cleared, got %v", b.orders[0].stop)
	}
	b.i = 2
	b.lastClose = 10
	b.processOrders()
	if len(b.trades) != 1 || math.Abs(priceOf(b)-9.7) > 1e-9 {
		t.Fatalf("bar2: latched limit should fill at 9.7; trades=%d price=%v", len(b.trades), priceOf(b))
	}
}

func TestStopLossFillsFirstWhenBothHit(t *testing.T) {
	// long trade entry 10, sl 9, tp 12; bar ranges low 8 high 13 -> both hit -> SL wins
	b := newBroker(threeBar(10, 13, 8, 11), Options{Cash: 1000, Margin: 1})
	b.i = 1
	b.lastClose = 11
	b.trades = []*trade{{size: 10, entryPrice: 10, entryBar: 0, sl: 9, tp: 12}}
	b.processContingent()
	if len(b.trades) != 0 {
		t.Fatalf("trade should be closed; open=%d", len(b.trades))
	}
	last := b.closedTrades[len(b.closedTrades)-1]
	if last.exitPrice != 9 { // stop-loss price, not tp 12
		t.Fatalf("exitPrice=%v want 9 (SL first)", last.exitPrice)
	}
}

func TestTakeProfitOnlyHit(t *testing.T) {
	// long entry 10, tp 12, no sl; bar high 13 low 11 -> tp fills at 12
	b := newBroker(threeBar(11, 13, 11, 12), Options{Cash: 1000, Margin: 1})
	b.i = 1
	b.lastClose = 12
	b.trades = []*trade{{size: 10, entryPrice: 10, entryBar: 0, tp: 12}}
	b.processContingent()
	if len(b.closedTrades) != 1 || b.closedTrades[0].exitPrice != 12 {
		t.Fatalf("tp exit price wrong")
	}
}

func TestBrokerEquityAndMargin(t *testing.T) {
	b := brokerWithData(t)
	b.lastClose = 12
	b.trades = []*trade{{size: 10, entryPrice: 10}}
	// equity = cash 1000 + pl (12-10)*10=20 = 1020
	if math.Abs(b.equityNow()-1020) > 1e-9 {
		t.Fatalf("equityNow=%v want 1020", b.equityNow())
	}
	// margin=1 -> leverage 1 -> marginUsed = value = |10|*12 = 120
	if math.Abs(b.marginUsed()-120) > 1e-9 {
		t.Fatalf("marginUsed=%v want 120", b.marginUsed())
	}
	if math.Abs(b.marginAvailable()-(1020-120)) > 1e-9 {
		t.Fatalf("marginAvailable=%v want 900", b.marginAvailable())
	}
}

func TestShortStopLossFillsFirstWhenBothHit(t *testing.T) {
	// short trade entry 10, sl 12, tp 8; bar high 13 (sl hit) low 7 (tp hit) -> SL wins (exit 12)
	b := newBroker(threeBar(10, 13, 7, 10), Options{Cash: 10000, Margin: 1})
	b.i = 1
	b.lastClose = 10
	b.trades = []*trade{{size: -10, entryPrice: 10, entryBar: 0, sl: 12, tp: 8}}
	b.processContingent()
	if len(b.trades) != 0 {
		t.Fatalf("short trade should be closed; open=%d", len(b.trades))
	}
	last := b.closedTrades[len(b.closedTrades)-1]
	if last.exitPrice != 12 {
		t.Fatalf("short exitPrice=%v want 12 (SL first, not TP 8)", last.exitPrice)
	}
}

func TestShortTakeProfitOnlyHit(t *testing.T) {
	// short entry 10, tp 8, no sl; bar high 9 (no sl) low 7 (<=8 hits tp) -> exit at 8
	b := newBroker(threeBar(8, 9, 7, 8), Options{Cash: 10000, Margin: 1})
	b.i = 1
	b.lastClose = 8
	b.trades = []*trade{{size: -10, entryPrice: 10, entryBar: 0, tp: 8}}
	b.processContingent()
	if len(b.closedTrades) != 1 || b.closedTrades[0].exitPrice != 8 {
		t.Fatalf("short TP exit wrong: closed=%d exit=%v", len(b.closedTrades), shortExit(b))
	}
}

func shortExit(b *broker) float64 {
	if len(b.closedTrades) == 0 {
		return -1
	}
	return b.closedTrades[len(b.closedTrades)-1].exitPrice
}

func TestNettingClosesOppositeFIFO(t *testing.T) {
	b := newBroker(threeBar(10, 12, 9, 11), Options{Cash: 10000, Margin: 1})
	b.i = 1
	b.lastClose = 11
	// open long 10 @10
	b.trades = []*trade{{size: 10, entryPrice: 10, entryBar: 0}}
	// a sell order for 4 units should net down the long to 6, not open a short
	residual := b.net(-4, 11)
	if residual != 0 {
		t.Fatalf("residual=%v want 0 (fully netted)", residual)
	}
	if len(b.trades) != 1 || b.trades[0].size != 6 {
		t.Fatalf("net long should be 6, got %v (trades=%d)", b.trades[0].size, len(b.trades))
	}
}

func TestNettingFlipsWhenLarger(t *testing.T) {
	b := newBroker(threeBar(10, 12, 9, 11), Options{Cash: 10000, Margin: 1})
	b.i = 1
	b.lastClose = 11
	b.trades = []*trade{{size: 10, entryPrice: 10, entryBar: 0}}
	residual := b.net(-15, 11) // close 10 long, 5 left to open short
	if residual != -5 {
		t.Fatalf("residual=%v want -5", residual)
	}
	if len(b.trades) != 0 {
		t.Fatalf("all longs should be closed, open=%d", len(b.trades))
	}
}

func TestExclusiveOrdersCancelsPendingAndClosesTrades(t *testing.T) {
	b := newBroker(threeBar(10, 12, 9, 11), Options{Cash: 100000, Margin: 1, ExclusiveOrders: true})
	b.i = 1
	b.lastClose = 11
	b.trades = []*trade{{size: 10, entryPrice: 8, entryBar: 0}} // pre-existing long
	b.orders = []*order{
		{size: 5},           // market buy -> exclusive fires
		{size: 5, limit: 1}, // far limit; must be canceled, not left pending
	}
	b.processOrders()
	if len(b.closedTrades) != 1 {
		t.Fatalf("pre-existing trade should be closed; closed=%d", len(b.closedTrades))
	}
	if len(b.orders) != 0 {
		t.Fatalf("far limit should be canceled; pending orders=%d", len(b.orders))
	}
	if len(b.trades) != 1 || b.trades[0].size != 5 {
		t.Fatalf("expected exactly one new trade of size 5; trades=%d", len(b.trades))
	}
}

func TestOrderCancelSkipsFill(t *testing.T) {
	b := newBroker(threeBar(10, 12, 9, 11), Options{Cash: 10000, Margin: 1})
	b.i = 1
	b.lastClose = 11
	o := &order{size: 5}
	o.Cancel()
	b.orders = []*order{o}
	b.processOrders()
	if len(b.trades) != 0 {
		t.Fatalf("canceled order should not fill; trades=%d", len(b.trades))
	}
}

func TestLiquidationWhenEquityNonPositive(t *testing.T) {
	b := newBroker(threeBar(10, 10, 10, 10), Options{Cash: 100, Margin: 1})
	b.i = 1
	b.lastClose = 1 // huge loss on a big short
	// short 200 @10 now price 1 ... actually make a loss that wipes equity:
	b.trades = []*trade{{size: -100, entryPrice: 1}} // short 100 @1, price now 1 -> pl 0; adjust:
	b.trades[0].entryPrice = 0.5                     // short @0.5, now 1 -> pl (1-0.5)*-100 = -50; eq=50>0
	b.trades[0].entryPrice = 0.0                     // pl (1-0)*-100=-100; eq = 100-100 = 0 -> liquidate
	err := b.next()
	if !errors.Is(err, ErrOutOfMoney) {
		t.Fatalf("expected ErrOutOfMoney, got %v", err)
	}
	if len(b.trades) != 0 {
		t.Fatalf("trades should be force-closed, open=%d", len(b.trades))
	}
	if b.equity[1] != 0 {
		t.Fatalf("equity[1]=%v want 0 after liquidation", b.equity[1])
	}
}

func TestRecordEquityNormalBar(t *testing.T) {
	b := newBroker(threeBar(10, 10, 10, 10), Options{Cash: 100, Margin: 1})
	b.i = 0
	b.lastClose = 10
	b.trades = []*trade{{size: 1, entryPrice: 8}} // pl (10-8)*1=2 -> eq 102
	if err := b.next(); err != nil {
		t.Fatalf("unexpected err %v", err)
	}
	if b.equity[0] != 102 {
		t.Fatalf("equity[0]=%v want 102", b.equity[0])
	}
}

func TestPositionCloseFillsAtNextOpenNotCurrentClose(t *testing.T) {
	// bar1 close=11; bar2 open=99. A close queued while on bar1 must fill at bar2 open (99), NOT bar1 close (11).
	d, err := FromOHLCV(
		[]time.Time{time.Unix(0, 0), time.Unix(1, 0), time.Unix(2, 0)},
		[]float64{10, 10, 99}, []float64{10, 12, 99},
		[]float64{9, 9, 99}, []float64{10, 11, 99}, []float64{0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	b := newBroker(d, Options{Cash: 100000, Margin: 1})
	b.i = 1
	b.lastClose = 11
	b.trades = []*trade{{size: 10, entryPrice: 10, entryBar: 0}}
	p := &Position{broker: b}
	p.Close() // queued on bar1
	if len(b.trades) != 1 {
		t.Fatalf("still open on bar1; trades=%d", len(b.trades))
	}
	b.i = 2 // advance to bar2
	b.lastClose = 11
	b.processOrders()
	if len(b.closedTrades) != 1 {
		t.Fatalf("should close on bar2; closed=%d", len(b.closedTrades))
	}
	if b.closedTrades[0].exitPrice != 99 {
		t.Fatalf("exitPrice=%v want 99 (bar2 open), not 11 (bar1 close)", b.closedTrades[0].exitPrice)
	}
}

func TestPartialCloseReducesEntryComm(t *testing.T) {
	b := newBroker(threeBar(10, 12, 9, 11), Options{Cash: 100000, Margin: 1, Commission: Pct(0.01)})
	b.i = 1
	b.lastClose = 11
	b.trades = []*trade{{size: 10, entryPrice: 10, entryBar: 0, entryComm: 1.0}}
	if residual := b.net(-4, 11); residual != 0 { // close 40% of the front long
		t.Fatalf("residual=%v want 0", residual)
	}
	if len(b.trades) != 1 || math.Abs(b.trades[0].entryComm-0.6) > 1e-9 {
		t.Fatalf("remaining entryComm=%v want 0.6", b.trades[0].entryComm)
	}
	last := b.closedTrades[len(b.closedTrades)-1]
	if math.Abs(last.entryComm-0.4) > 1e-9 {
		t.Fatalf("closed lot entryComm=%v want 0.4", last.entryComm)
	}
}

func TestPositionCloseUnderHedgingClosesEachTrade(t *testing.T) {
	// hedging on: two opposite open trades; Close() must close BOTH specifically (bypasses netting).
	d, err := FromOHLCV(
		[]time.Time{time.Unix(0, 0), time.Unix(1, 0), time.Unix(2, 0)},
		[]float64{10, 20, 10}, []float64{10, 20, 10},
		[]float64{10, 20, 10}, []float64{10, 20, 10}, []float64{0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	b := newBroker(d, Options{Cash: 100000, Margin: 1, Hedging: true})
	b.i = 0
	b.lastClose = 10
	b.trades = []*trade{
		{size: 5, entryPrice: 9, entryBar: 0},
		{size: -3, entryPrice: 11, entryBar: 0},
	}
	p := &Position{broker: b}
	p.Close()
	if len(b.orders) != 2 {
		t.Fatalf("want 2 deferred close orders, got %d", len(b.orders))
	}
	b.i = 1
	b.lastClose = 10
	b.processOrders() // fills at bar1 open=20
	if len(b.trades) != 0 {
		t.Fatalf("both trades should close under hedging; open=%d", len(b.trades))
	}
	if len(b.closedTrades) != 2 {
		t.Fatalf("closed=%d want 2", len(b.closedTrades))
	}
}

func TestPositionCloseStaleParentSkipped(t *testing.T) {
	// A close order whose parent already closed (by other means) must be silently skipped.
	d, err := FromOHLCV(
		[]time.Time{time.Unix(0, 0), time.Unix(1, 0), time.Unix(2, 0)},
		[]float64{10, 20, 10}, []float64{10, 20, 10},
		[]float64{10, 20, 10}, []float64{10, 20, 10}, []float64{0, 0, 0})
	if err != nil {
		t.Fatal(err)
	}
	b := newBroker(d, Options{Cash: 100000, Margin: 1})
	b.i = 0
	b.lastClose = 10
	tr := &trade{size: 5, entryPrice: 9, entryBar: 0}
	b.trades = []*trade{tr}
	p := &Position{broker: b}
	p.Close()              // queues a close order for tr
	b.closeTradeAt(tr, 10) // close tr by another means first
	closedBefore := len(b.closedTrades)
	b.i = 1
	b.lastClose = 10
	b.processOrders() // the stale close order must find tr not open -> skip
	if len(b.closedTrades) != closedBefore {
		t.Fatalf("stale close order should be skipped; closed grew %d -> %d", closedBefore, len(b.closedTrades))
	}
}

func TestResolveSizeFractional(t *testing.T) {
	// fixed size (>=1) truncates to whole units
	b := newBroker(threeBar(10, 10, 10, 10), Options{Cash: 1000, Margin: 1})
	b.lastClose = 10
	if got := b.resolveSize(&order{size: 7.9}, 10, 10); got != 7 {
		t.Fatalf("fixed size=%v want 7", got)
	}
	// fractional: 50% of equity 1000 at price 10 -> floor(500/10)=50
	if got := b.resolveSize(&order{size: 0.5}, 10, 10); got != 50 {
		t.Fatalf("fractional size=%v want 50", got)
	}
	// leverage 2x (margin 0.5) doubles buying power -> 100
	bl := newBroker(threeBar(10, 10, 10, 10), Options{Cash: 1000, Margin: 0.5})
	bl.lastClose = 10
	if got := bl.resolveSize(&order{size: 0.5}, 10, 10); got != 100 {
		t.Fatalf("levered fractional size=%v want 100", got)
	}
	// commission raises the per-unit price -> fewer units: floor(500/(10+0.1))=49
	bc := newBroker(threeBar(10, 10, 10, 10), Options{Cash: 1000, Margin: 1, Commission: Pct(0.01)})
	bc.lastClose = 10
	if got := bc.resolveSize(&order{size: 0.5}, 10, 10); got != 49 {
		t.Fatalf("fractional-with-commission size=%v want 49", got)
	}
}

func TestLiquidityCheckCancelsOversizedOrder(t *testing.T) {
	// $100 buying power; a 50-unit order at ~$10 = $500 notional must be canceled.
	b := newBroker(threeBar(10, 12, 9, 10), Options{Cash: 100, Margin: 1})
	b.i = 1
	b.lastClose = 10
	b.orders = []*order{{size: 50}}
	b.processOrders()
	if len(b.trades) != 0 {
		t.Fatalf("oversized order should be canceled; open trades=%d", len(b.trades))
	}
	if len(b.orders) != 0 {
		t.Fatalf("canceled order must not remain pending; orders=%d", len(b.orders))
	}
}
