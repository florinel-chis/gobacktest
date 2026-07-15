package indicators

import (
	"math"
	"testing"
)

func ramp(n int) (h, l, c []float64) {
	h = make([]float64, n)
	l = make([]float64, n)
	c = make([]float64, n)
	for i := 0; i < n; i++ {
		p := 100.0 + float64(i)
		h[i], l[i], c[i] = p+1, p-1, p
	}
	return
}

func TestWilliamsR(t *testing.T) {
	h, l, c := ramp(20)
	wr := WilliamsR(h, l, c, 14)
	// warmup: first 13 NaN (period-1)
	for i := 0; i < 13; i++ {
		if !math.IsNaN(wr[i]) {
			t.Fatalf("wr[%d]=%v want NaN", i, wr[i])
		}
	}
	if math.IsNaN(wr[13]) {
		t.Fatal("wr[13] should be a value")
	}
	// Williams %R is in [-100, 0]
	if wr[19] < -100 || wr[19] > 0 {
		t.Fatalf("wr[19]=%v out of [-100,0]", wr[19])
	}
}

func TestEMAOfWilliamsR(t *testing.T) {
	// the user's "EMA of Will R": compose without leaking the WillR warmup.
	h, l, c := ramp(40)
	wr := WilliamsR(h, l, c, 14) // 13 NaN warmup
	smoothed := EMA(wr, 9)       // EMA strips the 13 NaN, adds 8 more
	// total warmup = 13 + 8 = 21 leading NaN
	for i := 0; i < 21; i++ {
		if !math.IsNaN(smoothed[i]) {
			t.Fatalf("smoothed[%d]=%v want NaN (warmup)", i, smoothed[i])
		}
	}
	if math.IsNaN(smoothed[21]) {
		t.Fatal("smoothed[21] should be a value")
	}
	if math.IsNaN(smoothed[39]) {
		t.Fatal("smoothed[39] should be a value")
	}
}

func TestCCIWarmupAndFinite(t *testing.T) {
	h, l, c := ramp(30)
	period := 14
	out := CCI(h, l, c, period)
	if len(out) != 30 {
		t.Fatalf("len=%d want 30", len(out))
	}
	// CCI lookback = period-1 = 13
	lookback := period - 1
	for i := 0; i < lookback; i++ {
		if !math.IsNaN(out[i]) {
			t.Fatalf("CCI warmup out[%d]=%v want NaN", i, out[i])
		}
	}
	if math.IsNaN(out[lookback]) {
		t.Fatal("CCI[lookback] should be a value (post-warmup)")
	}
	for i := lookback; i < 30; i++ {
		if math.IsNaN(out[i]) || math.IsInf(out[i], 0) {
			t.Fatalf("CCI[%d]=%v must be finite", i, out[i])
		}
	}
	// short-input guard: too few bars -> all NaN, no panic
	sh, sl, sc := ramp(5)
	so := CCI(sh, sl, sc, 14)
	for i := range so {
		if !math.IsNaN(so[i]) {
			t.Fatalf("short CCI should be all NaN at %d", i)
		}
	}
}

func TestStochastic(t *testing.T) {
	h, l, c := ramp(40)
	fastK, slowK, slowD := 14, 3, 3
	k, d := Stochastic(h, l, c, fastK, slowK, slowD)
	if len(k) != 40 || len(d) != 40 {
		t.Fatalf("len k=%d d=%d want 40", len(k), len(d))
	}
	lookback := (fastK - 1) + (slowK - 1) + (slowD - 1) // 13+2+2 = 17
	for i := 0; i < lookback; i++ {
		if !math.IsNaN(k[i]) || !math.IsNaN(d[i]) {
			t.Fatalf("warmup index %d not NaN: k=%v d=%v", i, k[i], d[i])
		}
	}
	if math.IsNaN(k[lookback]) || math.IsNaN(d[lookback]) {
		t.Fatalf("post-warmup index %d should be valid (k=%v d=%v)", lookback, k[lookback], d[lookback])
	}
	for i := lookback; i < 40; i++ {
		if k[i] < 0 || k[i] > 100 || d[i] < 0 || d[i] > 100 {
			t.Fatalf("stoch out of [0,100] at %d: k=%v d=%v", i, k[i], d[i])
		}
	}
	// short-input guard: too few bars -> all NaN, no panic
	sh, sl, sc := ramp(5)
	sk, sd := Stochastic(sh, sl, sc, 14, 3, 3)
	for i := range sk {
		if !math.IsNaN(sk[i]) || !math.IsNaN(sd[i]) {
			t.Fatalf("short input should be all NaN at %d", i)
		}
	}
}
