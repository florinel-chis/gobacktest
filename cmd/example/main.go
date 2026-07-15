// Command example is the README quickstart as a runnable program.
// It runs an SMA(10/20) crossover on AAPL, prints stats, writes an HTML
// report, then demonstrates Optimize over a small parameter grid.
//
// Usage (from module root):
//
//	go run ./cmd/example
package main

import (
	"fmt"

	backtest "github.com/florinel-chis/gobacktest"
	"github.com/florinel-chis/gobacktest/indicators"
	"github.com/florinel-chis/gobacktest/lib"
	"github.com/florinel-chis/gobacktest/report"
)

// smaCross is the example strategy: buy on SMA fast/slow crossover, exit on cross-under.
type smaCross struct {
	n1, n2     int
	fast, slow *backtest.Indicator
}

func (s *smaCross) Init(st *backtest.State) {
	c := st.Data().Close()
	s.fast = st.I(fmt.Sprintf("SMA%d", s.n1), func() []float64 { return indicators.SMA(c, s.n1) },
		backtest.Overlay(), backtest.Color("#2962FF"))
	s.slow = st.I(fmt.Sprintf("SMA%d", s.n2), func() []float64 { return indicators.SMA(c, s.n2) },
		backtest.Overlay(), backtest.Color("#ff6d00"))
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
	data, err := backtest.FromCSV("testdata/AAPL_1d.csv")
	if err != nil {
		panic(err)
	}

	// --- Single run: SMA(10/20) crossover ---
	bt := backtest.New(data, &smaCross{n1: 10, n2: 20}, backtest.Options{Cash: 10000, Margin: 1, FinalizeTrades: true})
	res, err := bt.Run()
	if err != nil {
		panic(err)
	}
	stats := backtest.Compute(res, data, 0)

	fmt.Println("=== AAPL SMA(10/20) Crossover ===")
	fmt.Print(stats.String())
	fmt.Println()

	out := "/tmp/gobacktest-example.html"
	if err := report.Generate(data, res, stats, out, report.Title("AAPL SMA Crossover")); err != nil {
		panic(err)
	}
	fmt.Printf("report written to %s\n\n", out)

	// --- Optimize: grid-search n1 ∈ {5,10,15}, n2 ∈ {20,30,40}, constraint n1<n2 ---
	fmt.Println("=== Optimize SMA periods (maximize SQN) ===")
	opt, err := backtest.Optimize(
		data,
		backtest.Options{Cash: 10000, Margin: 1, FinalizeTrades: true},
		func(p backtest.Params) backtest.Strategy {
			n1 := p["n1"].(int)
			n2 := p["n2"].(int)
			return &smaCross{n1: n1, n2: n2}
		},
		map[string][]any{
			"n1": {5, 10, 15},
			"n2": {20, 30, 40},
		},
		backtest.OptimizeOptions{
			Maximize:   "SQN",
			Constraint: func(p backtest.Params) bool { return p["n1"].(int) < p["n2"].(int) },
		},
	)
	if err != nil {
		panic(err)
	}

	fmt.Printf("Best params: n1=%v n2=%v\n", opt.Best["n1"], opt.Best["n2"])
	fmt.Printf("Best SQN:    %.3f\n", opt.BestValue)
}
