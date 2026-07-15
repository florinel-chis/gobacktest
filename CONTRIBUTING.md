# Contributing to gobacktest

Navigational reference for contributors. For usage docs, see `docs/`.

---

## Project purpose

gobacktest is a faithful Go port of [backtesting.py](https://github.com/kernc/backtesting.py).
It reimplements the bar-by-bar engine, all ~32 statistics, and the HTML report from
documented behaviour and formula specs — not by translating AGPL source (clean-room). All
metrics are parity-validated against the Python library.

---

## Package map

| Package | Path | Role |
|---------|------|------|
| `backtest` | `.` (root) | Core engine: `Data`, `Strategy`, `State`, `Backtest`, `Result`, `Stats`, `Optimize`; stdlib-only |
| `indicators` | `indicators/` | TA indicator wrappers around `go-talib`; NaN-warmup + composition |
| `lib` | `lib/` | Crossover helpers (`Crossover`, `CrossUnder`); imports `backtest` (for `TrailStop(*State)`) — so parity_test.go is `package backtest_test` to avoid a cycle |
| `report` | `report/` | Self-contained HTML output via embedded Lightweight Charts v5; stdlib-only |
| `source` | `source/` | Provider-agnostic data-source contract: canonical `Interval`, `Source` interface, `ErrUnsupportedInterval`; stdlib-only |
| `strategies` | `strategies/` | Reusable parameterized `Strategy` implementations (`WilliamsROversold`, `AveragingGrid`) |
| `costs` | `costs/` | Trading-cost math the engine doesn't model natively (per-trade holding/financing cost) |
| `cmd/example` | `cmd/example/` | README quickstart as a runnable program: SMA(10/20) crossover on AAPL → stats + HTML report + Optimize demo |
| `cmd/genreport` | `cmd/genreport/` | Demo binary: SMA-cross on AAPL → writes HTML |
| `cmd/genheatmap` | `cmd/genheatmap/` | Demo binary: SMA-crossover Optimize over an n1×n2 grid on AAPL → HTML heatmap (`report.Heatmap`) |

---

## Dependency boundary

The **core engine** (`backtest` package) and **report** package are deliberately
**stdlib-only** — no third-party Go imports. This is an invariant; do not add external
dependencies to either package.

`go-talib` is only imported by `indicators/`. Adding a new indicator belongs
in `indicators/indicators.go`; adding a new crossover helper belongs in `lib/cross.go`.

Live-data providers (Oanda, Yahoo Finance, etc.) are out of scope for this module by design —
they live in their own repos and plug in via the `source.Source` interface. See
[docs/data.md](docs/data.md).

---

## Key commands

```
make test       # hermetic unit + integration tests (no network)
make vet        # go vet ./...
make race       # race detector on all packages
make check      # vet + test + security (the full local gate)
make security   # govulncheck + gosec + osv-scanner (install first with make tools)
make tools      # install the three security scanners
make parity     # parity matrix vs backtesting.py (needs .venv — see scripts/)
make report     # run cmd/genreport → /tmp/gobacktest-report.html
```

### Parity setup

Parity tests use build tag `parity` and are in `parity_test.go`. They read committed
CSV fixtures from `testdata/` and compare Go stats against reference JSON produced by
`scripts/gen_fixtures.py`. To set up:

```
cd scripts
python -m venv .venv && source .venv/bin/activate
pip install -r requirements.txt
python gen_fixtures.py        # regenerates testdata/*.json reference files
cd ..
make parity
```

**Invariant:** never change a fixture CSV without re-running `gen_fixtures.py` to
regenerate the matching JSON.

---

## Conventions

### Test-driven development

Write a failing test first (unit or parity), implement until it passes, then run
`make check`. All new public APIs need a test in `*_test.go` alongside the source file.

### Parity invariant

The parity suite feeds both Go and Python the **identical committed CSV file**. Do not
use live data in parity tests. If a stat formula changes, update the reference JSON by
re-running `gen_fixtures.py`.

### Anti-look-ahead loop order

The engine loop per bar:
1. Grow data view (set `n = i+1`).
2. Process queued orders → fill at bar `i`'s open.
3. Process contingent SL/TP orders.
4. Call `strategy.Next(st)`.

Orders submitted in `Next` at bar `i` fill at bar `i+1`'s open. **Never** invert this
order — this is the single highest-risk correctness area in the engine.

### NaN-warmup indicators

Every indicator function in `indicators/` returns a **full-length** `[]float64` slice
with leading `math.NaN()` for the warmup bars. The engine's `maxLeadingNaN` scan skips
these bars automatically — `Next` is never called with a NaN-valued indicator.

New indicators must follow the same NaN-warmup contract. Use `apply` or `applyHLC` from
`indicators/apply.go` as the implementation helper.

### #nosec policy

gosec annotations are allowed only when the usage is safe by design:
- Include the rule ID and a concise reason on the same line:
  `// #nosec G304 -- caller-supplied API argument`
- Never use blanket disables (e.g. `-exclude G304`).
- Add a row to the SECURITY.md table for every new annotation.

See `SECURITY.md` for the current list of annotated sites.

---

## Where to look

| Topic | Location |
|-------|---------|
| Engine implementation | `backtest.go`, `broker.go`, `strategy.go`, `indicator.go` |
| Data constructors | `data.go`, `data_constructors.go`, `csv.go` |
| Statistics formulas | `stats.go`, `stats_helpers.go`, `stats_string.go` |
| Optimize | `optimize.go`, `optimize_stat.go` |
| Trade / Order / Position | `trade.go`, `order.go`, `position.go` |
| Commission types | `commission.go` |
| Indicator wrappers | `indicators/indicators.go`, `indicators/apply.go` |
| Crossovers | `lib/cross.go` |
| HTML report | `report/report.go`, `report/templates/`, `report/assets/` |
| Data-source contract | `source/source.go` |
| Reusable strategies | `strategies/williamsr.go`, `strategies/grid.go` |
| Trading-cost math | `costs/costs.go` |
| Working example | `cmd/example/main.go` (quickstart: data → SMA crossover → stats → report + Optimize demo); also `cmd/genreport/main.go` |
| Parity tests | `parity_test.go` (build tag `parity`) |

---

## Usage docs

For end-user API reference and runnable snippets, see:

- [Getting started](docs/getting-started.md)
- [Data](docs/data.md)
- [Strategies](docs/strategies.md)
- [Indicators](docs/indicators.md)
- [Statistics](docs/statistics.md)
- [Optimization](docs/optimization.md)
- [Reports](docs/reports.md)
