//go:build parity

// Package backtest_test (external test package): parity_test.go exercises
// lib.TrailStop, which imports package backtest — an internal (package
// backtest) test file importing lib would be an import cycle. The dot-import
// keeps every exported backtest identifier (New, Options, Compute, Stats,
// State, Series, Indicator, ...) unqualified exactly as before.
package backtest_test

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	. "github.com/florinel-chis/gobacktest"
	indicators "github.com/florinel-chis/gobacktest/indicators"
	"github.com/florinel-chis/gobacktest/lib"
)

type goldenFixture struct {
	Stats  map[string]any   `json:"stats"`
	Trades []map[string]any `json:"trades"`
	Equity []float64        `json:"equity"`
}

// metricMap maps a Go Stats field to the backtesting.py stats key it must match.
// Only the metrics implemented so far are listed; later tasks extend this.
var metricMap = []struct {
	pyKey string
	get   func(Stats) float64
	exact bool
}{
	{"Equity Final [$]", func(s Stats) float64 { return s.EquityFinal }, false},
	{"Equity Peak [$]", func(s Stats) float64 { return s.EquityPeak }, false},
	{"Return [%]", func(s Stats) float64 { return s.ReturnPct }, false},
	{"Buy & Hold Return [%]", func(s Stats) float64 { return s.BuyHoldReturnPct }, false},
	{"# Trades", func(s Stats) float64 { return float64(s.NumTrades) }, true},
	{"Exposure Time [%]", func(s Stats) float64 { return s.ExposureTimePct }, false},
	{"Max. Drawdown [%]", func(s Stats) float64 { return s.MaxDrawdownPct }, false},
	{"Avg. Drawdown [%]", func(s Stats) float64 { return s.AvgDrawdownPct }, false},
	// Task 4: annualised return, volatility, CAGR.
	{"Return (Ann.) [%]", func(s Stats) float64 { return s.ReturnAnnPct }, false},
	{"Volatility (Ann.) [%]", func(s Stats) float64 { return s.VolatilityAnnPct }, false},
	{"CAGR [%]", func(s Stats) float64 { return s.CAGRPct }, false},
	// Task 5: risk ratios.
	{"Sharpe Ratio", func(s Stats) float64 { return s.SharpeRatio }, false},
	{"Sortino Ratio", func(s Stats) float64 { return s.SortinoRatio }, false},
	{"Calmar Ratio", func(s Stats) float64 { return s.CalmarRatio }, false},
	// Task 6: Alpha and Beta.
	{"Alpha [%]", func(s Stats) float64 { return s.AlphaPct }, false},
	{"Beta", func(s Stats) float64 { return s.Beta }, false},
	// Task 7: trade statistics.
	{"Win Rate [%]", func(s Stats) float64 { return s.WinRatePct }, false},
	{"Best Trade [%]", func(s Stats) float64 { return s.BestTradePct }, false},
	{"Worst Trade [%]", func(s Stats) float64 { return s.WorstTradePct }, false},
	{"Avg. Trade [%]", func(s Stats) float64 { return s.AvgTradePct }, false},
	{"Profit Factor", func(s Stats) float64 { return s.ProfitFactor }, false},
	{"Expectancy [%]", func(s Stats) float64 { return s.ExpectancyPct }, false},
	{"SQN", func(s Stats) float64 { return s.SQN }, false},
	{"Kelly Criterion", func(s Stats) float64 { return s.KellyCriterion }, false},
	// Task 8: commissions — absent in zero-commission scenarios (toFloat skips nil/NaN).
	{"Commissions [$]", func(s Stats) float64 { return s.CommissionsTotal }, false},
}

// durationMetricMap maps a Go Stats duration field to a backtesting.py key.
var durationMetricMap = []struct {
	pyKey string
	get   func(Stats) time.Duration
}{
	{"Max. Drawdown Duration", func(s Stats) time.Duration { return s.MaxDrawdownDur }},
	{"Avg. Drawdown Duration", func(s Stats) time.Duration { return s.AvgDrawdownDur }},
	// Task 7: trade durations.
	{"Max. Trade Duration", func(s Stats) time.Duration { return s.MaxTradeDur }},
	{"Avg. Trade Duration", func(s Stats) time.Duration { return s.AvgTradeDur }},
}

// parseTimedeltaDays parses a pandas Timedelta string ("N days HH:MM:SS" or "N days")
// and returns the total duration in fractional days.
func parseTimedeltaDays(s string) (float64, bool) {
	var days, h, m, sec int
	if n, _ := fmt.Sscanf(s, "%d days %d:%d:%d", &days, &h, &m, &sec); n == 4 {
		return float64(days) + float64(h)/24 + float64(m)/1440 + float64(sec)/86400, true
	}
	if n, _ := fmt.Sscanf(s, "%d days", &days); n == 1 {
		return float64(days), true
	}
	return 0, false
}

func TestParseTimedeltaDays(t *testing.T) {
	cases := []struct {
		in   string
		days float64
		ok   bool
	}{
		{"74 days 00:00:00", 74, true},
		{"45 days", 45, true},
		{"3 days 12:00:00", 3.5, true},
		{"garbage", 0, false},
	}
	for _, c := range cases {
		d, ok := parseTimedeltaDays(c.in)
		if ok != c.ok || (ok && math.Abs(d-c.days) > 1e-9) {
			t.Fatalf("parseTimedeltaDays(%q)=%v,%v want %v,%v", c.in, d, ok, c.days, c.ok)
		}
	}
}

func TestParityMatrix(t *testing.T) {
	dir := "testdata/parity"
	files, _ := filepath.Glob(filepath.Join(dir, "*.golden.json"))
	if len(files) == 0 {
		t.Skip("no golden fixtures — run scripts/gen_fixtures.py (needs the venv)")
	}
	var report []string
	report = append(report, "# Parity report\n\n| scenario | metric | go | py | rel.dev | ok |", "|---|---|---|---|---|---|")
	pass, total := 0, 0
	for _, f := range files {
		name := filepath.Base(f)
		var g goldenFixture
		b, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("%s: read: %v", name, err)
		}
		if err := json.Unmarshal(b, &g); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if g.Stats == nil {
			// Not a single-run fixture (e.g. optimize_aapl.golden.json); skip.
			continue
		}
		got := runScenario(t, name)
		for _, m := range metricMap {
			pyv, ok := toFloat(g.Stats[m.pyKey])
			if !ok {
				continue
			}
			gov := m.get(got)
			total++
			var dev float64
			if m.exact {
				dev = math.Abs(gov - pyv)
			} else if pyv != 0 {
				dev = math.Abs(gov-pyv) / math.Abs(pyv)
			} else {
				dev = math.Abs(gov - pyv)
			}
			tol := 1e-6
			if m.exact {
				tol = 0
			}
			okFlag := dev <= tol
			if okFlag {
				pass++
			}
			report = append(report, sprintRow(name, m.pyKey, gov, pyv, dev, okFlag))
			if !okFlag {
				t.Errorf("%s %q: go=%v py=%v reldev=%.2e", name, m.pyKey, gov, pyv, dev)
			}
		}
		// Duration metrics: compare in fractional days; tolerance exact match (1e-6).
		for _, m := range durationMetricMap {
			pyStr, ok := g.Stats[m.pyKey].(string)
			if !ok {
				continue
			}
			pyDays, ok := parseTimedeltaDays(pyStr)
			if !ok {
				continue
			}
			goDays := m.get(got).Hours() / 24
			total++
			diff := math.Abs(goDays - pyDays)
			okFlag := diff < 1e-6
			if okFlag {
				pass++
			}
			report = append(report, sprintRow(name, m.pyKey, goDays, pyDays, diff, okFlag))
			if !okFlag {
				t.Errorf("%s %q: go=%.2f days py=%.2f days diff=%.2f", name, m.pyKey, goDays, pyDays, diff)
			}
		}
	}
	if total == 0 {
		t.Fatal("no metrics were compared — check metricMap keys against the golden JSON")
	}
	report = append(report, "", sprintSummary(pass, total))
	os.WriteFile(filepath.Join(dir, "..", "parity-report.md"), []byte(joinLines(report)), 0o644)
	t.Logf("parity: %d/%d metrics within tolerance", pass, total)
}

// toFloat converts a JSON-decoded value (float64, int, string) to float64.
// Returns (0, false) for nil (JSON null), NaN, Inf, or unparseable strings,
// causing the caller to skip the metric rather than comparing against 0.
func toFloat(v any) (float64, bool) {
	if v == nil {
		return 0, false // JSON null (e.g. Python NaN serialised as null)
	}
	switch x := v.(type) {
	case float64:
		if math.IsNaN(x) || math.IsInf(x, 0) {
			return 0, false
		}
		return x, true
	case int:
		return float64(x), true
	case string:
		lower := strings.ToLower(x)
		if lower == "nan" || lower == "inf" || lower == "-inf" {
			return 0, false
		}
		var f float64
		if _, err := fmt.Sscanf(x, "%g", &f); err == nil {
			return f, true
		}
	}
	return 0, false
}

func sprintRow(scenario, metric string, gov, pyv, dev float64, ok bool) string {
	okStr := "YES"
	if !ok {
		okStr = "NO"
	}
	return fmt.Sprintf("| %s | %s | %.6g | %.6g | %.2e | %s |", scenario, metric, gov, pyv, dev, okStr)
}

func sprintSummary(pass, total int) string {
	return fmt.Sprintf("**Result: %d/%d metrics within tolerance**", pass, total)
}

func joinLines(lines []string) string {
	return strings.Join(lines, "\n") + "\n"
}

// parityLocalSMA delegates to indicators.SMA (TA-Lib backed), which emits NaN
// for the first period-1 bars — identical warmup to the hand-rolled version.
// Named parityLocalSMA (not localSMA) to avoid a duplicate-symbol conflict with the
// localSMA helper already defined in engine_smacross_test.go (no build tag).
func parityLocalSMA(s Series, period int) []float64 {
	return indicators.SMA([]float64(s), period)
}

type paritySmaCross struct{ fast, slow *Indicator }

func (st *paritySmaCross) Init(s *State) {
	st.fast = s.I("fast", func() []float64 { return parityLocalSMA(s.Data().Close(), 10) })
	st.slow = s.I("slow", func() []float64 { return parityLocalSMA(s.Data().Close(), 20) })
}
func (st *paritySmaCross) Next(s *State) {
	crossUp := st.fast.At(1) < st.slow.At(1) && st.fast.Last() > st.slow.Last()
	crossDn := st.fast.At(1) > st.slow.At(1) && st.fast.Last() < st.slow.Last()
	if crossUp && s.Position().Size() == 0 {
		s.Buy(Order{Size: 10})
	} else if crossDn && s.Position().IsLong() {
		s.Position().Close()
	}
}

// paritySmaCrossSLTP is SMA cross with SL=5% below and TP=10% above entry close.
type paritySmaCrossSLTP struct{ fast, slow *Indicator }

func (st *paritySmaCrossSLTP) Init(s *State) {
	st.fast = s.I("fast", func() []float64 { return parityLocalSMA(s.Data().Close(), 10) })
	st.slow = s.I("slow", func() []float64 { return parityLocalSMA(s.Data().Close(), 20) })
}
func (st *paritySmaCrossSLTP) Next(s *State) {
	crossUp := st.fast.At(1) < st.slow.At(1) && st.fast.Last() > st.slow.Last()
	crossDn := st.fast.At(1) > st.slow.At(1) && st.fast.Last() < st.slow.Last()
	if crossUp && s.Position().Size() == 0 {
		price := s.Data().Close().Last()
		s.Buy(Order{Size: 10, SL: price * 0.95, TP: price * 1.10})
	} else if crossDn && s.Position().IsLong() {
		s.Position().Close()
	}
}

// paritySmaCrossLeverage uses size=300 with margin=0.5 (2× leverage).
type paritySmaCrossLeverage struct{ fast, slow *Indicator }

func (st *paritySmaCrossLeverage) Init(s *State) {
	st.fast = s.I("fast", func() []float64 { return parityLocalSMA(s.Data().Close(), 10) })
	st.slow = s.I("slow", func() []float64 { return parityLocalSMA(s.Data().Close(), 20) })
}
func (st *paritySmaCrossLeverage) Next(s *State) {
	crossUp := st.fast.At(1) < st.slow.At(1) && st.fast.Last() > st.slow.Last()
	crossDn := st.fast.At(1) > st.slow.At(1) && st.fast.Last() < st.slow.Last()
	if crossUp && s.Position().Size() == 0 {
		s.Buy(Order{Size: 300})
	} else if crossDn && s.Position().IsLong() {
		s.Position().Close()
	}
}

// paritySmaCrossFrac uses FRACTIONAL sizing — buy 50% of available equity.
type paritySmaCrossFrac struct{ fast, slow *Indicator }

func (st *paritySmaCrossFrac) Init(s *State) {
	st.fast = s.I("fast", func() []float64 { return parityLocalSMA(s.Data().Close(), 10) })
	st.slow = s.I("slow", func() []float64 { return parityLocalSMA(s.Data().Close(), 20) })
}
func (st *paritySmaCrossFrac) Next(s *State) {
	crossUp := st.fast.At(1) < st.slow.At(1) && st.fast.Last() > st.slow.Last()
	crossDn := st.fast.At(1) > st.slow.At(1) && st.fast.Last() < st.slow.Last()
	if crossUp && s.Position().Size() == 0 {
		s.Buy(Order{Size: 0.5})
	} else if crossDn && s.Position().IsLong() {
		s.Position().Close()
	}
}

// parityTrailing is the matched Go port of backtesting.lib.TrailingStrategy's
// SmaTrailing scenario: SMA(10)/SMA(20) crossover entries, with the SL trailed
// every bar to close - 3*TrailingATR(20). There is no explicit close — the
// trailing SL (a contingent SL order processed each bar) is the only exit.
type parityTrailing struct {
	fast, slow, atr *Indicator
}

func (st *parityTrailing) Init(s *State) {
	st.fast = s.I("fast", func() []float64 { return parityLocalSMA(s.Data().Close(), 10) })
	st.slow = s.I("slow", func() []float64 { return parityLocalSMA(s.Data().Close(), 20) })
	st.atr = s.I("atr", func() []float64 {
		return lib.TrailingATR(s.Data().High(), s.Data().Low(), s.Data().Close(), 20)
	})
}
func (st *parityTrailing) Next(s *State) {
	crossUp := st.fast.At(1) < st.slow.At(1) && st.fast.Last() > st.slow.Last()
	if crossUp && s.Position().Size() == 0 {
		s.Buy(Order{Size: 10})
	}
	lib.TrailStop(s, st.atr.Last(), 3)
}

// parityCrossHold enters on the first bullish SMA cross (a data-determined bar,
// identical across engines) and NEVER exits, leaving a position OPEN at the last
// bar so finalize_trades must close it. This isolates the finalize exit-price
// path (last-bar open), which the crossover scenarios never reach because they
// flatten before the final bar.
type parityCrossHold struct{ fast, slow *Indicator }

func (st *parityCrossHold) Init(s *State) {
	st.fast = s.I("fast", func() []float64 { return parityLocalSMA(s.Data().Close(), 10) })
	st.slow = s.I("slow", func() []float64 { return parityLocalSMA(s.Data().Close(), 20) })
}
func (st *parityCrossHold) Next(s *State) {
	crossUp := st.fast.At(1) < st.slow.At(1) && st.fast.Last() > st.slow.Last()
	if crossUp && s.Position().Size() == 0 {
		s.Buy(Order{Size: 10})
	}
}

// runScenario maps a golden fixture name to the matched Go run.
func runScenario(t *testing.T, goldenName string) Stats {
	switch goldenName {
	case "frac_aapl.golden.json":
		d := firstNRows(t, "testdata/AAPL_1d.csv", 250)
		bt := New(d, &paritySmaCrossFrac{}, Options{Cash: 10000, Margin: 1, FinalizeTrades: true})
		res, err := bt.Run()
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return Compute(res, d, 0)
	case "frac_comm_aapl.golden.json":
		d := firstNRows(t, "testdata/AAPL_1d.csv", 250)
		bt := New(d, &paritySmaCrossFrac{}, Options{Cash: 10000, Commission: Pct(0.002), FinalizeTrades: true})
		res, err := bt.Run()
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return Compute(res, d, 0)
	case "frac_lev_aapl.golden.json":
		d := firstNRows(t, "testdata/AAPL_1d.csv", 250)
		bt := New(d, &paritySmaCrossFrac{}, Options{Cash: 10000, Margin: 0.5, FinalizeTrades: true})
		res, err := bt.Run()
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return Compute(res, d, 0)
	case "buyhold_aapl.golden.json":
		// Position open at the final bar → exercises finalize_trades exit price.
		d := firstNRows(t, "testdata/AAPL_1d.csv", 250)
		bt := New(d, &parityCrossHold{}, Options{Cash: 10000, Margin: 1, FinalizeTrades: true})
		res, err := bt.Run()
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return Compute(res, d, 0)
	case "sma_aapl.golden.json":
		d := firstNRows(t, "testdata/AAPL_1d.csv", 250)
		bt := New(d, &paritySmaCross{}, Options{Cash: 10000, Margin: 1, FinalizeTrades: true})
		res, err := bt.Run()
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return Compute(res, d, 0)
	case "sltp_aapl.golden.json":
		// Exercises SL/TP contingent fills intrabar; same size=10 as base.
		d := firstNRows(t, "testdata/AAPL_1d.csv", 250)
		bt := New(d, &paritySmaCrossSLTP{}, Options{Cash: 10000, Margin: 1, FinalizeTrades: true})
		res, err := bt.Run()
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return Compute(res, d, 0)
	case "commission_aapl.golden.json":
		// Validates Commissions [$] stat with commission=0.002 Pct model.
		d := firstNRows(t, "testdata/AAPL_1d.csv", 250)
		bt := New(d, &paritySmaCross{}, Options{Cash: 10000, Commission: Pct(0.002), FinalizeTrades: true})
		res, err := bt.Run()
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return Compute(res, d, 0)
	case "trailing_aapl.golden.json":
		// Exercises lib.TrailingATR + lib.TrailStop (backtesting.lib.TrailingStrategy parity).
		d := firstNRows(t, "testdata/AAPL_1d.csv", 250)
		bt := New(d, &parityTrailing{}, Options{Cash: 10000, Margin: 1, FinalizeTrades: true})
		res, err := bt.Run()
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return Compute(res, d, 0)
	case "leverage_aapl.golden.json":
		// Exercises margin/leverage accounting with size=300, margin=0.5 (2× leverage).
		d := firstNRows(t, "testdata/AAPL_1d.csv", 250)
		bt := New(d, &paritySmaCrossLeverage{}, Options{Cash: 10000, Margin: 0.5, FinalizeTrades: true})
		res, err := bt.Run()
		if err != nil {
			t.Fatalf("run: %v", err)
		}
		return Compute(res, d, 0)
	default:
		t.Fatalf("no Go scenario for %s", goldenName)
		return Stats{}
	}
}

// parityOptStrat is the matched SMA-cross strategy parameterized by n1/n2 for
// use in TestOptimizeParity. Identical crossover logic to paritySmaCross.
type parityOptStrat struct {
	n1, n2     int
	fast, slow *Indicator
}

func (st *parityOptStrat) Init(s *State) {
	n1, n2 := st.n1, st.n2
	st.fast = s.I("fast", func() []float64 { return parityLocalSMA(s.Data().Close(), n1) })
	st.slow = s.I("slow", func() []float64 { return parityLocalSMA(s.Data().Close(), n2) })
}
func (st *parityOptStrat) Next(s *State) {
	crossUp := st.fast.At(1) < st.slow.At(1) && st.fast.Last() > st.slow.Last()
	crossDn := st.fast.At(1) > st.slow.At(1) && st.fast.Last() < st.slow.Last()
	if crossUp && s.Position().Size() == 0 {
		s.Buy(Order{Size: 10})
	} else if crossDn && s.Position().IsLong() {
		s.Position().Close()
	}
}

func TestOptimizeParity(t *testing.T) {
	goldenPath := "testdata/parity/optimize_aapl.golden.json"
	b, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Skipf("optimize golden not found: %v (run scripts/gen_fixtures.py)", err)
	}
	var golden struct {
		Heatmap   map[string]float64 `json:"heatmap"`
		Best      map[string]int     `json:"best"`
		BestValue float64            `json:"best_value"`
	}
	if err := json.Unmarshal(b, &golden); err != nil {
		t.Fatalf("parse optimize golden: %v", err)
	}

	d := firstNRows(t, "testdata/AAPL_1d.csv", 250)
	res, err := Optimize(d, Options{Cash: 10000, Margin: 1, FinalizeTrades: true},
		func(p Params) Strategy { return &parityOptStrat{n1: p["n1"].(int), n2: p["n2"].(int)} },
		map[string][]any{"n1": {5, 10, 15}, "n2": {20, 30, 40}},
		OptimizeOptions{
			Maximize:      "SQN",
			ReturnHeatmap: true,
			Constraint:    func(p Params) bool { return p["n1"].(int) < p["n2"].(int) },
		})
	if err != nil {
		t.Fatalf("Optimize: %v", err)
	}

	// Build a lookup map from the Go heatmap (key: "n1=<v>,n2=<v>").
	goHeatmap := make(map[string]float64, len(res.Heatmap))
	for _, e := range res.Heatmap {
		key := fmt.Sprintf("n1=%d,n2=%d", e.Params["n1"].(int), e.Params["n2"].(int))
		goHeatmap[key] = e.Value
	}

	// Cardinality check: Go and golden heatmaps must have same number of entries.
	if len(goHeatmap) != len(golden.Heatmap) {
		t.Fatalf("heatmap cardinality mismatch: go=%d golden=%d", len(goHeatmap), len(golden.Heatmap))
	}

	// Compare per-combo SQN values within 1e-6 relative tolerance.
	matched, total := 0, 0
	for key, pyVal := range golden.Heatmap {
		total++
		goVal, ok := goHeatmap[key]
		if !ok {
			t.Errorf("combo %s: present in golden but missing from Go heatmap", key)
			continue
		}
		var relDev float64
		if pyVal != 0 {
			relDev = math.Abs(goVal-pyVal) / math.Abs(pyVal)
		} else {
			relDev = math.Abs(goVal - pyVal)
		}
		if relDev <= 1e-6 {
			matched++
		} else {
			t.Errorf("combo %s: go SQN=%.8f py SQN=%.8f reldev=%.2e (FAIL)", key, goVal, pyVal, relDev)
		}
	}
	t.Logf("optimize parity: %d/%d combos matched within 1e-6", matched, total)

	// Assert best params match.
	goBestN1, goBestN2 := res.Best["n1"].(int), res.Best["n2"].(int)
	pyBestN1, pyBestN2 := golden.Best["n1"], golden.Best["n2"]
	if goBestN1 != pyBestN1 || goBestN2 != pyBestN2 {
		t.Errorf("best params: go=(n1=%d,n2=%d) py=(n1=%d,n2=%d)",
			goBestN1, goBestN2, pyBestN1, pyBestN2)
	} else {
		t.Logf("best params match: n1=%d n2=%d (go SQN=%.8f py SQN=%.8f)",
			goBestN1, goBestN2, res.BestValue, golden.BestValue)
	}
	if total == 0 {
		t.Fatal("no golden combos to compare — check optimize_aapl.golden.json heatmap key")
	}
	if matched < total {
		t.Fatalf("optimize parity FAILED: only %d/%d combos matched", matched, total)
	}
}

// firstNRows reads a CSV file and returns a *Data with only the first nRows data rows.
func firstNRows(t *testing.T, path string, nRows int) *Data {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("firstNRows: open %s: %v", path, err)
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatalf("firstNRows: read %s: %v", path, err)
	}
	if len(rows) < 2 {
		t.Fatalf("firstNRows: %s has no data rows", path)
	}

	// Write header + first nRows data rows to a temp file, then FromCSV it.
	end := nRows + 1 // +1 for header
	if end > len(rows) {
		end = len(rows)
	}
	slice := rows[:end]

	tmp, err := os.CreateTemp(t.TempDir(), "parity_*.csv")
	if err != nil {
		t.Fatalf("firstNRows: create temp: %v", err)
	}
	defer tmp.Close()

	w := csv.NewWriter(tmp)
	if err := w.WriteAll(slice); err != nil {
		t.Fatalf("firstNRows: write temp: %v", err)
	}
	w.Flush()
	tmp.Close()

	d, err := FromCSV(tmp.Name())
	if err != nil {
		t.Fatalf("firstNRows: FromCSV: %v", err)
	}
	return d
}
