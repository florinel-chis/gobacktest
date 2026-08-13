// Command mlsignal backtests an externally-computed per-bar signal through the
// engine and reports it against buy-and-hold. It reads one CSV with OHLCV plus a
// signal column (a value > 0 opens a long, held for -hold bars) and runs
// strategies.SignalStrategy. The signal can come from anything computed offline:
// an indicator rule, an oversold screen, or a machine-learning forecast such as
// a candlestick foundation model. See docs/ml-signals.md for the full use case,
// including a Kronos dip-buy walk-through and gen_signals.py.
//
//	go run ./cmd/mlsignal -csv cmd/mlsignal/sample.csv
package main

import (
	"encoding/csv"
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	backtest "github.com/florinel-chis/gobacktest"
	"github.com/florinel-chis/gobacktest/strategies"
)

func main() {
	csvPath := flag.String("csv", "", "CSV with columns: timestamps,open,high,low,close,volume,<signal>")
	col := flag.String("col", "signal", "name of the signal column (>0 opens a long)")
	hold := flag.Int("hold", 6, "bars to hold each trade")
	size := flag.Float64("size", 0.95, "position size as a fraction of equity")
	spread := flag.Float64("spread", 0.0001, "relative spread charged per fill (0.0001 = 1bp)")
	cash := flag.Float64("cash", 10000, "starting cash")
	flag.Parse()
	if *csvPath == "" {
		fmt.Fprintln(os.Stderr, "usage: mlsignal -csv <file> [-col signal] [-hold 6] ...")
		os.Exit(2)
	}

	ts, o, h, l, c, v, sig, err := readCSV(*csvPath, *col)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read:", err)
		os.Exit(1)
	}
	data, err := backtest.FromOHLCV(ts, o, h, l, c, v)
	if err != nil {
		fmt.Fprintln(os.Stderr, "data:", err)
		os.Exit(1)
	}

	strat := &strategies.SignalStrategy{Signal: sig, Hold: *hold, Size: *size}
	bt := backtest.New(data, strat, backtest.Options{
		Cash: *cash, Spread: *spread, Margin: 1, FinalizeTrades: true,
	})
	res, err := bt.Run()
	if err != nil {
		fmt.Fprintln(os.Stderr, "run:", err)
		os.Exit(1)
	}
	s := backtest.Compute(res, data, 0.0)

	fmt.Printf("bars %d   signal %q   hold %d   spread %.1fbp/side\n",
		len(c), *col, *hold, *spread*1e4)
	fmt.Printf("  return          : %+.2f%%\n", s.ReturnPct)
	fmt.Printf("  buy & hold      : %+.2f%%\n", s.BuyHoldReturnPct)
	fmt.Printf("  edge vs B&H     : %+.2f%%\n", s.ReturnPct-s.BuyHoldReturnPct)
	fmt.Printf("  trades          : %d\n", s.NumTrades)
	fmt.Printf("  win rate        : %.0f%%\n", s.WinRatePct)
	fmt.Printf("  sharpe          : %.2f\n", s.SharpeRatio)
	fmt.Printf("  max drawdown    : %.1f%%\n", s.MaxDrawdownPct)
	fmt.Printf("  exposure        : %.0f%%\n", s.ExposureTimePct)
}

func readCSV(path, signal string) (ts []time.Time, o, h, l, c, v, sig []float64, err error) {
	f, err := os.Open(path) // #nosec G304 -- CLI-supplied path to a local research CSV
	if err != nil {
		return
	}
	defer f.Close()
	recs, err := csv.NewReader(f).ReadAll()
	if err != nil {
		return
	}
	if len(recs) < 2 {
		err = fmt.Errorf("csv has no data rows")
		return
	}
	col := map[string]int{}
	for i, name := range recs[0] {
		col[name] = i
	}
	need := []string{"timestamps", "open", "high", "low", "close", "volume", signal}
	for _, n := range need {
		if _, ok := col[n]; !ok {
			err = fmt.Errorf("missing column %q", n)
			return
		}
	}
	pf := func(s string) float64 { x, _ := strconv.ParseFloat(s, 64); return x }
	for _, r := range recs[1:] {
		t, e := time.Parse(time.RFC3339, r[col["timestamps"]])
		if e != nil {
			err = fmt.Errorf("bad timestamp %q: %w", r[col["timestamps"]], e)
			return
		}
		ts = append(ts, t)
		o = append(o, pf(r[col["open"]]))
		h = append(h, pf(r[col["high"]]))
		l = append(l, pf(r[col["low"]]))
		c = append(c, pf(r[col["close"]]))
		v = append(v, pf(r[col["volume"]]))
		sig = append(sig, pf(r[col[signal]]))
	}
	return
}
