# gobacktest

A clean-room Go port of [backtesting.py](https://github.com/kernc/backtesting.py): a bar-by-bar
backtesting engine, ~32 performance statistics, a grid/random optimizer, and a self-contained
HTML report — reimplemented from documented behaviour and formula specs, not translated from
the (AGPL) Python source. All statistics are parity-validated against backtesting.py: **252/252**
metrics match to the documented decimal place, driven from the same committed CSV fixtures fed
to both engines.

## Install

```
go get github.com/florinel-chis/gobacktest
```

Requires the Go version in [go.mod](go.mod).

## Quickstart

```go
package main

import (
	"fmt"

	backtest "github.com/florinel-chis/gobacktest"
	"github.com/florinel-chis/gobacktest/indicators"
	"github.com/florinel-chis/gobacktest/lib"
	"github.com/florinel-chis/gobacktest/report"
)

// smaCross is the example strategy: buy on SMA fast/slow crossover, exit on cross-under.
type smaCross struct {
	n1, n2     int
	fast, slow *backtest.Indicator
}

func (s *smaCross) Init(st *backtest.State) {
	c := st.Data().Close()
	s.fast = st.I(fmt.Sprintf("SMA%d", s.n1), func() []float64 { return indicators.SMA(c, s.n1) },
		backtest.Overlay(), backtest.Color("#2962FF"))
	s.slow = st.I(fmt.Sprintf("SMA%d", s.n2), func() []float64 { return indicators.SMA(c, s.n2) },
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

	// --- Single run: SMA(10/20) crossover ---
	bt := backtest.New(data, &smaCross{n1: 10, n2: 20}, backtest.Options{Cash: 10000, Margin: 1, FinalizeTrades: true})
	res, err := bt.Run()
	if err != nil {
		panic(err)
	}
	stats := backtest.Compute(res, data, 0)

	fmt.Println("=== AAPL SMA(10/20) Crossover ===")
	fmt.Print(stats.String())
	fmt.Println()

	out := "/tmp/gobacktest-example.html"
	if err := report.Generate(data, res, stats, out, report.Title("AAPL SMA Crossover")); err != nil {
		panic(err)
	}
	fmt.Printf("report written to %s\n\n", out)

	// --- Optimize: grid-search n1 ∈ {5,10,15}, n2 ∈ {20,30,40}, constraint n1<n2 ---
	fmt.Println("=== Optimize SMA periods (maximize SQN) ===")
	opt, err := backtest.Optimize(
		data,
		backtest.Options{Cash: 10000, Margin: 1, FinalizeTrades: true},
		func(p backtest.Params) backtest.Strategy {
			n1 := p["n1"].(int)
			n2 := p["n2"].(int)
			return &smaCross{n1: n1, n2: n2}
		},
		map[string][]any{
			"n1": {5, 10, 15},
			"n2": {20, 30, 40},
		},
		backtest.OptimizeOptions{
			Maximize:   "SQN",
			Constraint: func(p backtest.Params) bool { return p["n1"].(int) < p["n2"].(int) },
		},
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Best params: n1=%v n2=%v\n", opt.Best["n1"], opt.Best["n2"])
	fmt.Printf("Best SQN:    %.3f\n", opt.BestValue)
}
```

Run it (this is exactly `cmd/example`):

```
go run ./cmd/example
```

## Features

| Area | Details |
|------|---------|
| **Engine** | Bar-by-bar, anti-look-ahead (orders submitted in `Next` fill at the *next* bar's open); absolute **and fractional** (%-of-equity) sizing; SL/TP contingent orders (gap-correct fills); margin/leverage/hedging/exclusive-orders modes |
| **Statistics** | ~32 metrics mirroring backtesting.py: Sharpe, Sortino, Calmar, CAGR, SQN, Kelly, drawdown, win rate, profit factor, and more (degenerate inputs → NaN) |
| **Optimize** | Grid + random search, constraint filtering, parallel workers, custom objective; `report.Heatmap` renders the 2-param grid |
| **Report** | `report.Generate` — self-contained, offline HTML embedding TradingView Lightweight Charts v5 (vendored, Apache-2.0): OHLC, volume, equity curve, drawdown, overlay indicators, oscillator subpanes, trade markers, theme-aware |
| **Indicators** | `SMA`, `EMA`, `WMA`, `RSI`, `ATR`, `MACD`, `Bollinger`, `WilliamsR`, `Stochastic`, `ROC`, `ADX`, `CCI`, `OBV`, `Donchian`, `VWAP`; composable (e.g. EMA of Williams %R); NaN-warmup propagation |
| **Strategies** | `strategies` package: reusable, parameterized `Strategy` implementations (e.g. `WilliamsROversold`, `AveragingGrid`) — configure exported fields, pass to `backtest.New` or a factory for `backtest.Optimize` |
| **Source interface** | `source` package: provider-agnostic OHLCV fetch contract (`source.Source`, canonical `source.Interval`, `ErrUnsupportedInterval`) so strategies and tools don't depend on a specific data provider |
| **Costs** | `costs` package: trading-cost math the engine doesn't model natively (per-trade holding/financing cost) |

## Data sources

The core engine is data-source agnostic: `backtest.FromCSV`, `FromOHLCV`, and `FromBars`
construct a `*backtest.Data` from any OHLCV source. The `source` package defines a small
contract (`source.Source`, canonical `source.Interval`) so live-data providers can be plugged
in without the engine or strategies depending on any one of them.

Live-data providers are published as separate modules with their own `backtestsource`
sub-module implementing `source.Source`:

- [`oanda-go`](https://github.com/florinel-chis/oanda-go) — Oanda REST v20 client (candles +
  trading), with a `backtestsource` adapter.
- [`yahoo-go`](https://github.com/florinel-chis/yahoo-go) — Yahoo Finance downloader, with a
  `backtestsource` adapter.

See [docs/data.md](docs/data.md) for the full contract and constructors.

## Parity

gobacktest is validated against backtesting.py using a matrix of golden fixtures:

```
make parity   # requires a Python venv; see scripts/
```

The parity test suite (`parity_test.go`, build tag `parity`) reads the same committed
CSV files that the Python `scripts/gen_fixtures.py` script used to produce the reference
output. **252 statistic metrics** across the fixture matrix are checked to 1e-6. The
invariant: **never change a fixture CSV without re-running `gen_fixtures.py` to regenerate
the reference JSON.**

```
cd scripts
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
python gen_fixtures.py        # regenerates testdata/*.json reference files
cd ..
make parity
```

## Documentation

| Guide | Contents |
|-------|---------|
| [Getting started](docs/getting-started.md) | Install, minimal backtest, running locally |
| [Data](docs/data.md) | `FromCSV`, `FromOHLCV`, `FromBars`, the `source` contract, `Series` model |
| [Strategies](docs/strategies.md) | `Strategy` interface, `State` API, order lifecycle |
| [ML signals](docs/ml-signals.md) | Backtesting an external model's per-bar signal (e.g. a Kronos dip-buy), with `strategies.SignalStrategy` and `cmd/mlsignal` |
| [Indicators](docs/indicators.md) | All built-in indicators, composition, NaN warmup, crossovers |
| [Statistics](docs/statistics.md) | `Compute`, all ~32 metrics, `Stats.String()` |
| [Optimization](docs/optimization.md) | `Optimize`, build-factory pattern, grid/random/heatmap |
| [Reports](docs/reports.md) | `report.Generate`, panes, offline HTML |
| [CONTRIBUTING.md](CONTRIBUTING.md) | Contributor guide: architecture, commands, conventions |

## Security

See [SECURITY.md](SECURITY.md) for the local scanning policy and current status.

```
make tools     # install govulncheck, gosec, osv-scanner
make security  # run all three
make check     # vet + test + security (full local gate)
```

## License

gobacktest is released under the [MIT License](LICENSE).

This product bundles **TradingView Lightweight Charts™ v5.2.0** (Apache-2.0), vendored at
`report/assets/lightweight-charts.standalone.production.js` and embedded into every generated
`report.html`. See [NOTICE](NOTICE) for full attribution.

The `go-talib` Go module (MIT) is used by the `indicators` and `lib` packages; it is not
imported by the core engine.
