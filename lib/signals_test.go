package lib

import "testing"

func TestBarsSince(t *testing.T) {
	cases := []struct {
		name string
		cond []bool
		want int
	}{
		{"ends true", []bool{false, true, false, true}, 0},
		{"one bar ago", []bool{false, true, false}, 1},
		{"three ago", []bool{true, false, false, false}, 3},
		{"never", []bool{false, false, false}, 3},
		{"empty", []bool{}, 0},
		{"all true", []bool{true, true, true}, 0},
	}
	for _, c := range cases {
		if got := BarsSince(c.cond); got != c.want {
			t.Errorf("%s: BarsSince=%d want %d", c.name, got, c.want)
		}
	}
}
