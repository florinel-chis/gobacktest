// Package source defines a provider-agnostic contract for fetching OHLCV
// market data. Concrete providers (yahoo, oanda) implement Source; strategies
// and demos depend only on this interface, so providers are interchangeable.
package source

import (
	"context"
	"errors"
	"time"

	backtest "github.com/florinel-chis/gobacktest"
)

// Interval is a canonical bar interval shared by all providers. Providers map
// it to their native granularity strings and return an error wrapping
// ErrUnsupportedInterval for intervals they cannot serve.
type Interval string

// Canonical intervals.
const (
	M1  Interval = "1m"
	M2  Interval = "2m"
	M5  Interval = "5m"
	M10 Interval = "10m"
	M15 Interval = "15m"
	M30 Interval = "30m"
	H1  Interval = "1h"
	H4  Interval = "4h"
	D1  Interval = "1d"
	W1  Interval = "1wk"
	Mo1 Interval = "1mo"
)

// ErrUnsupportedInterval is the sentinel wrapped by providers when asked for
// an interval they do not support. Match with errors.Is.
var ErrUnsupportedInterval = errors.New("unsupported interval")

// Source fetches OHLCV bars for a symbol in [start, end).
type Source interface {
	Fetch(ctx context.Context, symbol string, start, end time.Time, interval Interval) (*backtest.Data, error)
}
