// Command genheatmap optimizes an SMA-crossover over an n1×n2 grid on the bundled
// AAPL data and writes a self-contained HTML heatmap — a demo of report.Heatmap.
package main

import (
	"flag"
	"fmt"

	backtest "github.com/florinel-chis/gobacktest"
	"github.com/florinel-chis/gobacktest/indicators"
	"github.com/florinel-chis/gobacktest/lib"
	"github.com/florinel-chis/gobacktest/report"
)

type sma struct {
	n1, n2     int
	fast, slow *backtest.Indicator
}

func (s *sma) Init(st *backtest.State) {
	c := st.Data().Close()
	s.fast = st.I("fast", func() []float64 { return indicators.SMA(c, s.n1) })
	s.slow = st.I("slow", func() []float64 { return indicators.SMA(c, s.n2) })
}

func (s *sma) Next(st *backtest.State) {
	switch {
	case lib.Crossover(s.fast.Series(), s.slow.Series()) && st.Position().Size() == 0:
		st.Buy(backtest.Order{Size: 10})
	case lib.CrossUnder(s.fast.Series(), s.slow.Series()) && st.Position().IsLong():
		st.Position().Close()
	}
}

func main() {
	out := flag.String("out", "/tmp/gobacktest-heatmap.html", "output HTML path")
	flag.Parse()

	data, err := backtest.FromCSV("testdata/AAPL_1d.csv")
	if err != nil {
		panic(err)
	}
	build := func(p backtest.Params) backtest.Strategy {
		return &sma{n1: p["n1"].(int), n2: p["n2"].(int)}
	}
	opt, err := backtest.Optimize(
		data,
		backtest.Options{Cash: 10000, Margin: 1, FinalizeTrades: true},
		build,
		map[string][]any{"n1": {5, 10, 15, 20}, "n2": {30, 40, 50, 60}},
		backtest.OptimizeOptions{Maximize: "SQN", ReturnHeatmap: true,
			Constraint: func(p backtest.Params) bool { return p["n1"].(int) < p["n2"].(int) }},
	)
	if err != nil {
		panic(err)
	}
	if err := report.Heatmap(opt, *out, report.Title("SMA n1×n2 optimization — SQN"), report.Metric("SQN")); err != nil {
		panic(err)
	}
	fmt.Printf("wrote %s  (best n1=%v n2=%v, SQN=%.3f)\n", *out, opt.Best["n1"], opt.Best["n2"], opt.BestValue)
}
