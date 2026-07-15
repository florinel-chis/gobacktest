// Command genreport runs an SMA-crossover backtest on the bundled AAPL data and
// writes a self-contained HTML report — a demo of the report package.
package main

import (
	"flag"
	"fmt"

	backtest "github.com/florinel-chis/gobacktest"
	"github.com/florinel-chis/gobacktest/indicators"
	"github.com/florinel-chis/gobacktest/lib"
	"github.com/florinel-chis/gobacktest/report"
)

type smaCross struct{ fast, slow *backtest.Indicator }

func (s *smaCross) Init(st *backtest.State) {
	c := st.Data().Close()
	s.fast = st.I("SMA10", func() []float64 { return indicators.SMA(c, 10) }, backtest.Overlay(), backtest.Color("#2962FF"))
	s.slow = st.I("SMA20", func() []float64 { return indicators.SMA(c, 20) }, backtest.Overlay(), backtest.Color("#ff6d00"))
	// A non-overlay oscillator to demo indicator subpanes.
	st.I("RSI14", func() []float64 { return indicators.RSI(c, 14) }, backtest.Color("#e91e63"))
}

func (s *smaCross) Next(st *backtest.State) {
	switch {
	case lib.Crossover(s.fast.Series(), s.slow.Series()) && st.Position().Size() == 0:
		st.Buy(backtest.Order{Size: 10})
	case lib.CrossUnder(s.fast.Series(), s.slow.Series()) && st.Position().IsLong():
		st.Position().Close()
	}
}

func main() {
	out := flag.String("out", "/tmp/gobacktest-report.html", "output HTML path")
	flag.Parse()

	data, err := backtest.FromCSV("testdata/AAPL_1d.csv")
	if err != nil {
		panic(err)
	}
	bt := backtest.New(data, &smaCross{}, backtest.Options{Cash: 10000, Margin: 1, FinalizeTrades: true})
	res, err := bt.Run()
	if err != nil {
		panic(err)
	}
	stats := backtest.Compute(res, data, 0)

	if err := report.Generate(data, res, stats, *out, report.Title("AAPL — SMA(10/20) Crossover")); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %s  (%d trades, return %.2f%%)\n", *out, stats.NumTrades, stats.ReturnPct)
}
