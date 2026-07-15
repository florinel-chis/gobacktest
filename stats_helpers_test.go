package backtest

import (
	"math"
	"testing"
	"time"
)

func TestGeometricMean(t *testing.T) {
	// returns 0.1, 0.2, -0.05 -> exp(mean(ln(1.1),ln(1.2),ln(0.95)))-1
	g := geometricMean([]float64{0.1, 0.2, -0.05})
	want := math.Exp((math.Log(1.1)+math.Log(1.2)+math.Log(0.95))/3) - 1
	if math.Abs(g-want) > 1e-12 {
		t.Fatalf("geometricMean=%v want %v", g, want)
	}
	if geometricMean([]float64{-1.0, 0.5}) != 0 { // 1+(-1)=0 -> guard returns 0
		t.Fatal("non-positive factor must return 0")
	}
}

func TestAnnualTradingDays(t *testing.T) {
	// daily weekday index (no weekends) -> 252
	var idx []time.Time
	d := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC) // Monday
	for i := 0; i < 20; i++ {
		wd := d.AddDate(0, 0, i).Weekday()
		if wd != time.Saturday && wd != time.Sunday {
			idx = append(idx, d.AddDate(0, 0, i))
		}
	}
	if got := annualTradingDays(idx); got != 252 {
		t.Fatalf("annualTradingDays(weekday daily)=%v want 252", got)
	}
}

func TestAnnualTradingDaysBranches(t *testing.T) {
	start := time.Date(2021, 1, 4, 0, 0, 0, 0, time.UTC) // Monday
	mk := func(step time.Duration, n int) []time.Time {
		idx := make([]time.Time, n)
		for i := range idx {
			idx[i] = start.Add(time.Duration(i) * step)
		}
		return idx
	}
	cases := []struct {
		name string
		idx  []time.Time
		want float64
	}{
		{"weekly", mk(7*24*time.Hour, 6), 52},
		{"monthly", mk(30*24*time.Hour, 6), 12},
		{"yearly", mk(365*24*time.Hour, 4), 1},
		{"calendar-daily-with-weekends", mk(24*time.Hour, 14), 365},
	}
	for _, c := range cases {
		if got := annualTradingDays(c.idx); got != c.want {
			t.Fatalf("%s: annualTradingDays=%v want %v", c.name, got, c.want)
		}
	}
}

// TestAnnualTradingDaysAnomalousFirstGap verifies the period is inferred from
// the MEDIAN gap (as in backtesting.py's _data_period), so a single anomalous
// leading gap cannot misclassify a daily series as weekly.
func TestAnnualTradingDaysAnomalousFirstGap(t *testing.T) {
	// First gap is 7 days (Mon → next Mon), then daily weekday bars.
	idx := []time.Time{time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)} // Monday
	d := time.Date(2024, 1, 8, 0, 0, 0, 0, time.UTC)                // next Monday
	for i := 0; i < 20; i++ {
		wd := d.AddDate(0, 0, i).Weekday()
		if wd != time.Saturday && wd != time.Sunday {
			idx = append(idx, d.AddDate(0, 0, i))
		}
	}
	if got := annualTradingDays(idx); got != 252 {
		t.Fatalf("annualTradingDays(daily with 7-day first gap)=%v want 252", got)
	}
}

func TestGeometricMeanStrictlyNegativeFactor(t *testing.T) {
	if geometricMean([]float64{-1.5}) != 0 {
		t.Fatal("r < -1 (factor < 0) must return 0")
	}
}

func TestDrawdownSeries(t *testing.T) {
	// equity [100, 120, 90, 130]:
	// runningMax = [100, 120, 120, 130]
	// dd         = [0,   0,  0.25, 0]
	equity := []float64{100, 120, 90, 130}
	dd := drawdownSeries(equity)
	want := []float64{0, 0, 0.25, 0}
	for i, w := range want {
		if math.Abs(dd[i]-w) > 1e-9 {
			t.Fatalf("dd[%d]=%.10f want %.10f", i, dd[i], w)
		}
	}
}

func TestDrawdownDurationsAndPeaks(t *testing.T) {
	t0 := time.Unix(0, 0)
	day := 24 * time.Hour
	curve := []EquityPoint{
		{Time: t0, Equity: 100},
		{Time: t0.Add(day), Equity: 120},     // HWM
		{Time: t0.Add(2 * day), Equity: 90},  // below HWM
		{Time: t0.Add(3 * day), Equity: 130}, // new HWM
	}
	durs, peaks := drawdownDurationsAndPeaks(curve)
	// one span: HWM@day1 → HWM@day3 = 2 days; peak dd = 1-90/120 = 0.25
	if len(durs) != 1 {
		t.Fatalf("expected 1 span, got %d (durs=%v peaks=%v)", len(durs), durs, peaks)
	}
	if durs[0] != 2*day {
		t.Fatalf("duration=%v want %v", durs[0], 2*day)
	}
	if math.Abs(peaks[0]-0.25) > 1e-9 {
		t.Fatalf("peak=%v want 0.25", peaks[0])
	}
}

func TestDrawdownDurationsUnrecovered(t *testing.T) {
	// Final unrecovered drawdown: never makes a new high → last span counted.
	t0 := time.Unix(0, 0)
	day := 24 * time.Hour
	curve := []EquityPoint{
		{Time: t0, Equity: 100},
		{Time: t0.Add(day), Equity: 120},    // HWM
		{Time: t0.Add(2 * day), Equity: 90}, // never recovered
	}
	durs, peaks := drawdownDurationsAndPeaks(curve)
	if len(durs) != 1 {
		t.Fatalf("expected 1 span, got %d", len(durs))
	}
	if durs[0] != day {
		t.Fatalf("duration=%v want %v", durs[0], day)
	}
	if math.Abs(peaks[0]-0.25) > 1e-9 {
		t.Fatalf("peak=%v want 0.25", peaks[0])
	}
}

func TestDayReturns(t *testing.T) {
	t0 := time.Unix(0, 0)
	day := 24 * time.Hour
	curve := []EquityPoint{
		{Time: t0, Equity: 100},
		{Time: t0.Add(day), Equity: 110},
		{Time: t0.Add(2 * day), Equity: 99},
	}
	got := dayReturns(curve)
	// [110/100-1, 99/110-1] = [0.1, -0.1]
	want := []float64{0.1, 99.0/110 - 1}
	if len(got) != len(want) {
		t.Fatalf("dayReturns len=%d want %d", len(got), len(want))
	}
	for i, w := range want {
		if math.Abs(got[i]-w) > 1e-12 {
			t.Fatalf("dayReturns[%d]=%v want %v", i, got[i], w)
		}
	}
	// Edge cases.
	if dayReturns(nil) != nil {
		t.Fatal("nil curve must return nil")
	}
	if dayReturns([]EquityPoint{{Time: t0, Equity: 100}}) != nil {
		t.Fatal("single-point curve must return nil")
	}
}

func TestGeometricMeanN(t *testing.T) {
	// geometricMeanN([0.1, 0.2, -0.05], n=4) divides by 4 not 3.
	rets := []float64{0.1, 0.2, -0.05}
	wantN3 := geometricMean(rets) // uses len=3
	gotN3 := geometricMeanN(rets, 3)
	if math.Abs(gotN3-wantN3) > 1e-12 {
		t.Fatalf("geometricMeanN n==len: got %v want %v", gotN3, wantN3)
	}
	// With n=4 the divisor is larger → result closer to 0.
	gotN4 := geometricMeanN(rets, 4)
	wantN4 := math.Exp((math.Log(1.1)+math.Log(1.2)+math.Log(0.95))/4) - 1
	if math.Abs(gotN4-wantN4) > 1e-12 {
		t.Fatalf("geometricMeanN n=4 = %v want %v", gotN4, wantN4)
	}
	if !(math.Abs(gotN4) < math.Abs(gotN3)) {
		t.Fatalf("geometricMeanN n=4 should be smaller-magnitude than n=3: got %v vs %v", gotN4, gotN3)
	}
	// Guard: non-positive factor returns 0.
	if geometricMeanN([]float64{-1.0, 0.5}, 3) != 0 {
		t.Fatal("non-positive factor must return 0")
	}
}

func TestSampleVariance(t *testing.T) {
	// population = [2, 4, 4, 4, 5, 5, 7, 9]; sample var (ddof=1) known to be 4.571...
	data := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	// sum = 40, mean = 5; ss = 9+1+1+1+0+0+4+16=32; sample var = 32/7 = 4.571428...
	want := 32.0 / 7
	got := sampleVariance(data)
	if math.Abs(got-want) > 1e-10 {
		t.Fatalf("sampleVariance=%v want %v", got, want)
	}
	// Single element → 0.
	if sampleVariance([]float64{42}) != 0 {
		t.Fatal("sampleVariance of 1 element must be 0")
	}
	// Empty → 0.
	if sampleVariance(nil) != 0 {
		t.Fatal("sampleVariance of nil must be 0")
	}
}

func TestDrawdownSeriesFlat(t *testing.T) {
	// All zeroes → no drawdown
	dd := drawdownSeries([]float64{100, 100, 100})
	for i, d := range dd {
		if d != 0 {
			t.Fatalf("dd[%d]=%v want 0 for flat equity", i, d)
		}
	}
}

func TestDrawdownDurationsEmpty(t *testing.T) {
	durs, peaks := drawdownDurationsAndPeaks(nil)
	if durs != nil || peaks != nil {
		t.Fatal("empty curve must return nil slices")
	}
}

func TestSampleCovariance(t *testing.T) {
	// x = [1, 2, 3], y = [4, 5, 6] → perfectly correlated.
	// mean_x=2, mean_y=5; cov = ((1-2)(4-5)+(2-2)(5-5)+(3-2)(6-5))/(3-1) = (1+0+1)/2 = 1.
	x := []float64{1, 2, 3}
	y := []float64{4, 5, 6}
	got := sampleCovariance(x, y)
	if math.Abs(got-1.0) > 1e-12 {
		t.Fatalf("sampleCovariance=%v want 1.0", got)
	}

	// x = y → sampleCovariance(x, x) == sampleVariance(x).
	z := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	covXX := sampleCovariance(z, z)
	varX := sampleVariance(z)
	if math.Abs(covXX-varX) > 1e-10 {
		t.Fatalf("sampleCovariance(x,x)=%v sampleVariance(x)=%v: should be equal", covXX, varX)
	}

	// Too few points → 0.
	if sampleCovariance([]float64{1}, []float64{2}) != 0 {
		t.Fatal("sampleCovariance of 1-element slices must be 0")
	}
	// Mismatched lengths → 0.
	if sampleCovariance([]float64{1, 2}, []float64{1}) != 0 {
		t.Fatal("mismatched-length sampleCovariance must be 0")
	}
}
