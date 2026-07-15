package backtest

import (
	"testing"
	"time"
)

func newTestData() *Data {
	t0 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	times := []time.Time{t0, t0.AddDate(0, 0, 1), t0.AddDate(0, 0, 2)}
	d, err := FromOHLCV(times,
		[]float64{10, 11, 12}, // open
		[]float64{13, 14, 15}, // high
		[]float64{9, 10, 11},  // low
		[]float64{12, 13, 14}, // close
		[]float64{100, 200, 300})
	if err != nil {
		panic(err)
	}
	return d
}

func TestDataAccessors(t *testing.T) {
	d := newTestData()
	if d.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", d.Len())
	}
	if d.Close().Last() != 14 {
		t.Fatalf("Close().Last() = %v, want 14", d.Close().Last())
	}
	if d.High().At(1) != 14 {
		t.Fatalf("High().At(1) = %v, want 14", d.High().At(1))
	}
}

func TestDataSetLenGivesZeroCopyView(t *testing.T) {
	d := newTestData()
	d.setLen(2)
	if d.Len() != 2 {
		t.Fatalf("Len() = %d, want 2", d.Len())
	}
	if d.Close().Last() != 13 {
		t.Fatalf("Close().Last() = %v, want 13 after setLen(2)", d.Close().Last())
	}
	if d.Close().Len() != 2 {
		t.Fatalf("Close().Len() = %d, want 2", d.Close().Len())
	}
}

func TestDataAddColumn(t *testing.T) {
	d := newTestData()
	if err := d.AddColumn("sma", []float64{1, 2, 3}); err != nil {
		t.Fatalf("AddColumn error: %v", err)
	}
	col, ok := d.Column("sma")
	if !ok || col.Last() != 3 {
		t.Fatalf("Column(sma) = %v, %v; want 3, true", col, ok)
	}
	// wrong length is rejected
	if err := d.AddColumn("bad", []float64{1, 2}); err == nil {
		t.Fatal("AddColumn with wrong length should error")
	}
}

func TestFromBars(t *testing.T) {
	t0 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	bars := []Bar{
		{Time: t0, Open: 10, High: 13, Low: 9, Close: 12, Volume: 100},
		{Time: t0.AddDate(0, 0, 1), Open: 11, High: 14, Low: 10, Close: 13, Volume: 200},
	}
	d := FromBars(bars)
	if d.Len() != 2 || d.Close().Last() != 13 {
		t.Fatalf("FromBars: Len=%d Close.Last=%v; want 2, 13", d.Len(), d.Close().Last())
	}
}

func TestFromOHLCVLengthMismatch(t *testing.T) {
	_, err := FromOHLCV(
		[]time.Time{time.Now()},
		[]float64{1, 2}, // mismatched
		[]float64{1}, []float64{1}, []float64{1}, []float64{1})
	if err == nil {
		t.Fatal("FromOHLCV with mismatched lengths should error")
	}
}

func TestDataSetLenOutOfRangePanics(t *testing.T) {
	d := newTestData() // 3 bars
	for _, n := range []int{-1, 4} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatalf("setLen(%d) should panic", n)
				}
			}()
			d.setLen(n)
		}()
	}
}
