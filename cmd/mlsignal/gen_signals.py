#!/usr/bin/env python3
"""Generate a point-in-time dip-buy signal from the Kronos candlestick model.

This is the reference producer for `cmd/mlsignal`. It reads an OHLCV CSV and
writes the same rows plus a `signal` column: for each bar that closed DOWN, it
asks Kronos — using only bars up to and including that bar — to forecast the next
few bars, and sets signal=1 when the model expects a higher close (i.e. it expects
the dip to recover). Everything else is 0.

The forecast at bar i uses only data <= i, so the signal has no lookahead: paired
with the engine's next-bar fills, an entry decided at the close of bar i is taken
at bar i+1. `cmd/mlsignal` then backtests it with costs against buy-and-hold.

Setup (Kronos is a separate project, not a dependency of this module):
    git clone https://github.com/shiyu-coder/Kronos && cd Kronos
    pip install -r requirements.txt
    # then run this script from inside the Kronos checkout, or add it to sys.path

Usage:
    python gen_signals.py INPUT_OHLCV.csv OUTPUT_WITH_SIGNAL.csv

INPUT columns: timestamps,open,high,low,close,volume  (timestamps ISO-8601).
"""
import sys
import numpy as np
import pandas as pd
from model import Kronos, KronosTokenizer, KronosPredictor  # from the Kronos repo

H = 6        # forecast / hold horizon (bars)
CTX = 500    # max context bars
SAMPLES = 10 # sampled futures to average
WINDOW = 250 # most-recent bars to generate signals for


def main(inp, outp):
    df = pd.read_csv(inp)
    df["timestamps"] = pd.to_datetime(df["timestamps"])
    n = len(df)
    start = max(1, n - WINDOW)

    tok = KronosTokenizer.from_pretrained("NeoQuasar/Kronos-Tokenizer-base")
    model = Kronos.from_pretrained("NeoQuasar/Kronos-small")
    pred = KronosPredictor(model, tok, max_context=CTX)  # auto-selects CUDA/MPS/CPU

    sig = np.zeros(n, dtype=int)
    for t in range(start, n):
        if df["close"].iloc[t] >= df["close"].iloc[t - 1]:
            continue  # only act on down candles
        hor = min(H, n - 1 - t)
        if hor < 1:
            continue
        cs = max(0, t - CTX + 1)
        x_df = df.iloc[cs:t + 1][["open", "high", "low", "close", "volume"]].reset_index(drop=True)
        x_ts = df.iloc[cs:t + 1]["timestamps"].reset_index(drop=True)
        y_ts = df.iloc[t + 1:t + 1 + hor]["timestamps"].reset_index(drop=True)
        fc = pred.predict(df=x_df, x_timestamp=x_ts, y_timestamp=y_ts, pred_len=hor,
                          T=1.0, top_p=0.9, sample_count=SAMPLES, verbose=False)
        if float(fc["close"].iloc[-1]) > float(df["close"].iloc[t]):
            sig[t] = 1

    out = df.copy()
    out["timestamps"] = out["timestamps"].map(lambda d: d.isoformat())
    out["signal"] = sig
    out.to_csv(outp, index=False)
    print(f"wrote {outp}: {int(sig.sum())} signals over {n - start} candidate bars")


if __name__ == "__main__":
    if len(sys.argv) != 3:
        sys.exit("usage: python gen_signals.py INPUT_OHLCV.csv OUTPUT_WITH_SIGNAL.csv")
    main(sys.argv[1], sys.argv[2])
