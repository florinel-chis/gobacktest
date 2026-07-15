# Data

gobacktest stores OHLCV as a struct-of-arrays (`*backtest.Data`). During a run,
the engine grows a current-bar *view* one bar at a time so future data is never
visible inside `Next` — the anti-look-ahead guarantee.

---

## Constructors

### FromCSV

```go
data, err := backtest.FromCSV("AAPL_1d.csv")
```

Reads a CSV with a header row. Required columns (case-insensitive):
`Date`, `Open`, `High`, `Low`, `Close`, `Volume`.
Dates are parsed as `2006-01-02` or RFC3339.

```
Date,Open,High,Low,Close,Volume
2024-01-02,185.19,185.88,182.79,185.20,79488000
2024-01-03,184.22,185.88,183.43,184.40,58400000
...
```

### FromOHLCV

```go
func FromOHLCV(
    t      []time.Time,
    open, high, low, close, volume []float64,
) (*backtest.Data, error)
```

All slices must be the same length. The function retains (does not copy) the
slices; do not mutate them after construction.

```go
import "time"

t := []time.Time{
    time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
    time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC),
}
o := []float64{185.19, 184.22}
h := []float64{185.88, 185.88}
l := []float64{182.79, 183.43}
c := []float64{185.20, 184.40}
v := []float64{79488000, 58400000}

data, err := backtest.FromOHLCV(t, o, h, l, c, v)
```

### FromBars

```go
func FromBars(bars []backtest.Bar) *backtest.Data
```

Convenience constructor for an array-of-structs source (no error return):

```go
bars := []backtest.Bar{
    {Time: time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC),
     Open: 185.19, High: 185.88, Low: 182.79, Close: 185.20, Volume: 79488000},
}
data := backtest.FromBars(bars)
```

---

## Data / Series model

### Accessors (inside Init / Next)

```go
// Inside Init or Next — the view is limited to bars seen so far.
c := st.Data().Close() // backtest.Series ([]float64 subslice)
h := st.Data().High()
l := st.Data().Low()
o := st.Data().Open()
v := st.Data().Volume()
t := st.Data().Time()  // []time.Time
```

### Series methods

`backtest.Series` is `type Series []float64`. All methods operate on the *current view*.

```go
c.Last()   // most recent close (current bar)
c.At(0)    // same as Last()
c.At(1)    // one bar ago
c.Len()    // number of bars visible so far
```

### Full dataset (outside a run)

```go
bars := data.Bars()    // []backtest.Bar — full dataset, not view-limited
n    := data.FullLen() // total bar count
```

`Bars()` is used by `report.Generate` to build the OHLC chart.

---

## Notes

- The current-bar view is zero-indexed: `Close()[0]` is the *oldest* visible bar;
  `Close().Last()` is the current bar.
- Do not call `data.Bars()` inside `Next` to read future bars — the `Data` pointer
  returned by `st.Data()` is the same object but its view is already limited.
  Use `At(i)` or `Last()` for historical access.

---

## Data sources

The `source` package defines a provider-agnostic contract so strategies and
tools can fetch OHLCV data without depending on a specific provider:

```go
package source

// Canonical intervals: M1 "1m", M5 "5m", M10 "10m", M15 "15m", M30 "30m",
// H1 "1h", H4 "4h", D1 "1d", W1 "1wk", Mo1 "1mo".
type Interval string

// Source fetches OHLCV bars for a symbol in [start, end).
type Source interface {
    Fetch(ctx context.Context, symbol string, start, end time.Time, interval Interval) (*backtest.Data, error)
}
```

Providers that cannot serve an interval return an error wrapping the
`source.ErrUnsupportedInterval` sentinel (match with `errors.Is`).

This module intentionally ships no `source.Source` implementations — live-data
providers are separate modules that each expose a `backtestsource` sub-module
implementing the interface:

- [`oanda-go`](https://github.com/florinel-chis/oanda-go) — Oanda REST v20 client
  (candles + trading).
- [`yahoo-go`](https://github.com/florinel-chis/yahoo-go) — Yahoo Finance downloader.

Import a provider's `backtestsource` package, construct its `source.Source`, and
pass it a symbol/interval/date-range the same way regardless of provider:

```go
var src source.Source = backtestsource.New(...) // from oanda-go or yahoo-go
data, err := src.Fetch(ctx, "AAPL", start, end, source.D1)
```
