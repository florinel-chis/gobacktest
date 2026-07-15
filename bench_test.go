package backtest

import (
	"math"
	"testing"
)

// benchStrat is a minimal SMA-cross used by the benchmarks.
type benchStrat struct{ fast, slow *Indicator }

func (s *benchStrat) Init(st *State) {
	s.fast = st.I("f", func() []float64 { return benchSMA(st.Data().Close(), 10) })
	s.slow = st.I("s", func() []float64 { return benchSMA(st.Data().Close(), 20) })
}
func (s *benchStrat) Next(st *State) {
	if s.fast.Len() < 2 {
		return
	}
	if s.fast.At(1) < s.slow.At(1) && s.fast.Last() > s.slow.Last() && st.Position().Size() == 0 {
		st.Buy(Order{Size: 10})
	} else if s.fast.At(1) > s.slow.At(1) && s.fast.Last() < s.slow.Last() && st.Position().IsLong() {
		st.Position().Close()
	}
}

func benchSMA(s Series, n int) []float64 {
	out := make([]float64, len(s))
	for i := range s {
		if i < n-1 {
			out[i] = math.NaN()
			continue
		}
		sum := 0.0
		for j := i - n + 1; j <= i; j++ {
			sum += s[j]
		}
		out[i] = sum / float64(n)
	}
	return out
}

func benchData(tb testing.TB) *Data {
	d, err := FromCSV("testdata/AAPL_1d.csv") // ~1509 daily bars
	if err != nil {
		tb.Fatal(err)
	}
	return d
}

func BenchmarkRun(b *testing.B) {
	d := benchData(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		bt := New(d.clone(), &benchStrat{}, Options{Cash: 10000, Margin: 1, FinalizeTrades: true})
		if _, err := bt.Run(); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkCompute(b *testing.B) {
	d := benchData(b)
	bt := New(d.clone(), &benchStrat{}, Options{Cash: 10000, Margin: 1, FinalizeTrades: true})
	res, err := bt.Run()
	if err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = Compute(res, d, 0)
	}
}

func BenchmarkOptimize(b *testing.B) {
	d := benchData(b)
	build := func(p Params) Strategy { return &benchStrat{} } // params unused; measures the pipeline
	space := map[string][]any{"n1": {5, 10, 15}, "n2": {20, 30, 40}}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Optimize(d, Options{Cash: 10000, Margin: 1, FinalizeTrades: true}, build, space,
			OptimizeOptions{Maximize: "SQN"}); err != nil {
			b.Fatal(err)
		}
	}
}
