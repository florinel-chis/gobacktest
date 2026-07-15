package backtest

import (
	"sync"
	"testing"
)

// ---------------------------------------------------------------------------
// Task 4: Optimize
// ---------------------------------------------------------------------------

// paramSma is a parameterised SMA crossover strategy used by TestOptimizeFindsBest.
// It uses the in-package localSMA helper (engine_smacross_test.go, no build tag).
type paramSma struct {
	n1, n2     int
	fast, slow *Indicator
}

func (st *paramSma) Init(s *State) {
	st.fast = s.I("fast", func() []float64 { return localSMA(s.Data().Close(), st.n1) })
	st.slow = s.I("slow", func() []float64 { return localSMA(s.Data().Close(), st.n2) })
}

func (st *paramSma) Next(s *State) {
	if st.fast.Len() < 2 {
		return
	}
	crossUp := st.fast.At(1) <= st.slow.At(1) && st.fast.Last() > st.slow.Last()
	crossDn := st.fast.At(1) >= st.slow.At(1) && st.fast.Last() < st.slow.Last()
	if crossUp && s.Position().Size() == 0 {
		s.Buy(Order{Size: 10})
	} else if crossDn && s.Position().IsLong() {
		s.Position().Close()
	}
}

func TestOptimizeFindsBest(t *testing.T) {
	// Monotonic rally; each combo runs independently via per-worker data.clone().
	d := ramp(120)
	space := map[string][]any{"n1": {5, 10}, "n2": {20, 40}}
	build := func(p Params) Strategy {
		return &paramSma{n1: p["n1"].(int), n2: p["n2"].(int)}
	}
	res, err := Optimize(d, Options{Cash: 10000, Margin: 1, FinalizeTrades: true}, build, space,
		OptimizeOptions{
			Maximize:      "Return [%]",
			ReturnHeatmap: true,
			Constraint:    func(p Params) bool { return p["n1"].(int) < p["n2"].(int) },
		})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Heatmap) != 4 { // space has 4 combos; constraint n1<n2 passes all of them
		t.Fatalf("heatmap=%d want 4", len(res.Heatmap))
	}
	if res.Best == nil {
		t.Fatal("no best found")
	}
	// best value must be the max over the heatmap
	max := res.Heatmap[0].Value
	for _, h := range res.Heatmap {
		if h.Value > max {
			max = h.Value
		}
	}
	if res.BestValue != max {
		t.Fatalf("BestValue=%v != heatmap max %v", res.BestValue, max)
	}
}

func TestOptimizeEmptySpaceError(t *testing.T) {
	d := ramp(30)
	build := func(p Params) Strategy { return &buyAndHold{} }

	// empty space
	_, err := Optimize(d, Options{Cash: 10000, Margin: 1}, build, map[string][]any{}, OptimizeOptions{})
	if err == nil {
		t.Fatal("expected error for empty space")
	}

	// space that produces no combos after filtering
	space := map[string][]any{"n1": {5, 10}, "n2": {1, 2}}
	_, err = Optimize(d, Options{Cash: 10000, Margin: 1}, build, space,
		OptimizeOptions{Constraint: func(p Params) bool { return false }})
	if err == nil {
		t.Fatal("expected error when all combos filtered out")
	}
}

func TestOptimizeMaximizeFunc(t *testing.T) {
	d := ramp(120)
	space := map[string][]any{"n1": {5, 10}, "n2": {20, 40}}
	build := func(p Params) Strategy { return &paramSma{n1: p["n1"].(int), n2: p["n2"].(int)} }
	res, err := Optimize(d, Options{Cash: 10000, Margin: 1, FinalizeTrades: true}, build, space,
		OptimizeOptions{MaximizeFunc: func(s Stats) float64 { return s.ReturnPct }, ReturnHeatmap: true})
	if err != nil {
		t.Fatal(err)
	}
	if res.Best == nil {
		t.Fatal("no best found")
	}
	// BestStats must be populated and consistent: BestValue came from the func over BestStats.
	if res.BestStats.ReturnPct != res.BestValue {
		t.Fatalf("MaximizeFunc: BestValue=%v but BestStats.ReturnPct=%v", res.BestValue, res.BestStats.ReturnPct)
	}
}

func TestOptimizeRace(t *testing.T) {
	// Run with multiple workers and many combos; -race must be clean.
	d := ramp(60)
	space := map[string][]any{"n1": {5, 10, 15}, "n2": {20, 30, 40}}
	build := func(p Params) Strategy {
		return &paramSma{n1: p["n1"].(int), n2: p["n2"].(int)}
	}
	_, err := Optimize(d, Options{Cash: 10000, Margin: 1, FinalizeTrades: true}, build, space,
		OptimizeOptions{Workers: 4, Maximize: "SQN"})
	if err != nil {
		t.Fatal(err)
	}
}

// ---------------------------------------------------------------------------
// Task 1: Data.clone
// ---------------------------------------------------------------------------

func TestDataCloneIsIndependent(t *testing.T) {
	d := ramp(30) // helper returning *Data (define if absent: 30 bars)
	a := d.clone()
	b := d.clone()
	a.setLen(5)
	b.setLen(20)
	if a.Len() != 5 || b.Len() != 20 {
		t.Fatalf("clones share n: a=%d b=%d", a.Len(), b.Len())
	}
	// extra maps independent
	a.AddColumn("x", make([]float64, d.fullLen()))
	if _, ok := b.Column("x"); ok {
		t.Fatal("clones share extra map")
	}
	// shared backing slices (clone is cheap, not a deep copy)
	if &a.close[0] != &d.close[0] {
		t.Fatal("clone should share the close backing slice (read-only)")
	}
}

func TestDataCloneConcurrentRunsNoRace(t *testing.T) {
	d := ramp(60)
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			dc := d.clone()
			for n := 1; n <= dc.fullLen(); n++ {
				dc.setLen(n)
				_ = dc.Close().Last()
			}
		}()
	}
	wg.Wait()
}

// ---------------------------------------------------------------------------
// Task 2: gridCombos / sampleCombos / filterCombos
// ---------------------------------------------------------------------------

func TestGridCombos(t *testing.T) {
	space := map[string][]any{"n1": {10, 20}, "n2": {50, 100, 150}}
	c := gridCombos(space)
	if len(c) != 6 { // 2 * 3
		t.Fatalf("combos=%d want 6", len(c))
	}
	// deterministic: keys sorted (n1 before n2), n1 outer
	if c[0]["n1"] != 10 || c[0]["n2"] != 50 {
		t.Fatalf("first combo wrong: %+v", c[0])
	}
}

func TestFilterAndSample(t *testing.T) {
	space := map[string][]any{"n1": {5, 10, 15}, "n2": {10, 20}}
	c := gridCombos(space)
	c = filterCombos(c, func(p Params) bool { return p["n1"].(int) < p["n2"].(int) })
	for _, p := range c {
		if p["n1"].(int) >= p["n2"].(int) {
			t.Fatalf("constraint not applied: %+v", p)
		}
	}
	s := sampleCombos(c, 2, 42)
	if len(s) != 2 {
		t.Fatalf("sample=%d want 2", len(s))
	}
	// deterministic with same seed
	s2 := sampleCombos(c, 2, 42)
	if s[0]["n1"] != s2[0]["n1"] || s[0]["n2"] != s2[0]["n2"] {
		t.Fatal("sampling not deterministic for same seed")
	}
}

// ---------------------------------------------------------------------------
// Task 3: statValue
// ---------------------------------------------------------------------------

func TestStatValue(t *testing.T) {
	s := Stats{SQN: 1.5, SharpeRatio: 0.7, ReturnPct: 12.3}
	for name, want := range map[string]float64{"SQN": 1.5, "Sharpe Ratio": 0.7, "Return [%]": 12.3} {
		got, ok := statValue(s, name)
		if !ok || got != want {
			t.Fatalf("statValue(%q)=%v,%v want %v", name, got, ok, want)
		}
	}
	if _, ok := statValue(s, "Nonexistent"); ok {
		t.Fatal("unknown stat should return false")
	}
}
