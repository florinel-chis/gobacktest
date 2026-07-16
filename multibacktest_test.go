package backtest

import (
	"math"
	"testing"
)

// aaplSlice loads the bundled AAPL fixture and returns a fresh *Data built from
// bars [from:to) — a simple way to carve multiple independent datasets out of one
// CSV for MultiBacktest tests, without needing a dedicated slicing constructor.
func aaplSlice(tb testing.TB, from, to int) *Data {
	tb.Helper()
	full := benchData(tb)
	bars := full.Bars()
	if to > len(bars) {
		tb.Fatalf("aaplSlice: to=%d exceeds %d bars", to, len(bars))
	}
	return FromBars(bars[from:to])
}

func multiOpts() Options {
	return Options{Cash: 10000, Margin: 1, FinalizeTrades: true}
}

// smaFactory returns a build func producing a fresh paramSma per call, matching
// MultiBacktest's "fresh Strategy per dataset" contract.
func smaFactory() func() Strategy {
	return func() Strategy { return &paramSma{n1: 10, n2: 20} }
}

func TestMultiBacktestMatchesIndividualRuns(t *testing.T) {
	d1 := aaplSlice(t, 0, 250)
	d2 := aaplSlice(t, 250, 500)

	mb := NewMultiBacktest([]*Data{d1, d2}, smaFactory(), multiOpts())
	results, err := mb.Run()
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 2 {
		t.Fatalf("len(results)=%d want 2", len(results))
	}

	// Independently run each dataset the "normal" way and compare.
	individually := []*Data{aaplSlice(t, 0, 250), aaplSlice(t, 250, 500)}
	for i, d := range individually {
		want, err := New(d, smaFactory()(), multiOpts()).Run()
		if err != nil {
			t.Fatalf("dataset %d: %v", i, err)
		}
		got := results[i]
		if got.FinalEquity != want.FinalEquity {
			t.Errorf("dataset %d: FinalEquity=%v want %v", i, got.FinalEquity, want.FinalEquity)
		}
		if len(got.Trades) != len(want.Trades) {
			t.Fatalf("dataset %d: len(Trades)=%d want %d", i, len(got.Trades), len(want.Trades))
		}
		for j := range got.Trades {
			if got.Trades[j].EntryPrice != want.Trades[j].EntryPrice ||
				got.Trades[j].ExitPrice != want.Trades[j].ExitPrice ||
				got.Trades[j].Size != want.Trades[j].Size {
				t.Errorf("dataset %d trade %d: got %+v want %+v", i, j, got.Trades[j], want.Trades[j])
			}
		}
	}
}

// TestMultiBacktestSharedDataset passes the SAME *Data pointer for every slot.
// Each run mutates per-run state (bar-view length, indicator columns), so
// without a per-run clone concurrent runs corrupt each other. All results must
// match a solo run of the same dataset. Run under -race to catch data races.
func TestMultiBacktestSharedDataset(t *testing.T) {
	shared := aaplSlice(t, 0, 250)

	want, err := New(aaplSlice(t, 0, 250), smaFactory()(), multiOpts()).Run()
	if err != nil {
		t.Fatal(err)
	}

	mb := NewMultiBacktest([]*Data{shared, shared, shared, shared}, smaFactory(), multiOpts()).Workers(4)
	results, err := mb.Run()
	if err != nil {
		t.Fatal(err)
	}
	for i, got := range results {
		if got.FinalEquity != want.FinalEquity {
			t.Errorf("run %d: FinalEquity=%v want %v", i, got.FinalEquity, want.FinalEquity)
		}
		if len(got.Trades) != len(want.Trades) {
			t.Errorf("run %d: len(Trades)=%d want %d", i, len(got.Trades), len(want.Trades))
		}
	}
	// The shared dataset must still expose its full bar view afterwards, so it
	// stays usable by the caller (e.g. for Compute or another run).
	if shared.Len() != shared.FullLen() {
		t.Errorf("shared dataset view length %d != full length %d after Run", shared.Len(), shared.FullLen())
	}
}

func TestMultiBacktestStatsAndMean(t *testing.T) {
	datasets := []*Data{ramp(60), ramp(90), ramp(120)}
	mb := NewMultiBacktest(datasets, func() Strategy { return &buyAndHold{} }, multiOpts())

	stats, err := mb.Stats(0)
	if err != nil {
		t.Fatal(err)
	}
	if len(stats) != len(datasets) {
		t.Fatalf("len(stats)=%d want %d", len(stats), len(datasets))
	}

	sum := 0.0
	for _, s := range stats {
		sum += s.ReturnPct
	}
	wantMean := sum / float64(len(stats))

	got, err := mb.MeanStat(0, func(s Stats) float64 { return s.ReturnPct })
	if err != nil {
		t.Fatal(err)
	}
	if math.Abs(got-wantMean) > 1e-9 {
		t.Fatalf("MeanStat=%v want %v", got, wantMean)
	}
}

func TestMultiBacktestOrderDeterministic(t *testing.T) {
	datasets := []*Data{ramp(40), ramp(70), ramp(55), ramp(100)}

	var prevEquities []float64
	for run := 0; run < 5; run++ {
		mb := NewMultiBacktest(datasets, func() Strategy { return &buyAndHold{} }, multiOpts())
		results, err := mb.Run()
		if err != nil {
			t.Fatal(err)
		}
		equities := make([]float64, len(results))
		for i, r := range results {
			equities[i] = r.FinalEquity
		}
		if prevEquities != nil {
			for i := range equities {
				if equities[i] != prevEquities[i] {
					t.Fatalf("run %d: order/value not stable at index %d: got %v want %v",
						run, i, equities[i], prevEquities[i])
				}
			}
		}
		prevEquities = equities
	}
}

func TestMultiBacktestEmpty(t *testing.T) {
	mb := NewMultiBacktest(nil, func() Strategy { return &buyAndHold{} }, multiOpts())
	results, err := mb.Run()
	if err != nil {
		t.Fatalf("empty datasets: unexpected error %v", err)
	}
	if results != nil {
		t.Fatalf("empty datasets: results=%v want nil", results)
	}
}

func TestMultiBacktestRace(t *testing.T) {
	datasets := []*Data{ramp(60), ramp(90), ramp(120), ramp(45), ramp(200), ramp(150)}
	mb := NewMultiBacktest(datasets, smaFactory(), multiOpts()).Workers(4)
	if _, err := mb.Run(); err != nil {
		t.Fatal(err)
	}
}
