#!/usr/bin/env python3
"""Run the Williams %R oversold strategy in backtesting.py for parity comparison.

Reads the CSVs produced by `go run ./cmd/wrparity <dir>` (OHLCV + the go-talib
WilliamsR21/EMA12 columns) so BOTH engines consume identical bars AND identical
indicator arrays. This isolates engine + statistics parity from indicator
computation. Writes py_stats.json keyed the same way as go_stats.json.

Usage: python wr_parity.py <dir>
"""
import json, math, sys, os
import pandas as pd
from backtesting import Backtest, Strategy


class WROversold(Strategy):
    def init(self):
        # Feed the precomputed (shared) indicator columns; self.I registers them
        # so backtesting.py honours their NaN warmup exactly like the Go engine.
        self.wr = self.I(lambda: self.data.WilliamsR21, name="WilliamsR(21)")
        self.ema = self.I(lambda: self.data.EMA12, name="EMA(12)")

    def next(self):
        if not self.position and self.wr[-1] <= -80 and self.ema[-1] <= -80:
            self.buy(size=1)
        # Arm a +10% take-profit off the real entry price, once open.
        for t in self.trades:
            if not t.tp:
                t.tp = t.entry_price * 1.10


# backtesting.py stat name -> our shared key.
KEYMAP = {
    "Return [%]": "ReturnPct",
    "Buy & Hold Return [%]": "BuyHoldReturnPct",
    "# Trades": "NumTrades",
    "Win Rate [%]": "WinRatePct",
    "Max. Drawdown [%]": "MaxDrawdownPct",
    "Sharpe Ratio": "SharpeRatio",
    "Sortino Ratio": "SortinoRatio",
    "Calmar Ratio": "CalmarRatio",
    "Exposure Time [%]": "ExposureTimePct",
    "Equity Final [$]": "EquityFinal",
    "Best Trade [%]": "BestTradePct",
    "Worst Trade [%]": "WorstTradePct",
    "Avg. Trade [%]": "AvgTradePct",
    "Profit Factor": "ProfitFactor",
    "Expectancy [%]": "ExpectancyPct",
    "SQN": "SQN",
}


def scalar(v):
    if isinstance(v, float) and (math.isnan(v) or math.isinf(v)):
        return None
    if hasattr(v, "item"):
        v = v.item()
    return v


def main():
    d = sys.argv[1]
    symbols = ["AAPL", "MSFT", "NVDA", "AMZN", "SPY"]
    out = {}
    for sym in symbols:
        path = os.path.join(d, f"{sym}.csv")
        if not os.path.exists(path):
            continue
        df = pd.read_csv(path, parse_dates=["Date"], index_col="Date")
        bt = Backtest(df, WROversold, cash=10000, commission=0.0,
                      margin=1.0, finalize_trades=True)
        stats = bt.run()
        out[sym] = {k: scalar(stats[name]) for name, k in KEYMAP.items()}
        print(f"{sym}: Return {stats['Return [%]']:.2f}%  "
              f"Trades {int(stats['# Trades'])}  Sharpe {stats['Sharpe Ratio']:.2f}")

    with open(os.path.join(d, "py_stats.json"), "w") as f:
        json.dump(out, f, indent=2)
    print(f"wrote {os.path.join(d, 'py_stats.json')} ({len(out)} symbols)")


if __name__ == "__main__":
    raise SystemExit(main())
