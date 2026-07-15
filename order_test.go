package backtest

import "testing"

func TestOrderDirection(t *testing.T) {
	long := &order{size: 0.5}
	short := &order{size: -0.5}
	if !long.isLong() || long.isShort() {
		t.Fatal("size>0 must be long")
	}
	if !short.isShort() || short.isLong() {
		t.Fatal("size<0 must be short")
	}
}
