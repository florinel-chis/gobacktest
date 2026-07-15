# Reports

`report.Generate` produces a self-contained HTML file that embeds
[TradingView Lightweight Charts v5](https://tradingview.github.io/lightweight-charts/)
(vendored, Apache-2.0) and the backtest data as inline JSON. The result opens in any
browser with no network access required.

---

## Signature

```go
import "github.com/florinel-chis/gobacktest/report"

func Generate(
    data   *backtest.Data,
    result *backtest.Result,
    stats  backtest.Stats,
    path   string,
    opts   ...report.Option,
) error
```

- `data` — the same `*Data` used for the run (builds OHLC + volume panes).
- `result` — the `*Result` returned by `bt.Run()`.
- `stats` — the `Stats` returned by `backtest.Compute(...)`.
- `path` — output file path (e.g. `"report.html"` or `"/tmp/gobacktest-report.html"`).
- `opts` — optional configuration (see below).

Returns an error if the file cannot be created or the template cannot be rendered.

---

## Options

```go
report.Title("AAPL — SMA(10/20) Crossover")
```

The only current option is `Title`, which sets the HTML `<title>` and the heading
displayed at the top of the HTML file. Defaults to `"Backtest Report"`.

---

## Example

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

    out := "/tmp/gobacktest-report.html"
    if err := report.Generate(data, res, stats, out,
        report.Title("AAPL — SMA(10/20) Crossover")); err != nil {
        panic(err)
    }
    fmt.Printf("wrote %s  (%d trades, %.2f%%)\n", out, stats.NumTrades, stats.ReturnPct)
}
```

Or use the Makefile target:

```
make report   # go run ./cmd/genreport → /tmp/gobacktest-report.html
```

---

## Panes

The HTML output contains five panes rendered by Lightweight Charts v5:

| Pane | Contents |
|------|---------|
| **Price** | Candlestick (OHLC); overlay indicators drawn here; buy/sell trade markers |
| **Volume** | Bar histogram; green on up-days, red on down-days |
| **Equity** | Equity curve as a line series |
| **Drawdown** | Drawdown series (always ≤ 0%) as a histogram |
| **Statistics** | Two-column table of all ~32 metrics (mirrors `Stats.String()`) |

Non-overlay indicators (e.g. RSI, WilliamsR) are rendered in a separate sub-pane
below the price chart.

---

## Indicator colours

If a `Color` option was passed to `st.I(...)`, the indicator uses that colour.
Otherwise, the HTML output cycles through a default 10-colour palette:

```
#2962FF  #ff6d00  #00bcd4  #e91e63  #4caf50
#9c27b0  #ff9800  #03a9f4  #f44336  #8bc34a
```

---

## Offline use

The generated HTML file is fully self-contained:
- Lightweight Charts v5 JavaScript is embedded inline (no CDN call needed).
- OHLC, equity, indicator, and stats data is embedded as inline JSON.
- No fonts, images, or external resources are fetched at render time.

The file can be archived, emailed, or opened on an air-gapped machine.

---

## Implementation notes

- `report.Generate` uses `data.Bars()` (the full dataset) to build OHLC and
  volume series — this is correct even if the run started after a warmup period.
- The drawdown series is recomputed from the equity curve inside the package;
  it does not use `Stats.MaxDrawdownPct`.
- `path` is a caller-supplied argument; the package annotates the `os.Create` call
  with `// #nosec G304` per the project's security policy.
