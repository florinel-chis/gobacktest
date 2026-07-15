#!/usr/bin/env python3
"""Generate golden parity fixtures from the real backtesting.py.

Each scenario runs backtesting.py on a committed CSV and dumps the full stats
series + trades + equity curve to testdata/parity/<name>.golden.json. The Go
parity test (parity_test.go) runs the behaviorally-identical Go strategy on the
same CSV and asserts per-metric parity. Requires backtesting>=0.3.
"""
import json, math, os, sys
import pandas as pd
from backtesting import Backtest, Strategy
from backtesting.lib import crossover, TrailingStrategy

OUT = "testdata/parity"
DATA = "testdata/AAPL_1d.csv"

def SMA(arr, n):
    return pd.Series(arr).rolling(n).mean()

class SmaCross(Strategy):
    n1 = 10
    n2 = 20
    def init(self):
        self.fast = self.I(SMA, self.data.Close, self.n1)
        self.slow = self.I(SMA, self.data.Close, self.n2)
    def next(self):
        if crossover(self.fast, self.slow):
            if not self.position:
                self.buy(size=10)
        elif crossover(self.slow, self.fast):
            if self.position.is_long:
                self.position.close()

class SmaCrossSLTP(Strategy):
    """SMA cross with fixed SL=5% below entry close and TP=10% above entry close."""
    n1 = 10
    n2 = 20
    def init(self):
        self.fast = self.I(SMA, self.data.Close, self.n1)
        self.slow = self.I(SMA, self.data.Close, self.n2)
    def next(self):
        if crossover(self.fast, self.slow):
            if not self.position:
                price = self.data.Close[-1]
                self.buy(size=10, sl=price * 0.95, tp=price * 1.10)
        elif crossover(self.slow, self.fast):
            if self.position.is_long:
                self.position.close()

class SmaCrossLeverage(Strategy):
    """SMA cross with fixed size=300 to exercise leverage (margin=0.5)."""
    n1 = 10
    n2 = 20
    def init(self):
        self.fast = self.I(SMA, self.data.Close, self.n1)
        self.slow = self.I(SMA, self.data.Close, self.n2)
    def next(self):
        if crossover(self.fast, self.slow):
            if not self.position:
                self.buy(size=300)
        elif crossover(self.slow, self.fast):
            if self.position.is_long:
                self.position.close()

class SmaCrossFrac(Strategy):
    """SMA cross using FRACTIONAL sizing — buy 50% of available equity."""
    n1 = 10
    n2 = 20
    def init(self):
        self.fast = self.I(SMA, self.data.Close, self.n1)
        self.slow = self.I(SMA, self.data.Close, self.n2)
    def next(self):
        if crossover(self.fast, self.slow):
            if not self.position:
                self.buy(size=.5)
        elif crossover(self.slow, self.fast):
            if self.position.is_long:
                self.position.close()

class CrossHoldOpen(Strategy):
    """Enters on the first bullish SMA cross (a data-determined bar, identical to
    the other cross scenarios) and NEVER exits, so the position is STILL OPEN at
    the last bar and finalize_trades must close it. Isolates the finalize
    exit-price path (last-bar open in backtesting.py) with an entry bar that
    aligns exactly across engines — a "buy on first bar" entry would instead
    expose the warmup-start convention, not the finalize price."""
    n1 = 10
    n2 = 20
    def init(self):
        self.fast = self.I(SMA, self.data.Close, self.n1)
        self.slow = self.I(SMA, self.data.Close, self.n2)
    def next(self):
        if crossover(self.fast, self.slow) and not self.position:
            self.buy(size=10)

class SmaCrossOpt(Strategy):
    """Parameterized SMA cross for optimize parity — n1/n2 are class attributes."""
    n1 = 10
    n2 = 20
    def init(self):
        self.fast = self.I(SMA, self.data.Close, self.n1)
        self.slow = self.I(SMA, self.data.Close, self.n2)
    def next(self):
        if crossover(self.fast, self.slow):
            if not self.position:
                self.buy(size=10)
        elif crossover(self.slow, self.fast):
            if self.position.is_long:
                self.position.close()

class SmaTrailing(TrailingStrategy):
    """SMA cross with a trailing stop-loss (backtesting.lib.TrailingStrategy)."""
    n1 = 10
    n2 = 20
    def init(self):
        super().init()
        self.set_atr_periods(20)
        self.set_trailing_sl(3)
        self.fast = self.I(SMA, self.data.Close, self.n1)
        self.slow = self.I(SMA, self.data.Close, self.n2)
    def next(self):
        super().next()   # trails the SL of open trades
        if crossover(self.fast, self.slow) and not self.position:
            self.buy(size=10)
        # exit is the trailing SL (no explicit close)

SCENARIOS = [
    # name, strategy, nrows, kwargs
    ("sma_aapl",        SmaCross,         250, dict(cash=10000, commission=0,     finalize_trades=True)),
    ("sltp_aapl",       SmaCrossSLTP,     250, dict(cash=10000, commission=0,     finalize_trades=True)),
    ("commission_aapl", SmaCross,         250, dict(cash=10000, commission=0.002, finalize_trades=True)),
    ("leverage_aapl",   SmaCrossLeverage, 250, dict(cash=10000, commission=0,     margin=0.5,  finalize_trades=True)),
    # Fractional (percentage-of-equity) sizing — the previously-unvalidated path.
    ("frac_aapl",       SmaCrossFrac,     250, dict(cash=10000, commission=0,     finalize_trades=True)),
    ("frac_comm_aapl",  SmaCrossFrac,     250, dict(cash=10000, commission=0.002, finalize_trades=True)),
    ("frac_lev_aapl",   SmaCrossFrac,     250, dict(cash=10000, commission=0,     margin=0.5,  finalize_trades=True)),
    ("trailing_aapl",   SmaTrailing,      250, dict(cash=10000, commission=0,     finalize_trades=True)),
    # Position left OPEN at the last bar → exercises the finalize exit-price path.
    ("buyhold_aapl",    CrossHoldOpen,    250, dict(cash=10000, commission=0,     finalize_trades=True)),
]

def _scalar(v):
    """Convert a stats value to a JSON-safe scalar.

    NaN / Inf → None (JSON null) so the Go harness's toFloat returns (0,false)
    and skips the comparison rather than comparing 0 vs NaN.
    """
    if isinstance(v, float):
        if math.isnan(v) or math.isinf(v):
            return None
        return v
    if isinstance(v, (int, float)):
        return v
    return str(v)

def dump(name, stats):
    trades = stats["_trades"][["Size","EntryBar","ExitBar","EntryPrice","ExitPrice","PnL","ReturnPct"]].to_dict("records")
    equity = list(stats["_equity_curve"]["Equity"])
    scalars = {k: _scalar(v) for k, v in stats.items() if not k.startswith("_")}
    obj = {"stats": scalars, "trades": trades, "equity": equity}
    os.makedirs(OUT, exist_ok=True)
    with open(f"{OUT}/{name}.golden.json", "w") as f:
        json.dump(obj, f, indent=2, default=str)
    print(f"wrote {OUT}/{name}.golden.json ({len(trades)} trades)")

def dump_optimize(name, stats, heatmap):
    """Dump optimize golden fixture: heatmap + best params + best value."""
    best_idx = heatmap.idxmax()  # (n1, n2) tuple
    best_value = heatmap.max()
    heatmap_dict = {}
    for idx, val in heatmap.items():
        n1, n2 = idx
        key = f"n1={n1},n2={n2}"
        heatmap_dict[key] = None if (isinstance(val, float) and math.isnan(val)) else val
    obj = {
        "heatmap": heatmap_dict,
        "best": {"n1": int(best_idx[0]), "n2": int(best_idx[1])},
        "best_value": float(best_value),
    }
    os.makedirs(OUT, exist_ok=True)
    with open(f"{OUT}/{name}.golden.json", "w") as f:
        json.dump(obj, f, indent=2)
    print(f"wrote {OUT}/{name}.golden.json ({len(heatmap_dict)} combos, best n1={best_idx[0]} n2={best_idx[1]} SQN={best_value:.6f})")

def main():
    df = pd.read_csv(DATA, parse_dates=["Date"], index_col="Date")
    for name, strat, nrows, kw in SCENARIOS:
        data = df.iloc[:nrows]
        bt = Backtest(data, strat, **kw)
        dump(name, bt.run())

    # Optimize parity fixture.
    data = df.iloc[:250]
    bt = Backtest(data, SmaCrossOpt, cash=10000, commission=0, finalize_trades=True)
    stats, heatmap = bt.optimize(
        n1=[5, 10, 15],
        n2=[20, 30, 40],
        maximize="SQN",
        method="grid",
        constraint=lambda p: p.n1 < p.n2,
        return_heatmap=True,
    )
    dump_optimize("optimize_aapl", stats, heatmap)

    return 0

if __name__ == "__main__":
    raise SystemExit(main())
