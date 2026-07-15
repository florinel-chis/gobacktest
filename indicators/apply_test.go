package indicators

import (
	"math"
	"testing"
)

func TestApplyNegativeLookbackAllNaN(t *testing.T) {
	// EMA(s, 0) passes lookback -1 into apply; invalid input must yield a
	// full-length all-NaN slice (like a too-short input), never a panic.
	in := []float64{1, 2, 3, 4, 5}
	out := EMA(in, 0)
	if len(out) != len(in) {
		t.Fatalf("len=%d want %d", len(out), len(in))
	}
	for i, v := range out {
		if !math.IsNaN(v) {
			t.Fatalf("out[%d]=%v want NaN (invalid lookback)", i, v)
		}
	}
}

func TestApplyHLCNegativeLookbackAllNaN(t *testing.T) {
	h := []float64{1, 2, 3}
	l := []float64{0, 1, 2}
	c := []float64{0.5, 1.5, 2.5}
	out := applyHLC(h, l, c, -1, func(hh, ll, cc []float64) []float64 {
		return make([]float64, len(cc))
	})
	if len(out) != len(c) {
		t.Fatalf("len=%d want %d", len(out), len(c))
	}
	for i, v := range out {
		if !math.IsNaN(v) {
			t.Fatalf("out[%d]=%v want NaN (invalid lookback)", i, v)
		}
	}
}

func TestApplyHLC(t *testing.T) {
	h := []float64{1, 2, 3, 4}
	l := []float64{0, 1, 2, 3}
	c := []float64{0.5, 1.5, 2.5, 3.5}
	// fn emulates a talib HLC indicator with lookback 1 (zero-fills index 0):
	got := applyHLC(h, l, c, 1, func(hh, ll, cc []float64) []float64 {
		out := make([]float64, len(cc))
		for i := 1; i < len(cc); i++ {
			out[i] = hh[i] - ll[i] // = 1 for all
		}
		return out
	})
	if !math.IsNaN(got[0]) {
		t.Fatalf("got[0]=%v want NaN (warmup)", got[0])
	}
	if got[1] != 1 || got[2] != 1 || got[3] != 1 {
		t.Fatalf("applyHLC values=%v want [NaN,1,1,1]", got)
	}
}
