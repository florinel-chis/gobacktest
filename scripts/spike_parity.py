"""
Parity spike: run SMA-crossover strategy in backtesting.py on a 250-bar AAPL slice,
dump trades + equity to a work directory as py_spike.json for comparison with the
Go engine.

Usage:
    python spike_parity.py [csv_path] [out_path]

Defaults: reads AAPL_250.csv and writes py_spike.json in SPIKE_WORKDIR
(environment variable, default: current directory).
"""

import json
import math
import os
import sys
import pandas as pd
from backtesting import Backtest, Strategy
from backtesting.lib import crossover

WORKDIR = os.environ.get("SPIKE_WORKDIR", ".")
CSV_PATH = sys.argv[1] if len(sys.argv) > 1 else f"{WORKDIR}/AAPL_250.csv"
OUT_PATH = sys.argv[2] if len(sys.argv) > 2 else f"{WORKDIR}/py_spike.json"

FAST_PERIOD = 10
SLOW_PERIOD = 20
TRADE_SIZE = 10
CASH = 10_000.0


def SMA(arr, n):
    return pd.Series(arr).rolling(n).mean()


class SmaCross(Strategy):
    fast_period = FAST_PERIOD
    slow_period = SLOW_PERIOD

    def init(self):
        self.fast = self.I(SMA, self.data.Close, self.fast_period)
        self.slow = self.I(SMA, self.data.Close, self.slow_period)

    def next(self):
        if crossover(self.fast, self.slow):
            if not self.position:
                self.buy(size=TRADE_SIZE)
        elif crossover(self.slow, self.fast):
            if self.position.is_long:
                self.position.close()


# Load data
data = pd.read_csv(CSV_PATH, index_col=0, parse_dates=True)
data.index.name = "Date"

print(f"Data shape: {data.shape}")
print(f"Date range: {data.index[0]} .. {data.index[-1]}")

# Run backtest
bt = Backtest(
    data,
    SmaCross,
    cash=CASH,
    commission=0.0,
    margin=1.0,
    trade_on_close=False,
    hedging=False,
    exclusive_orders=False,
    finalize_trades=True,
)
stats = bt.run()

print("\n=== Stats ===")
print(f"# Trades:         {stats['# Trades']}")
print(f"Return [%]:       {stats['Return [%]']:.4f}")
print(f"Equity Final [$]: {stats['Equity Final [$]']:.4f}")

# Extract trades
trades_df = stats["_trades"]
print("\n=== Trades (raw columns) ===")
print(trades_df.columns.tolist())
print(trades_df[["Size", "EntryBar", "ExitBar", "EntryPrice", "ExitPrice", "PnL"]].to_string())

# Extract equity curve
eq_curve = stats["_equity_curve"]
print(f"\nEquity curve length: {len(eq_curve)}")
print(f"Equity curve first 25 values:\n{eq_curve['Equity'][:25].to_string()}")

# Serialize
trades_records = []
for _, row in trades_df.iterrows():
    trades_records.append({
        "Size": float(row["Size"]),
        "EntryBar": int(row["EntryBar"]),
        "ExitBar": int(row["ExitBar"]),
        "EntryPrice": float(row["EntryPrice"]),
        "ExitPrice": float(row["ExitPrice"]),
        "PnL": float(row["PnL"]),
    })

equity_list = [float(v) for v in eq_curve["Equity"]]

output = {
    "num_trades": int(stats["# Trades"]),
    "return_pct": float(stats["Return [%]"]),
    "equity_final": float(stats["Equity Final [$]"]),
    "trades": trades_records,
    "equity": equity_list,
}

with open(OUT_PATH, "w") as f:
    json.dump(output, f, indent=2)

print(f"\nWrote {OUT_PATH}")
