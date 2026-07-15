# Optimization

`backtest.Optimize` searches a parameter space for the combination that maximises a
chosen statistic. It mirrors backtesting.py's `Backtest.optimize()` semantics: grid
search by default, random sampling when `MaxTries` is set, and parallel execution
via a bounded worker pool.

---

## Signature

```go
func Optimize(
    data  *backtest.Data,
    opts  backtest.Options,
    build func(backtest.Params) backtest.Strategy,
    space map[string][]any,
    oo    backtest.OptimizeOptions,
) (*backtest.OptimizeResult, error)
```

- `data` — the dataset (each worker gets its own `clone()`; the original is not mutated).
- `opts` — engine options applied to every run.
- `build` — a factory that constructs a fresh `Strategy` for each parameter combination.
  Must be safe to call concurrently; must not capture shared mutable state.
- `space` — the parameter grid. Keys are arbitrary strings; values are slices of `any`.
- `oo` — optimization configuration (see below).

---

## OptimizeOptions

```go
type OptimizeOptions struct {
    Maximize      string                   // stat name to maximise (default "SQN")
    MaximizeFunc  func(backtest.Stats) float64 // custom objective; overrides Maximize
    Constraint    func(backtest.Params) bool   // filter combos before evaluation
    MaxTries      int                      // 0 = full grid; >0 = random subset
    RandomSeed    int64                    // seed for reproducible random sampling
    Workers       int                      // goroutines; 0 = GOMAXPROCS
    ReturnHeatmap bool                     // populate OptimizeResult.Heatmap
    RiskFreeRate  float64                  // forwarded to Compute
}
```

Valid `Maximize` strings (must match `Stats.String()` labels exactly):

`"SQN"`, `"Sharpe Ratio"`, `"Sortino Ratio"`, `"Calmar Ratio"`, `"Return [%]"`,
`"Equity Final [$]"`, `"Profit Factor"`, `"Win Rate [%]"`, `"Expectancy [%]"`,
`"Max. Drawdown [%]"`.

---

## OptimizeResult

```go
type OptimizeResult struct {
    Best      backtest.Params  // best parameter combination (read-only)
    BestStats backtest.Stats   // Stats for the best run
    BestValue float64          // metric value for Best
    Heatmap   []HeatmapEntry   // all evaluated combos (when ReturnHeatmap = true)
}

type HeatmapEntry struct {
    Params backtest.Params
    Value  float64
    Stats  backtest.Stats
}
```

---

## Build-factory pattern

The `build` function is the canonical way to parameterise a strategy. Create a fresh
strategy struct per call; do not close over shared mutable state:

```go
type smaCross struct {
    fast, slow int
    fastInd, slowInd *backtest.Indicator
}

func (s *smaCross) Init(st *backtest.State) {
    c := st.Data().Close()
    s.fastInd = st.I("SMAfast", func() []float64 { return indicators.SMA(c, s.fast) })
    s.slowInd = st.I("SMAslow", func() []float64 { return indicators.SMA(c, s.slow) })
}

func (s *smaCross) Next(st *backtest.State) {
    switch {
    case lib.Crossover(s.fastInd.Series(), s.slowInd.Series()) && st.Position().Size() == 0:
        st.Buy(backtest.Order{Size: 10})
    case lib.CrossUnder(s.fastInd.Series(), s.slowInd.Series()) && st.Position().IsLong():
        st.Position().Close()
    }
}

// build constructs a fresh smaCross from Params.
func build(p backtest.Params) backtest.Strategy {
    return &smaCross{
        fast: p["fast"].(int),
        slow: p["slow"].(int),
    }
}
```

---

## Grid search example

```go
package main

import (
    "fmt"

    backtest "github.com/florinel-chis/gobacktest"
    "github.com/florinel-chis/gobacktest/indicators"
    "github.com/florinel-chis/gobacktest/lib"
)

func main() {
    data, err := backtest.FromCSV("testdata/AAPL_1d.csv")
    if err != nil {
        panic(err)
    }

    space := map[string][]any{
        "fast": {5, 10, 15, 20},
        "slow": {20, 30, 40, 50},
    }

    opts := backtest.Options{Cash: 10_000, FinalizeTrades: true}

    res, err := backtest.Optimize(data, opts, build, space, backtest.OptimizeOptions{
        Maximize:      "SQN",
        Constraint:    func(p backtest.Params) bool { return p["fast"].(int) < p["slow"].(int) },
        ReturnHeatmap: true,
    })
    if err != nil {
        panic(err)
    }

    fmt.Printf("best: fast=%d slow=%d  SQN=%.3f\n",
        res.Best["fast"], res.Best["slow"], res.BestValue)
    fmt.Println(res.BestStats)
}
```

---

## Random sampling

Set `MaxTries` to evaluate a random subset of the grid (without replacement):

```go
backtest.OptimizeOptions{
    Maximize:   "Sharpe Ratio",
    MaxTries:   50,
    RandomSeed: 42, // reproducible
}
```

---

## Custom objective

Use `MaximizeFunc` for metrics not in the built-in list:

```go
backtest.OptimizeOptions{
    MaximizeFunc: func(s backtest.Stats) float64 {
        // Maximise Sharpe but penalise more than 50 trades.
        if s.NumTrades > 50 {
            return s.SharpeRatio - 1
        }
        return s.SharpeRatio
    },
}
```

---

## Running across many datasets — MultiBacktest

`MultiBacktest` runs a single strategy across many datasets in parallel — e.g. a
portfolio of symbols, or walk-forward folds — mirroring backtesting.py's
`lib.MultiBacktest`. Like `Optimize`, it needs a strategy **factory** (`func()
Strategy`) so each dataset gets its own fresh `Strategy` instance with independent
indicator state:

```go
datasets := []*backtest.Data{aapl, msft, spy, tsla}

mb := backtest.NewMultiBacktest(datasets, func() backtest.Strategy {
    return &SmaCross{N1: 10, N2: 20}
}, backtest.Options{Cash: 10_000, Commission: backtest.Pct(0.002)})

// One *Result per dataset, in input order.
results, err := mb.Run()

// One Stats per dataset, in input order.
stats, err := mb.Stats(0 /* riskFreeRate */)

// Mean of any metric across datasets (NaN values skipped).
meanReturn, err := mb.MeanStat(0, func(s backtest.Stats) float64 { return s.ReturnPct })
```

`Workers(n)` bounds the goroutine pool (0, the default, uses
`runtime.GOMAXPROCS(0)`), same as `OptimizeOptions.Workers`. Each dataset is run
exactly once, so — unlike `Optimize`, which clones the shared dataset per combo —
no cloning is needed: results are written to a pre-allocated slice indexed by
dataset position, giving a deterministic order regardless of goroutine scheduling.
An empty `datasets` slice returns `(nil, nil)`. If any dataset's run errors, `Run`
returns the first error by ascending dataset index.

---

## Notes

- Combos that run out of money (`ErrOutOfMoney`) are treated as the worst possible
  metric (`-math.MaxFloat64`) rather than a fatal error.
- The best result is the **first** combo (in grid enumeration order) that achieves
  the maximum value — deterministic regardless of goroutine scheduling.
- Grid enumeration is alphabetical by key: the key that sorts last varies fastest
  (innermost loop), matching backtesting.py's grid ordering.
