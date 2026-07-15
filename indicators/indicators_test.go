package indicators

import (
	"math"
	"testing"
)

func TestSMA(t *testing.T) {
	in := []float64{1, 2, 3, 4, 5}
	out := SMA(in, 3)
	if len(out) != 5 {
		t.Fatalf("len=%d want 5", len(out))
	}
	// warmup: first period-1 = 2 are NaN
	if !math.IsNaN(out[0]) || !math.IsNaN(out[1]) {
		t.Fatalf("warmup not NaN: %v", out[:2])
	}
	// SMA(3) at idx2 = (1+2+3)/3 = 2; idx4 = (3+4+5)/3 = 4
	if math.Abs(out[2]-2) > 1e-9 || math.Abs(out[4]-4) > 1e-9 {
		t.Fatalf("SMA values wrong: %v", out)
	}
}

func TestEMAAcceptsNaNPrefixedInput(t *testing.T) {
	// composition: EMA of a series that already has leading NaN must skip the NaN
	in := []float64{math.NaN(), math.NaN(), 1, 2, 3, 4, 5, 6}
	out := EMA(in, 3)
	if len(out) != len(in) {
		t.Fatalf("len mismatch")
	}
	// first non-NaN input is at idx2; EMA(3) lookback 2 → first value at idx2+2 = 4
	for i := 0; i < 4; i++ {
		if !math.IsNaN(out[i]) {
			t.Fatalf("out[%d]=%v want NaN", i, out[i])
		}
	}
	if math.IsNaN(out[4]) {
		t.Fatal("out[4] should be a value, got NaN")
	}
	if math.Abs(out[4]-2.0) > 1e-9 {
		t.Fatalf("out[4]=%v want 2.0 (EMA seed = SMA of first 3)", out[4])
	}
}
