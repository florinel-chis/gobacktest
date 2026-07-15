# Getting Started

## Install

```
go get github.com/florinel-chis/gobacktest
```

Requires Go 1.26.5+. The core engine and report package are stdlib-only. The
`indicators` and `lib` packages pull in `go-talib` automatically via `go.mod`.

## Minimal backtest

```go
package main

import (
    "fmt"

    backtest "github.com/florinel-chis/gobacktest"
    "github.com/florinel-chis/gobacktest/indicators"
    "github.com/florinel-chis/gobacktest/lib"
)

// buyAndHold buys on bar 0 and never sells.
type buyAndHold struct{ sma *backtest.Indicator }

func (s *buyAndHold) Init(st *backtest.State) {
    c := st.Data().Close()
    s.sma = st.I("SMA20", func() []float64 { return indicators.SMA(c, 20) },
        backtest.Overlay())
}

func (s *buyAndHold) Next(st *backtest.State) {
    if lib.Crossover(s.sma.Series(), st.Data().Close()) && st.Position().Size() == 0 {
        st.Buy(backtest.Order{Size: 0.99}) // 99% of equity
    }
}

func main() {
    data, err := backtest.FromCSV("AAPL.csv")
    if err != nil {
        panic(err)
    }

    bt := backtest.New(data, &buyAndHold{}, backtest.Options{
        Cash:           10_000,
        Commission:     backtest.Pct(0.001), // 0.1 % per trade
        FinalizeTrades: true,
    })

    res, err := bt.Run()
    if err != nil {
        panic(err)
    }

    stats := backtest.Compute(res, data, 0)
    fmt.Println(stats)
}
```

## Running the built-in demo

The repository ships a ready-made SMA-crossover demo that writes a full HTML report:

```
make report
# equivalent: go run ./cmd/genreport
# writes: /tmp/gobacktest-report.html
```

## Local checks

```
make test      # hermetic test suite (no network)
make vet       # go vet
make check     # vet + test + security
make parity    # parity vs backtesting.py (needs .venv; see scripts/)
```

## Key concepts

| Concept | Summary |
|---------|---------|
| **Bar-by-bar loop** | `Next` is called once per bar with a growing view of the data; future bars are invisible. |
| **Order lifecycle** | Orders queued in `Next` are filled at the **next bar's open** (never same-bar); SL/TP orders are contingent. |
| **NaN warmup** | Indicators return `NaN` for bars before their warmup period; the engine skips those bars automatically. |
| **Indicator registration** | Call `st.I(...)` inside `Init`, **not** `Next`; the compute function runs once over the full dataset. |
