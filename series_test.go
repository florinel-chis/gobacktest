package backtest

import "testing"

func TestSeries(t *testing.T) {
	s := Series{10, 20, 30, 40}
	if got := s.Last(); got != 40 {
		t.Fatalf("Last() = %v, want 40", got)
	}
	if got := s.At(0); got != 40 {
		t.Fatalf("At(0) = %v, want 40", got)
	}
	if got := s.At(2); got != 20 {
		t.Fatalf("At(2) = %v, want 20", got)
	}
	if got := s.Len(); got != 4 {
		t.Fatalf("Len() = %v, want 4", got)
	}
}

func TestSeriesSubsliceIsZeroCopy(t *testing.T) {
	full := Series{1, 2, 3, 4, 5}
	view := full[:3]
	if view.Last() != 3 {
		t.Fatalf("view.Last() = %v, want 3", view.Last())
	}
	// mutating the backing array is visible through the view (proves no copy)
	full[2] = 99
	if view.Last() != 99 {
		t.Fatalf("view.Last() = %v, want 99 (zero-copy)", view.Last())
	}
}
