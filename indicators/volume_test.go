package indicators

import (
	"math"
	"testing"
)

func TestOBVDefinedFromBarZero(t *testing.T) {
	_, _, c := ramp(20)
	vol := make([]float64, 20)
	for i := range vol {
		vol[i] = 1000 + float64(i)
	}
	out := OBV(c, vol)
	if len(out) != 20 {
		t.Fatalf("len=%d want 20", len(out))
	}
	// OBV lookback = 0: defined from bar 0.
	if math.IsNaN(out[0]) {
		t.Fatal("OBV[0] should be a value, not NaN (lookback 0)")
	}
	if out[0] != vol[0] {
		t.Fatalf("OBV[0]=%v want %v (seeded with first volume)", out[0], vol[0])
	}
	// ramp's close is strictly increasing, so OBV should be monotonically
	// non-decreasing (every bar adds volume).
	for i := 1; i < len(out); i++ {
		if math.IsNaN(out[i]) {
			t.Fatalf("OBV[%d] should not be NaN", i)
		}
		if out[i] <= out[i-1] {
			t.Fatalf("OBV[%d]=%v should be > OBV[%d]=%v (monotonic uptrend)", i, out[i], i-1, out[i-1])
		}
	}
}

func TestOBVComposesWithNaNPrefixedInput(t *testing.T) {
	// close carries leading NaN (e.g. output of another indicator); OBV should
	// skip the NaN prefix and pad the same amount back onto the output.
	c := []float64{math.NaN(), math.NaN(), 1, 2, 3, 2}
	vol := []float64{10, 20, 30, 40, 50, 60}
	out := OBV(c, vol)
	if len(out) != len(c) {
		t.Fatalf("len=%d want %d", len(out), len(c))
	}
	if !math.IsNaN(out[0]) || !math.IsNaN(out[1]) {
		t.Fatalf("out[0:2]=%v want NaN prefix", out[:2])
	}
	if math.IsNaN(out[2]) {
		t.Fatal("out[2] should be a value")
	}
	if out[2] != 30 {
		t.Fatalf("out[2]=%v want 30 (seeded with vol[2])", out[2])
	}
}
