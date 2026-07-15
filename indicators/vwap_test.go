package indicators

import (
	"math"
	"testing"
)

func TestVWAP(t *testing.T) {
	high := []float64{10, 12, 14}
	low := []float64{8, 10, 12}
	cl := []float64{9, 11, 13} // typical = 9, 11, 13
	vol := []float64{100, 200, 300}
	out := VWAP(high, low, cl, vol)
	if len(out) != 3 {
		t.Fatalf("len=%d want 3", len(out))
	}
	// bar0: 9; bar1: (9*100+11*200)/300 = 3100/300; bar2: (9*100+11*200+13*300)/600 = 7000/600
	wants := []float64{9, 3100.0 / 300, 7000.0 / 600}
	for i, w := range wants {
		if math.Abs(out[i]-w) > 1e-9 {
			t.Errorf("VWAP[%d]=%v want %v", i, out[i], w)
		}
	}
}

func TestVWAPZeroVolumeNaN(t *testing.T) {
	out := VWAP([]float64{10}, []float64{10}, []float64{10}, []float64{0})
	if !math.IsNaN(out[0]) {
		t.Errorf("zero cumulative volume should yield NaN, got %v", out[0])
	}
}
