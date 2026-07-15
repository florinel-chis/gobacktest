package backtest

import (
	"errors"
	"math"
	"math/rand"
	"runtime"
	"sort"
	"sync"
)

// Params is a single parameter combination: key → value.
type Params map[string]any

// gridCombos returns the full Cartesian product of the parameter space as a
// deterministic slice. Keys are iterated in sorted order; the first (alphabetically
// smallest) key varies slowest (outer loop), matching backtesting.py behaviour.
func gridCombos(space map[string][]any) []Params {
	// sort keys for determinism
	keys := make([]string, 0, len(space))
	for k := range space {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// build combos iteratively: start with one empty combo, extend for each key
	combos := []Params{{}}
	for _, k := range keys {
		vals := space[k]
		next := make([]Params, 0, len(combos)*len(vals))
		for _, existing := range combos {
			for _, v := range vals {
				p := make(Params, len(existing)+1)
				for ek, ev := range existing {
					p[ek] = ev
				}
				p[k] = v
				next = append(next, p)
			}
		}
		combos = next
	}
	return combos
}

// filterCombos returns only the combos for which constraint returns true.
func filterCombos(combos []Params, constraint func(Params) bool) []Params {
	out := combos[:0:0] // nil-safe empty slice sharing no backing with input
	for _, p := range combos {
		if constraint(p) {
			out = append(out, p)
		}
	}
	return out
}

// sampleCombos returns a random subset of up to maxTries combos, chosen without
// replacement. The result is deterministic for a given seed. If maxTries <= 0 or
// >= len(combos), the full slice is returned unchanged.
func sampleCombos(combos []Params, maxTries int, seed int64) []Params {
	if maxTries <= 0 || maxTries >= len(combos) {
		return combos
	}
	// Fisher-Yates partial shuffle on a copy of the indices
	rng := rand.New(rand.NewSource(seed)) // #nosec G404 -- deterministic seeded sampling for reproducible optimization, not security-sensitive
	indices := make([]int, len(combos))
	for i := range indices {
		indices[i] = i
	}
	for i := 0; i < maxTries; i++ {
		j := i + rng.Intn(len(indices)-i)
		indices[i], indices[j] = indices[j], indices[i]
	}
	out := make([]Params, maxTries)
	for i := range out {
		out[i] = combos[indices[i]]
	}
	return out
}

// ---------------------------------------------------------------------------
// Optimize: parallel, race-safe parameter optimizer
// ---------------------------------------------------------------------------

// OptimizeOptions configures how Optimize explores the parameter space.
type OptimizeOptions struct {
	// Maximize is the stat name to maximise (e.g. "SQN", "Return [%]").
	// Ignored when MaximizeFunc is set. Defaults to "SQN" when both are zero.
	Maximize string
	// MaximizeFunc is an optional custom metric extractor. When non-nil it takes
	// precedence over Maximize.
	MaximizeFunc func(Stats) float64
	// Constraint filters combos before evaluation; nil means accept all.
	// Workers MUST NOT mutate the Params they receive — treat them as read-only.
	Constraint func(Params) bool
	// MaxTries limits random sampling: 0 or >= len(combos) → full grid.
	MaxTries int
	// RandomSeed seeds the sampler (deterministic for a given seed).
	RandomSeed int64
	// Workers is the goroutine pool size; 0 → runtime.GOMAXPROCS(0).
	Workers int
	// ReturnHeatmap populates OptimizeResult.Heatmap for every evaluated combo.
	ReturnHeatmap bool
	// RiskFreeRate is the annualised risk-free rate forwarded to Compute.
	RiskFreeRate float64
}

// HeatmapEntry holds one combo's parameters, metric value, and full stats.
// Params is read-only. Stats is the zero Stats when Value == -math.MaxFloat64 (combo ran out of money).
type HeatmapEntry struct {
	Params Params
	Value  float64
	Stats  Stats
}

// OptimizeResult is the output of Optimize.
type OptimizeResult struct {
	// Best is the parameter combination that achieved BestValue (first in
	// enumeration order when multiple combos tie). Nil only when no combo ran.
	// Best is read-only; it aliases a heatmap entry's Params.
	Best      Params
	BestStats Stats
	BestValue float64
	// Heatmap is populated (in enumeration order) when OptimizeOptions.ReturnHeatmap
	// is true.
	Heatmap []HeatmapEntry
}

// Optimize runs a backtest for every parameter combination in space, using a
// bounded worker pool for parallelism, and returns the best result together
// with an optional heatmap.
//
// The build factory is called concurrently (one fresh Strategy per combo) and must
// not capture or mutate shared state. The Params passed to build are read-only.
//
// Race safety: each worker operates on its own data.clone() and writes results
// into a pre-allocated slice at its assigned index, protected by a mutex.
// Deterministic best: results are scanned in enumeration order after all workers
// finish, so the outcome is reproducible regardless of goroutine scheduling.
//
// Returns an error if space is empty or all combos are filtered out; propagates
// any non-ErrOutOfMoney run error. ErrOutOfMoney is treated as the worst possible
// metric (-math.MaxFloat64) rather than a fatal error.
func Optimize(
	data *Data,
	opts Options,
	build func(Params) Strategy,
	space map[string][]any,
	oo OptimizeOptions,
) (*OptimizeResult, error) {
	if len(space) == 0 {
		return nil, errors.New("backtest: Optimize needs a non-empty parameter space")
	}

	// 1. Enumerate → filter → sample (all deterministic).
	combos := gridCombos(space)
	if oo.Constraint != nil {
		combos = filterCombos(combos, oo.Constraint)
	}
	if len(combos) == 0 {
		return nil, errors.New("backtest: Optimize: all parameter combinations were rejected by Constraint")
	}
	combos = sampleCombos(combos, oo.MaxTries, oo.RandomSeed)

	// 2. Resolve defaults.
	workers := oo.Workers
	if workers <= 0 {
		workers = runtime.GOMAXPROCS(0)
	}
	if workers > len(combos) {
		workers = len(combos)
	}
	maximize := oo.Maximize
	if maximize == "" && oo.MaximizeFunc == nil {
		maximize = "SQN"
	}

	// 3. Pre-allocate result slots (indexed by combo position for determinism).
	type slot struct {
		value float64
		stats Stats
		ran   bool // false means this combo encountered a non-ErrOutOfMoney error
	}
	results := make([]slot, len(combos))

	// 4. Fan work out over the channel; workers consume it concurrently.
	type workItem struct {
		idx   int
		combo Params // read-only — workers MUST NOT mutate
	}
	ch := make(chan workItem, len(combos))
	for i, c := range combos {
		ch <- workItem{idx: i, combo: c}
	}
	close(ch)

	var mu sync.Mutex
	var firstErr error
	var wg sync.WaitGroup

	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for item := range ch {
				// Each worker owns its own Data clone — the only mutable per-run state.
				dc := data.clone()
				strat := build(item.combo)
				bt := New(dc, strat, opts)
				res, err := bt.Run()

				var metric float64
				var st Stats

				if err != nil {
					if errors.Is(err, ErrOutOfMoney) {
						// Treat out-of-money as the worst result, not a fatal error.
						metric = -math.MaxFloat64
					} else {
						mu.Lock()
						if firstErr == nil {
							firstErr = err
						}
						mu.Unlock()
						continue // leave results[item.idx].ran = false
					}
				} else {
					st = Compute(res, dc, oo.RiskFreeRate)
					if oo.MaximizeFunc != nil {
						metric = oo.MaximizeFunc(st)
					} else {
						var ok bool
						metric, ok = statValue(st, maximize)
						if !ok {
							metric = -math.MaxFloat64
						}
					}
					if math.IsNaN(metric) || math.IsInf(metric, 0) {
						metric = -math.MaxFloat64
					}
				}

				mu.Lock()
				results[item.idx] = slot{value: metric, stats: st, ran: true}
				mu.Unlock()
			}
		}()
	}
	wg.Wait()

	if firstErr != nil {
		return nil, firstErr
	}

	// 5. Scan in enumeration order → first combo achieving the maximum wins.
	// This guarantees a deterministic best regardless of goroutine completion order.
	bestIdx := -1
	bestVal := math.Inf(-1)
	for i, r := range results {
		if !r.ran {
			continue
		}
		if bestIdx == -1 || r.value > bestVal {
			bestVal = r.value
			bestIdx = i
		}
	}

	out := &OptimizeResult{}
	if bestIdx >= 0 {
		out.Best = combos[bestIdx]
		out.BestStats = results[bestIdx].stats
		out.BestValue = bestVal
	}

	if oo.ReturnHeatmap {
		out.Heatmap = make([]HeatmapEntry, len(combos))
		for i, r := range results {
			out.Heatmap[i] = HeatmapEntry{
				Params: combos[i],
				Value:  r.value,
				Stats:  r.stats,
			}
		}
	}

	return out, nil
}
