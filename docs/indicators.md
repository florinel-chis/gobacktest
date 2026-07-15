# Indicators

The `indicators` package wraps [go-talib](https://github.com/markcheno/go-talib) with
two additions:

1. **NaN warmup propagation** — every function returns a full-length `[]float64` slice.
   Bars before the indicator is "warm" contain `math.NaN()`, not zero.
2. **Composition** — because inputs may already carry leading NaN, indicators can be
   chained (e.g. EMA of Williams %R) without requiring warmup alignment by hand.

Import:

```go
import (
    "github.com/florinel-chis/gobacktest/indicators"
    "github.com/florinel-chis/gobacktest/lib"
)
```

---

## Single-series indicators

```go
// SMA — simple moving average (warmup = period-1)
indicators.SMA(s []float64, period int) []float64

// EMA — exponential moving average (warmup = period-1)
// Accepts NaN-prefixed input for composition.
indicators.EMA(s []float64, period int) []float64

// WMA — weighted moving average (warmup = period-1)
indicators.WMA(s []float64, period int) []float64

// RSI — relative strength index (warmup = period)
indicators.RSI(s []float64, period int) []float64

// ROC — rate-of-change % (warmup = period)
indicators.ROC(s []float64, period int) []float64
```

---

## High/Low/Close indicators

```go
// ATR — average true range (warmup = period)
indicators.ATR(high, low, close []float64, period int) []float64

// WilliamsR — Williams %R in [-100, 0] (warmup = period-1)
indicators.WilliamsR(high, low, close []float64, period int) []float64

// Stochastic — slow %K and %D lines
// (warmup = (fastK-1) + (slowK-1) + (slowD-1))
indicators.Stochastic(high, low, close []float64, fastK, slowK, slowD int) (k, d []float64)

// ADX — average directional movement index in [0, 100].
// Double-smoothed (DM smoothing, then DX smoothing), so warmup = 2*period-1,
// NOT period-1 like a single-smoothed indicator.
indicators.ADX(high, low, close []float64, period int) []float64

// CCI — commodity channel index; unbounded (warmup = period-1)
indicators.CCI(high, low, close []float64, period int) []float64
```

---

## Volume indicators

```go
// OBV — on-balance volume, a cumulative running total from bar 0 (warmup = 0).
// close and volume must be length-aligned.
indicators.OBV(close, volume []float64) []float64

// VWAP — running volume-weighted average price of the typical price (H+L+C)/3,
// cumulative from bar 0 (warmup = 0). NOT session-anchored — intraday daily
// resets are a caller concern (slice per session and concatenate). Bars with
// zero cumulative volume yield NaN.
indicators.VWAP(high, low, close, volume []float64) []float64
```

---

## Multi-output indicators

```go
// MACD — macd line, signal line, histogram (warmup = slow+signal-2)
macd, sig, hist := indicators.MACD(s []float64, fast, slow, signal int)

// Bollinger Bands — upper, middle, lower (warmup = period-1)
upper, mid, lower := indicators.Bollinger(s []float64, period int, dev float64)

// Donchian channel — upper (highest high), mid (midpoint), lower (lowest low)
// over the period (warmup = period-1)
upper, mid, lower := indicators.Donchian(high, low []float64, period int)
```

---

## Composition: EMA of Williams %R

Because every indicator function preserves leading NaN, they chain cleanly:

```go
func (s *myStrat) Init(st *backtest.State) {
    h := st.Data().High()
    l := st.Data().Low()
    c := st.Data().Close()

    // Williams %R(14), then smooth with EMA(9).
    // The combined warmup is (14-1) + (9-1) = 21 bars.
    wrPct := indicators.WilliamsR(h, l, c, 14)   // []float64, NaN for first 13 bars
    emaWR  := indicators.EMA(wrPct, 9)            // []float64, NaN for first 21 bars

    s.wr  = st.I("WR%14",    func() []float64 { return wrPct }, backtest.Color("#e91e63"))
    s.ema = st.I("EMA-WR9",  func() []float64 { return emaWR }, backtest.Color("#2962FF"))
}

func (s *myStrat) Next(st *backtest.State) {
    wr  := s.wr.Last()
    ema := s.ema.Last()

    // Buy when WR crosses above its EMA from oversold territory.
    if lib.Crossover(s.wr.Series(), s.ema.Series()) && wr < -50 && st.Position().Size() == 0 {
        st.Buy(backtest.Order{Size: 0.99})
    }
    if lib.CrossUnder(s.wr.Series(), s.ema.Series()) && st.Position().IsLong() {
        st.Position().Close()
    }
    _ = ema // suppress "unused" warning when using the series form above
}
```

---

## NaN warmup

Every indicator is "warm" after a fixed number of bars. The engine's `maxLeadingNaN`
scan detects the warmup automatically and skips those bars entirely — `Next` is never
called before all registered indicators have a valid value. This means you can safely
call `ind.Last()` in `Next` without a NaN guard.

| Indicator | Warmup bars |
|-----------|------------|
| `SMA(n)` | n-1 |
| `EMA(n)` | n-1 |
| `WMA(n)` | n-1 |
| `RSI(n)` | n |
| `ATR(n)` | n |
| `ROC(n)` | n |
| `WilliamsR(n)` | n-1 |
| `MACD(fast,slow,sig)` | slow+sig-2 |
| `Bollinger(n,dev)` | n-1 |
| `Stochastic(fK,sK,sD)` | (fK-1)+(sK-1)+(sD-1) |
| `ADX(n)` | 2n-1 (double-smoothed — twice the warmup you might expect) |
| `CCI(n)` | n-1 |
| `OBV` | 0 (cumulative from bar 0) |
| `Donchian(n)` | n-1 |
| `VWAP` | 0 (cumulative from bar 0) |
| Composition `EMA(WilliamsR(h,l,c,14),9)` | (14-1)+(9-1) = 21 |

---

## Crossovers (lib package)

```go
// Crossover reports whether a crossed above b on the most recent bar.
lib.Crossover(a, b []float64) bool

// CrossUnder reports whether a crossed below b on the most recent bar.
lib.CrossUnder(a, b []float64) bool
```

Both accept `[]float64`; use `ind.Series()` to convert an `*Indicator` handle:

```go
lib.Crossover(s.fast.Series(), s.slow.Series())
```

`Crossover` returns `false` if `a` and `b` have different lengths, or if either
of the last two values is NaN.

---

## Indicator handle API

`st.I(...)` returns a `*backtest.Indicator` handle. Inside `Next`:

```go
ind.Last()      // value at the current bar
ind.At(0)       // same as Last()
ind.At(1)       // one bar ago
ind.Len()       // bars visible so far (same as data view length)
ind.Series()    // []float64 subslice up to the current bar — pass to lib.Crossover
```

---

## Registering indicators

Always register in `Init`, never in `Next`. Each name must be unique within a run.
The compute closure receives a full-length data view and must return a slice of the
same length as the dataset.

```go
func (s *myStrat) Init(st *backtest.State) {
    c := st.Data().Close()
    // OK: registered once, computed once.
    s.sma = st.I("SMA20", func() []float64 { return indicators.SMA(c, 20) })

    // Pass plot options as additional arguments.
    s.ema = st.I("EMA9", func() []float64 { return indicators.EMA(c, 9) },
        backtest.Overlay(),
        backtest.Color("#2962FF"),
    )
}
```
