# Statistics

`backtest.Compute` derives ~32 performance metrics from a `*Result` and its `*Data`.
All formulas mirror backtesting.py's `compute_stats()` and are parity-validated against
it using committed CSV fixtures.

---

## Compute

```go
func Compute(r *backtest.Result, d *backtest.Data, riskFreeRate float64) backtest.Stats
```

- `r` — the `*Result` returned by `bt.Run()`.
- `d` — the same `*Data` used to create the `Backtest`.
- `riskFreeRate` — annualised risk-free rate (e.g. `0.02` for 2%). Pass `0` for the
  default (matches backtesting.py's default).

```go
res, err := bt.Run()
if err != nil {
    panic(err)
}
stats := backtest.Compute(res, data, 0)
fmt.Println(stats) // formatted table (see below)
```

---

## Stats fields

```go
type Stats struct {
    // Time window (full data range, including indicator warmup)
    Start            time.Time
    End              time.Time
    Duration         time.Duration

    // Equity
    ExposureTimePct  float64   // fraction of bars with an open position
    EquityFinal      float64
    EquityPeak       float64
    ReturnPct        float64
    BuyHoldReturnPct float64

    // Annualised / risk-adjusted
    ReturnAnnPct     float64
    VolatilityAnnPct float64
    CAGRPct          float64
    SharpeRatio      float64
    SortinoRatio     float64
    CalmarRatio      float64
    AlphaPct         float64   // Jensen CAPM alpha
    Beta             float64

    // Drawdown
    MaxDrawdownPct   float64
    AvgDrawdownPct   float64
    MaxDrawdownDur   time.Duration
    AvgDrawdownDur   time.Duration

    // Trade statistics
    NumTrades        int
    WinRatePct       float64
    BestTradePct     float64
    WorstTradePct    float64
    AvgTradePct      float64
    MaxTradeDur      time.Duration
    AvgTradeDur      time.Duration
    ProfitFactor     float64
    ExpectancyPct    float64   // arithmetic mean return per trade
    SQN              float64   // System Quality Number
    KellyCriterion   float64

    // Cost
    CommissionsTotal float64
}
```

---

## String output

`Stats.String()` returns a formatted two-column table mirroring backtesting.py's
`print(stats)` output (label left-aligned, value right-aligned):

```
Start                            2020-01-02 00:00:00
End                              2023-12-29 00:00:00
Duration                              1457 days
Exposure Time [%]                         42.31
Equity Final [$]                       14823.56
Equity Peak [$]                        16204.10
Return [%]                                48.24
Buy & Hold Return [%]                    181.20
Return (Ann.) [%]                         10.51
Volatility (Ann.) [%]                     19.32
CAGR [%]                                  10.48
Sharpe Ratio                              0.544
Sortino Ratio                             0.812
Calmar Ratio                              0.371
Alpha [%]                                -98.45
Beta                                      0.5234
Max. Drawdown [%]                        -28.30
Avg. Drawdown [%]                         -6.14
Max. Drawdown Duration                  392 days
Avg. Drawdown Duration                   51 days
# Trades                                      47
Win Rate [%]                              57.45
Best Trade [%]                            24.10
Worst Trade [%]                          -12.60
Avg. Trade [%]                             0.82
Max. Trade Duration                      84 days
Avg. Trade Duration                      12 days
Profit Factor                             1.432
Expectancy [%]                             1.14
SQN                                        1.803
Kelly Criterion                            0.1234
```

---

## Annualisation factor

The annualisation factor `N` mirrors backtesting.py's weekend-detection heuristic:

- **Daily data with weekends** (5/7 trading days): `N = 365`
- **Daily data without weekends** (true trading days): `N = 252`
- **Weekly**: `N = 52`
- **Monthly**: `N = 12`

---

## Parity

All 32+ metrics are validated against backtesting.py using the golden-fixture approach:
the same committed CSV feeds both the Go and Python engines. Run the check with:

```
make parity
```

(Requires a `.venv` with `backtesting` installed; see `scripts/` for setup.)
