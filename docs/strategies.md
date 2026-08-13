# Strategies

A strategy is any Go type that implements the `backtest.Strategy` interface:

```go
type Strategy interface {
    Init(*State)
    Next(*State)
}
```

---

## Init

`Init` is called **once** before the bar loop begins, with a full-length data view.
Use it to:
- Pre-compute indicators via `st.I(...)`.
- Store any indicator handles your `Next` method will read.

```go
type myStrat struct {
    sma *backtest.Indicator
    rsi *backtest.Indicator
}

func (s *myStrat) Init(st *backtest.State) {
    c := st.Data().Close()
    s.sma = st.I("SMA20", func() []float64 { return indicators.SMA(c, 20) },
        backtest.Overlay(), backtest.Color("#2962FF"))
    s.rsi = st.I("RSI14", func() []float64 { return indicators.RSI(c, 14) })
}
```

**`st.I` signature:**

```go
func (s *State) I(name string, compute func() []float64, opts ...backtest.IndicatorOption) *backtest.Indicator
```

The `compute` closure runs immediately (once, on the full data). The returned
`*Indicator` handle auto-slices per bar when read in `Next`.

**Indicator plot options:**

```go
backtest.Overlay()       // draw on the price pane (e.g. moving averages)
backtest.Color("#hex")   // line colour for the HTML report
```

---

## Next

`Next` is called once per bar (after the engine has grown the data view by one bar
and processed any pending orders). This is where you decide whether to buy or sell.

```go
func (s *myStrat) Next(st *backtest.State) {
    if s.rsi.Last() < 30 && st.Position().Size() == 0 {
        st.Buy(backtest.Order{Size: 0.5}) // 50% of equity
    }
    if s.rsi.Last() > 70 && st.Position().IsLong() {
        st.Position().Close()
    }
}
```

---

## State API

### Buying and selling

```go
handle := st.Buy(backtest.Order{...})   // enqueue a long order
handle := st.Sell(backtest.Order{...})  // enqueue a short order (size > 0 → stored negative)
```

Both return an `OrderHandle` that can cancel the order before it fills:

```go
handle.Cancel()
```

### Order fields

```go
type Order struct {
    Size  float64   // fraction (0,1] of equity OR absolute units ≥ 1
    Limit float64   // limit price (0 = market order)
    Stop  float64   // stop-entry price (0 = unset)
    SL    float64   // stop-loss price (0 = unset)
    TP    float64   // take-profit price (0 = unset)
    Tag   any       // arbitrary label attached to the resulting Trade
}
```

`Size` semantics:
- **Fraction** `(0, 1]` — e.g. `0.5` buys with 50% of current equity.
- **Absolute** `≥ 1` — e.g. `10` buys exactly 10 units.

`Sell.Size` must also be `> 0`; the engine stores it as negative internally.

### Position

```go
pos := st.Position()
pos.Size()    // signed sum of open trade sizes (positive = long)
pos.IsLong()  // true if Size() > 0
pos.IsShort() // true if Size() < 0
pos.PL()      // mark-to-market P&L in cash
pos.Close()   // queue a close of all open trades (fills at next bar's open)
```

### Other state accessors

```go
st.Data()          // *backtest.Data — current-bar view
st.Equity()        // float64 — current equity (cash + open P&L)
st.Trades()        // []Trade — open trades snapshot
st.ClosedTrades()  // []Trade — all closed trades
st.Orders()        // []OrderHandle — pending (unfilled) orders
```

---

## Bar-by-bar model (anti-look-ahead)

The engine loop per bar is:

1. Grow the data view to include bar `i`.
2. Process queued orders → fill at bar `i`'s **open** price (or the limit/stop price if hit).
3. Process contingent SL/TP orders.
4. Call `strategy.Next(st)`.

Orders submitted in `Next` during bar `i` are filled at bar `i+1`'s open.
This means `Next` can never accidentally trade on a price it hasn't yet "seen".

---

## Full example: SMA crossover

```go
package main

import (
    "fmt"

    backtest "github.com/florinel-chis/gobacktest"
    "github.com/florinel-chis/gobacktest/indicators"
    "github.com/florinel-chis/gobacktest/lib"
    "github.com/florinel-chis/gobacktest/report"
)

type smaCross struct{ fast, slow *backtest.Indicator }

func (s *smaCross) Init(st *backtest.State) {
    c := st.Data().Close()
    s.fast = st.I("SMA10", func() []float64 { return indicators.SMA(c, 10) },
        backtest.Overlay(), backtest.Color("#2962FF"))
    s.slow = st.I("SMA20", func() []float64 { return indicators.SMA(c, 20) },
        backtest.Overlay(), backtest.Color("#ff6d00"))
}

func (s *smaCross) Next(st *backtest.State) {
    switch {
    case lib.Crossover(s.fast.Series(), s.slow.Series()) && st.Position().Size() == 0:
        st.Buy(backtest.Order{Size: 10})
    case lib.CrossUnder(s.fast.Series(), s.slow.Series()) && st.Position().IsLong():
        st.Position().Close()
    }
}

func main() {
    data, err := backtest.FromCSV("testdata/AAPL_1d.csv")
    if err != nil {
        panic(err)
    }
    bt := backtest.New(data, &smaCross{}, backtest.Options{
        Cash: 10_000, Margin: 1, FinalizeTrades: true,
    })
    res, err := bt.Run()
    if err != nil {
        panic(err)
    }
    stats := backtest.Compute(res, data, 0)
    fmt.Println(stats)
    _ = report.Generate(data, res, stats, "report.html", report.Title("SMA Cross"))
}
```

---

## Engine options

```go
type Options struct {
    Cash            float64    // starting cash
    Spread          float64    // bid/ask half-spread fraction
    Commission      Commission // per-trade cost (nil = free)
    Margin          float64    // margin requirement (1 = no leverage)
    TradeOnClose    bool       // fill at close instead of next open
    Hedging         bool       // allow simultaneous long + short
    ExclusiveOrders bool       // cancel prior orders before filling new ones
    FinalizeTrades  bool       // close all open trades after the last bar
}
```

**Commission helpers:**

```go
backtest.Pct(0.001)             // 0.1% of trade value per side
backtest.FixedPlusPct(1, 0.001) // $1 + 0.1% per trade
```

## Trailing stops & mutable open trades

`State.OpenTrades()` returns mutable handles to the currently-open trades, so a strategy can
adjust a live trade's stop-loss/take-profit each bar (`OpenTrade.SetSL`, `SetTP`, plus
`IsLong`/`EntryPrice`/`SL`/`TP` readers). The `lib` package ports backtesting.py's
`TrailingStrategy`:

```go
// In Init: register the trailing ATR (backtesting.py's SMA-of-TR + bfill, NOT Wilder's).
s.atr = st.I("atr", func() []float64 {
    return lib.TrailingATR(st.Data().High(), st.Data().Low(), st.Data().Close(), 20)
})

// In Next: after your entries, trail every open trade's SL by 3×ATR (only ever tightening).
lib.TrailStop(st, s.atr.Last(), 3)
```

A mutated `SL()`/`TP()` takes effect on the next bar's SL/TP check. The trade exits when the
trailing stop is hit — no explicit `Close()` needed.

---

## Built-in strategies

The `strategies` package ships a few ready-to-run types:

- `AveragingGrid` — long-only averaging-down grid with per-lot take-profits.
- `WilliamsROversold` — Williams %R oversold entries.
- `SignalStrategy` — trades an externally-computed per-bar signal (long when
  `Signal[bar] > 0`, exit after `Hold` bars). Use it to backtest any predictor
  that emits one number per bar — an indicator rule, a screen, or a
  machine-learning forecast. See [ML signals](ml-signals.md) for the full pattern
  and a Kronos dip-buy case study, plus the runnable `cmd/mlsignal` example.
