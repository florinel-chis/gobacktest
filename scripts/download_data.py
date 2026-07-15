#!/usr/bin/env python3
"""Download canonical OHLCV snapshots for gobacktest test fixtures.

Uses yfinance (auto_adjust=True) over a FROZEN date range so the committed
CSVs are reproducible. The same files feed both the Python golden run and the
Go tests (identical-inputs invariant, spec section 8.3-8.4).

Usage: python scripts/download_data.py [--out testdata]
Requires a current yfinance (>=1.x); old versions fail with "No price data".
"""
import argparse
import os
import sys
import yfinance as yf

SYMBOLS = ["AAPL", "TSLA", "SPY", "MSFT"]
START = "2018-01-01"
END = "2023-12-31"  # frozen, past dates -> stable snapshot


def main() -> int:
    ap = argparse.ArgumentParser()
    ap.add_argument("--out", default="testdata")
    args = ap.parse_args()
    os.makedirs(args.out, exist_ok=True)

    ver = tuple(int(x) for x in yf.__version__.split(".")[:2])
    if ver < (1, 0):
        print(f"yfinance {yf.__version__} is too old; need >=1.0", file=sys.stderr)
        return 1

    for sym in SYMBOLS:
        df = yf.download(sym, start=START, end=END, interval="1d",
                         auto_adjust=True, progress=False)
        if df.empty:
            print(f"ERROR: no data for {sym}", file=sys.stderr)
            return 1
        df = df[["Open", "High", "Low", "Close", "Volume"]].dropna()
        # Flatten any MultiIndex columns yfinance may produce for a single symbol.
        df.columns = ["Open", "High", "Low", "Close", "Volume"]
        path = f"{args.out}/{sym}_1d.csv"
        df.to_csv(path, index_label="Date")
        print(f"wrote {path} ({len(df)} rows)")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
