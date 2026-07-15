package source_test

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"

	backtest "github.com/florinel-chis/gobacktest"
	"github.com/florinel-chis/gobacktest/source"
)

func TestIntervalConstants(t *testing.T) {
	want := map[source.Interval]string{
		source.M1:  "1m",
		source.M2:  "2m",
		source.M5:  "5m",
		source.M10: "10m",
		source.M15: "15m",
		source.M30: "30m",
		source.H1:  "1h",
		source.H4:  "4h",
		source.D1:  "1d",
		source.W1:  "1wk",
		source.Mo1: "1mo",
	}
	for iv, s := range want {
		if string(iv) != s {
			t.Errorf("interval %q: want %q", iv, s)
		}
	}
}

func TestErrUnsupportedIntervalSentinel(t *testing.T) {
	wrapped := fmt.Errorf("someprovider: %w: %q", source.ErrUnsupportedInterval, source.H4)
	if !errors.Is(wrapped, source.ErrUnsupportedInterval) {
		t.Fatal("wrapped error should match ErrUnsupportedInterval via errors.Is")
	}
}

// fakeSource proves the interface shape compiles for an implementer.
type fakeSource struct{}

func (fakeSource) Fetch(_ context.Context, _ string, _, _ time.Time, _ source.Interval) (*backtest.Data, error) {
	return backtest.FromBars(nil), nil
}

func TestSourceInterface(t *testing.T) {
	var s source.Source = fakeSource{}
	d, err := s.Fetch(context.Background(), "X", time.Time{}, time.Time{}, source.D1)
	if err != nil || d == nil {
		t.Fatalf("Fetch: d=%v err=%v", d, err)
	}
}
