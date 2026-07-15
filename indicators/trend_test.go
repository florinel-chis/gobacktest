package indicators

import (
	"math"
	"testing"
)

func TestRSIWarmupAndRange(t *testing.T) {
	in := make([]float64, 30)
	for i := range in {
		in[i] = 100 + math.Sin(float64(i)) // non-monotonic so RSI isn't pinned
	}
	out := RSI(in, 14)
	// RSI lookback = period = 14 → first 14 NaN
	for i := 0; i < 14; i++ {
		if !math.IsNaN(out[i]) {
			t.Fatalf("RSI warmup out[%d]=%v want NaN", i, out[i])
		}
	}
	if math.IsNaN(out[14]) {
		t.Fatal("RSI[14] should be a value")
	}
	if out[14] < 0 || out[14] > 100 {
		t.Fatalf("RSI[14]=%v out of [0,100]", out[14])
	}
}

func TestATRWarmupAndRange(t *testing.T) {
	h, l, c := ramp(30)
	period := 14 // ATR lookback = period
	out := ATR(h, l, c, period)
	for i := 0; i < period; i++ {
		if !math.IsNaN(out[i]) {
			t.Fatalf("ATR warmup out[%d]=%v want NaN", i, out[i])
		}
	}
	if math.IsNaN(out[period]) {
		t.Fatal("ATR[period] should be a value (post-warmup)")
	}
	for i := period; i < 30; i++ {
		if out[i] < 0 {
			t.Fatalf("ATR[%d]=%v must be >= 0", i, out[i])
		}
	}
	// short-input guard: too few bars -> all NaN, no panic
	sh, sl, sc := ramp(5)
	so := ATR(sh, sl, sc, 14)
	for i := range so {
		if !math.IsNaN(so[i]) {
			t.Fatalf("short ATR should be all NaN at %d", i)
		}
	}
}

func TestMACDHistConsistency(t *testing.T) {
	in := make([]float64, 60)
	for i := range in {
		in[i] = 100 + float64(i) + 5*math.Sin(float64(i)/3)
	}
	macd, signal, hist := MACD(in, 12, 26, 9)
	if len(macd) != 60 || len(signal) != 60 || len(hist) != 60 {
		t.Fatal("length mismatch")
	}
	// MACD(12,26,9) lookback = slow+signal-2 = 33; index 33 must be defined.
	if math.IsNaN(macd[33]) || math.IsNaN(signal[33]) || math.IsNaN(hist[33]) {
		t.Fatal("MACD outputs must be defined at index 33 (post-warmup)")
	}
	for i := 0; i < 33; i++ {
		if !math.IsNaN(hist[i]) {
			t.Fatalf("MACD warmup hist[%d]=%v want NaN", i, hist[i])
		}
	}
	// where all three are defined, hist == macd - signal
	for i := range hist {
		if !math.IsNaN(hist[i]) {
			if math.Abs(hist[i]-(macd[i]-signal[i])) > 1e-6 {
				t.Fatalf("hist[%d] != macd-signal", i)
			}
		}
	}
}

func TestBollingerOrdering(t *testing.T) {
	in := make([]float64, 30)
	for i := range in {
		in[i] = 100 + 3*math.Sin(float64(i)/2)
	}
	up, mid, lo := Bollinger(in, 20, 2.0)
	for i := 0; i < 19; i++ {
		if !math.IsNaN(mid[i]) {
			t.Fatalf("Bollinger warmup mid[%d]=%v want NaN", i, mid[i])
		}
	}
	if math.IsNaN(up[19]) || math.IsNaN(mid[19]) || math.IsNaN(lo[19]) {
		t.Fatal("Bollinger must be defined at index 19 (period-1)")
	}
	for i := 19; i < 30; i++ {
		if up[i] < mid[i] || mid[i] < lo[i] {
			t.Fatalf("bollinger ordering at %d: %v %v %v", i, up[i], mid[i], lo[i])
		}
	}
}

// TestWMAWarmupAndValue checks WMA warmup = period-1 and a known value.
func TestWMAWarmupAndValue(t *testing.T) {
	in := []float64{1, 2, 3, 4, 5, 6, 7}
	out := WMA(in, 3)
	if len(out) != 7 {
		t.Fatalf("len=%d want 7", len(out))
	}
	// WMA warmup = period-1 = 2 → first 2 NaN
	for i := 0; i < 2; i++ {
		if !math.IsNaN(out[i]) {
			t.Fatalf("WMA warmup out[%d]=%v want NaN", i, out[i])
		}
	}
	if math.IsNaN(out[2]) {
		t.Fatal("WMA[2] should be a value")
	}
	// WMA(3) at idx2: weights 1,2,3 → (1*1 + 2*2 + 3*3)/(1+2+3) = 14/6 ≈ 2.333
	want := (1.0*1 + 2.0*2 + 3.0*3) / (1 + 2 + 3)
	if math.Abs(out[2]-want) > 1e-9 {
		t.Fatalf("WMA[2]=%v want %v", out[2], want)
	}
}

// TestROCWarmupAndValue checks ROC warmup = period and a known value.
func TestROCWarmupAndValue(t *testing.T) {
	in := []float64{10, 11, 12, 13, 14, 15, 16}
	out := ROC(in, 3)
	if len(out) != 7 {
		t.Fatalf("len=%d want 7", len(out))
	}
	// ROC warmup = period = 3 → first 3 NaN
	for i := 0; i < 3; i++ {
		if !math.IsNaN(out[i]) {
			t.Fatalf("ROC warmup out[%d]=%v want NaN", i, out[i])
		}
	}
	if math.IsNaN(out[3]) {
		t.Fatal("ROC[3] should be a value")
	}
	// ROC(3) at idx3 = (13-10)/10 * 100 = 30.0
	want := (13.0 - 10.0) / 10.0 * 100.0
	if math.Abs(out[3]-want) > 1e-9 {
		t.Fatalf("ROC[3]=%v want %v", out[3], want)
	}
}

func TestADXWarmupAndRange(t *testing.T) {
	h, l, c := ramp(40)
	period := 14
	out := ADX(h, l, c, period)
	if len(out) != 40 {
		t.Fatalf("len=%d want 40", len(out))
	}
	// ADX is double-smoothed: lookback = 2*period-1 = 27
	lookback := 2*period - 1
	for i := 0; i < lookback; i++ {
		if !math.IsNaN(out[i]) {
			t.Fatalf("ADX warmup out[%d]=%v want NaN", i, out[i])
		}
	}
	if math.IsNaN(out[lookback]) {
		t.Fatal("ADX[lookback] should be a value (post-warmup)")
	}
	for i := lookback; i < 40; i++ {
		if out[i] < 0 || out[i] > 100 {
			t.Fatalf("ADX[%d]=%v out of [0,100]", i, out[i])
		}
	}
	// short-input guard: too few bars -> all NaN, no panic
	sh, sl, sc := ramp(5)
	so := ADX(sh, sl, sc, 14)
	for i := range so {
		if !math.IsNaN(so[i]) {
			t.Fatalf("short ADX should be all NaN at %d", i)
		}
	}
}

func TestDonchianOrderingAndWarmup(t *testing.T) {
	h, l, c := ramp(30)
	_ = c
	period := 10
	upper, mid, lower := Donchian(h, l, period)
	if len(upper) != 30 || len(mid) != 30 || len(lower) != 30 {
		t.Fatal("length mismatch")
	}
	lookback := period - 1
	for i := 0; i < lookback; i++ {
		if !math.IsNaN(upper[i]) || !math.IsNaN(mid[i]) || !math.IsNaN(lower[i]) {
			t.Fatalf("Donchian warmup at %d not NaN: %v %v %v", i, upper[i], mid[i], lower[i])
		}
	}
	if math.IsNaN(upper[lookback]) || math.IsNaN(mid[lookback]) || math.IsNaN(lower[lookback]) {
		t.Fatal("Donchian must be defined at index period-1")
	}
	for i := lookback; i < 30; i++ {
		if upper[i] < mid[i] || mid[i] < lower[i] {
			t.Fatalf("donchian ordering at %d: upper=%v mid=%v lower=%v", i, upper[i], mid[i], lower[i])
		}
	}
}

func TestDonchianShortInput(t *testing.T) {
	h, l, _ := ramp(5) // < lookback 9 for period 10
	upper, mid, lower := Donchian(h, l, 10)
	for i := range upper {
		if !math.IsNaN(upper[i]) || !math.IsNaN(mid[i]) || !math.IsNaN(lower[i]) {
			t.Fatalf("short-input Donchian must be all NaN at %d", i)
		}
	}
}

func TestMACDShortInput(t *testing.T) {
	in := make([]float64, 10) // < lookback 33
	for i := range in {
		in[i] = float64(i)
	}
	macd, sig, hist := MACD(in, 12, 26, 9)
	for i := range macd {
		if !math.IsNaN(macd[i]) || !math.IsNaN(sig[i]) || !math.IsNaN(hist[i]) {
			t.Fatalf("short-input MACD must be all NaN at %d", i)
		}
	}
}

func TestBollingerShortInput(t *testing.T) {
	in := make([]float64, 5) // < lookback 19
	for i := range in {
		in[i] = float64(i)
	}
	up, mid, lo := Bollinger(in, 20, 2.0)
	for i := range up {
		if !math.IsNaN(up[i]) || !math.IsNaN(mid[i]) || !math.IsNaN(lo[i]) {
			t.Fatalf("short-input Bollinger must be all NaN at %d", i)
		}
	}
}
