# Backtesting an external model signal

A price-prediction model — a moving-average rule, an oversold screen, or a
machine-learning forecast such as a candlestick foundation model — only tells you
*what it thinks*. Whether that is worth trading is a separate question, and the
only honest way to answer it is a backtest with costs, benchmarked against buying
and holding. This engine makes that a three-step loop:

1. Compute one number per bar offline and write it to a CSV column.
2. Backtest it with [`strategies.SignalStrategy`](../strategies/signal.go).
3. Read the stats against buy-and-hold.

The point of separating the two halves is that the engine stays model-agnostic.
Any predictor that can emit a per-bar signal plugs in unchanged.

## The strategy

`SignalStrategy` goes long when `Signal[bar] > 0` and exits `Hold` bars later at
market. It holds one position at a time. That is the whole rule — the intelligence
lives in how you build the `Signal` slice.

```go
strat := &strategies.SignalStrategy{
    Signal: signal,  // one value per bar; > 0 opens a long
    Hold:   6,       // exit six bars later
    Size:   0.95,    // 95% of equity per trade
}
bt := backtest.New(data, strat, backtest.Options{
    Cash: 10000, Spread: 0.0001, Margin: 1, FinalizeTrades: true,
})
res, _ := bt.Run()
stats := backtest.Compute(res, data, 0.0)
fmt.Printf("return %.2f%%  buy&hold %.2f%%\n", stats.ReturnPct, stats.BuyHoldReturnPct)
```

### No lookahead

`Signal[i]` must be computable from information available at the **close of bar i**
only — never from later bars. Because an order enqueued during bar `i` fills on
bar `i+1`, a correctly built signal is acted on one bar after it is known, with no
peeking ahead. Getting this wrong is the most common way a model backtest turns
into fiction, so build the signal producer to respect it.

## Run the example

A ready-made command reads a CSV (`timestamps,open,high,low,close,volume,signal`)
and prints the result against buy-and-hold:

```
go run ./cmd/mlsignal -csv cmd/mlsignal/sample.csv
```

`sample.csv` is a small synthetic series included so the command runs out of the
box; its signal is illustrative, not meaningful. Point `-csv` at your own file to
test a real signal. Flags: `-col` (signal column name), `-hold`, `-size`,
`-spread` (relative, `0.0001` = 1bp per fill), `-cash`.

## Case study: a candlestick foundation model

The motivating example is [Kronos](https://github.com/shiyu-coder/Kronos), an open
model that forecasts candlesticks. A natural strategy is to buy a down candle the
model expects to recover: on each down bar, forecast the next day and enter long
if the predicted close is higher. [`cmd/mlsignal/gen_signals.py`](../cmd/mlsignal/gen_signals.py)
produces exactly this signal, point-in-time, from an OHLCV CSV.

Tested this way over an out-of-sample window across ten FX and CFD markets, the
model-filtered version did not beat the alternatives. Running the identical rule
with and without the model isolates its contribution:

| Strategy | Mean return | vs buy-and-hold | Win rate | Trades |
|---|---|---|---|---|
| Dip-buy, model-filtered | +0.38% | −0.67% | 61% | 167 |
| Dip-buy, no model | +0.69% | −0.37% | 55% | 313 |
| Buy and hold | +1.06% | — | — | — |

The model filter cut trade count roughly in half and lifted the win rate, but not
the return: it removed winners along with losers, so a higher hit rate bought
nothing. This is the value of running it as a strategy rather than scoring
direction accuracy — the comparison against a trivial baseline and against holding
is where an apparent signal usually disappears.

Reproduce it end to end:

```
# 1. build a point-in-time signal from your OHLCV (needs the Kronos checkout)
python cmd/mlsignal/gen_signals.py my_ohlcv.csv my_signal.csv

# 2. backtest it with costs, against buy-and-hold
go run ./cmd/mlsignal -csv my_signal.csv -hold 6 -spread 0.0001
```

## Caveats

A single backtest is one regime, one horizon, and one exit rule. Use several
windows and markets, keep the cost assumption realistic, and always read the
result next to buy-and-hold and a no-model baseline before believing a signal has
an edge.
