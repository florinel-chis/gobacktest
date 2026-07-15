package backtest

import (
	"fmt"
	"time"
)

// Bar is a single OHLCV candle (array-of-structs convenience for callers).
type Bar struct {
	Time                           time.Time
	Open, High, Low, Close, Volume float64
}

// Data holds OHLCV as typed columns (struct-of-arrays). The current-bar view
// length n is advanced by the engine via setLen; accessors return zero-copy
// subslices up to n.
// NOTE: update clone() when adding fields.
type Data struct {
	time                           []time.Time
	open, high, low, close, volume []float64
	extra                          map[string]Series
	n                              int
}

func (d *Data) fullLen() int { return len(d.close) }
func (d *Data) Len() int     { return d.n }
func (d *Data) setLen(n int) {
	if n < 0 || n > d.fullLen() {
		panic(fmt.Sprintf("backtest: setLen(%d) out of range [0,%d]", n, d.fullLen()))
	}
	d.n = n
}
func (d *Data) Time() []time.Time { return d.time[:d.n] }
func (d *Data) Open() Series      { return d.open[:d.n] }
func (d *Data) High() Series      { return d.high[:d.n] }
func (d *Data) Low() Series       { return d.low[:d.n] }
func (d *Data) Close() Series     { return d.close[:d.n] }
func (d *Data) Volume() Series    { return d.volume[:d.n] }

// AddColumn attaches a named float64 column (e.g. an indicator) aligned to bars.
func (d *Data) AddColumn(name string, vals []float64) error {
	if len(vals) != d.fullLen() {
		return fmt.Errorf("backtest: column %q length %d != data length %d", name, len(vals), d.fullLen())
	}
	if d.extra == nil {
		d.extra = make(map[string]Series)
	}
	d.extra[name] = Series(vals)
	return nil
}

// Column returns a named column sliced to the current view.
func (d *Data) Column(name string) (Series, bool) {
	s, ok := d.extra[name]
	if !ok {
		return nil, false
	}
	return s[:d.n], true
}

// timeAt returns the timestamp at absolute bar index i (full series).
func (d *Data) timeAt(i int) time.Time { return d.time[i] }

func (d *Data) openAt(i int) float64  { return d.open[i] }
func (d *Data) highAt(i int) float64  { return d.high[i] }
func (d *Data) lowAt(i int) float64   { return d.low[i] }
func (d *Data) closeAt(i int) float64 { return d.close[i] }

// TimeAt returns the timestamp at absolute bar index i (full series, exported).
func (d *Data) TimeAt(i int) time.Time { return d.time[i] }

// FullLen returns the total number of bars in the dataset (unaffected by the
// current bar-view length used during a run).
func (d *Data) FullLen() int { return d.fullLen() }

// clone returns a shallow copy sharing the immutable OHLCV/time slices but with
// an independent view length and a fresh indicator map — so parallel runs (e.g.
// Optimize) never share mutable state. The OHLCV slices are read-only.
func (d *Data) clone() *Data {
	return &Data{
		time:   d.time,
		open:   d.open,
		high:   d.high,
		low:    d.low,
		close:  d.close,
		volume: d.volume,
		extra:  make(map[string]Series),
		n:      d.fullLen(),
	}
}

// Bars returns all OHLCV bars as a slice of Bar (full dataset, not view-limited).
// Use this outside a run to access the complete data for report generation.
func (d *Data) Bars() []Bar {
	n := d.fullLen()
	out := make([]Bar, n)
	for i := range out {
		out[i] = Bar{
			Time:   d.time[i],
			Open:   d.open[i],
			High:   d.high[i],
			Low:    d.low[i],
			Close:  d.close[i],
			Volume: d.volume[i],
		}
	}
	return out
}
