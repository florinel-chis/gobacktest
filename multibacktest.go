package backtest

import (
	"math"
	"runtime"
	"sync"
)

// MultiBacktest runs a single strategy across many datasets in parallel — e.g. a
// portfolio of symbols or walk-forward folds. Mirrors backtesting.py's
// lib.MultiBacktest. build must return a FRESH Strategy on each call (each dataset
// gets its own strategy instance, like Optimize) since a Strategy holds per-run
// indicator state.
type MultiBacktest struct {
	datasets []*Data
	build    func() Strategy
	opts     Options
	workers  int // 0 → runtime.GOMAXPROCS(0)
}

// NewMultiBacktest constructs a MultiBacktest over the given datasets. datasets
// and build are used as-is; build is called once per dataset (concurrently) to
// obtain a fresh Strategy instance.
func NewMultiBacktest(datasets []*Data, build func() Strategy, opts Options) *MultiBacktest {
	return &MultiBacktest{datasets: datasets, build: build, opts: opts}
}

// Workers sets the goroutine pool size (0 → runtime.GOMAXPROCS(0)). Returns the
// receiver for chaining.
func (m *MultiBacktest) Workers(n int) *MultiBacktest {
	m.workers = n
	return m
}

// Run backtests every dataset concurrently and returns one *Result per dataset in
// the SAME ORDER as the input datasets slice. If any dataset errors, Run returns
// the first error (by ascending dataset index, deterministic regardless of
// goroutine completion order) and a nil slice.
//
// An empty datasets slice returns (nil slice, nil error) — there is nothing to
// run and nothing to report as an error.
//
// Race safety: each worker calls build() to obtain its own Strategy and runs
// New(datasets[i], strat, opts).Run() — datasets are read-only during a run (no
// clone is needed, unlike Optimize, because each dataset is used exactly once).
// Results are written into a pre-allocated slice at the worker's assigned index.
func (m *MultiBacktest) Run() ([]*Result, error) {
	if len(m.datasets) == 0 {
		return nil, nil
	}

	workers := m.workers
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers > len(m.datasets) {
		workers = len(m.datasets)
	}

	results := make([]*Result, len(m.datasets))
	errs := make([]error, len(m.datasets))

	ch := make(chan int, len(m.datasets))
	for i := range m.datasets {
		ch <- i
	}
	close(ch)

	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := range ch {
				strat := m.build()
				bt := New(m.datasets[i], strat, m.opts)
				res, err := bt.Run()
				results[i] = res
				errs[i] = err
			}
		}()
	}
	wg.Wait()

	for _, err := range errs {
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

// Stats runs every dataset and computes per-dataset Stats (same order as the
// input datasets slice).
func (m *MultiBacktest) Stats(riskFreeRate float64) ([]Stats, error) {
	results, err := m.Run()
	if err != nil {
		return nil, err
	}
	stats := make([]Stats, len(results))
	for i, res := range results {
		stats[i] = Compute(res, m.datasets[i], riskFreeRate)
	}
	return stats, nil
}

// MeanStat runs every dataset (via Stats) and returns the mean of one metric
// across all datasets, skipping NaN values. sel extracts the metric from a
// Stats value, e.g. func(s Stats) float64 { return s.ReturnPct }. Returns NaN
// if there are no datasets or every selected value is NaN.
func (m *MultiBacktest) MeanStat(riskFreeRate float64, sel func(Stats) float64) (float64, error) {
	stats, err := m.Stats(riskFreeRate)
	if err != nil {
		return math.NaN(), err
	}
	sum := 0.0
	n := 0
	for _, s := range stats {
		v := sel(s)
		if math.IsNaN(v) {
			continue
		}
		sum += v
		n++
	}
	if n == 0 {
		return math.NaN(), nil
	}
	return sum / float64(n), nil
}
