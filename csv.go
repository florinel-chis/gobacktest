package backtest

import (
	"encoding/csv"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// FromCSV loads OHLCV from a CSV with a header row. Required columns
// (case-insensitive): Date, Open, High, Low, Close, Volume. Dates are parsed as
// "2006-01-02" or RFC3339.
func FromCSV(path string) (*Data, error) {
	f, err := os.Open(path) // #nosec G304 -- input path is a caller-supplied API argument
	if err != nil {
		return nil, err
	}
	defer f.Close()

	r := csv.NewReader(f)
	rows, err := r.ReadAll()
	if err != nil {
		return nil, err
	}
	if len(rows) < 2 {
		return nil, fmt.Errorf("backtest: CSV %s has no data rows", path)
	}

	idx := map[string]int{}
	for i, h := range rows[0] {
		idx[strings.ToLower(strings.TrimSpace(h))] = i
	}
	for _, req := range []string{"date", "open", "high", "low", "close", "volume"} {
		if _, ok := idx[req]; !ok {
			return nil, fmt.Errorf("backtest: CSV %s missing column %q", path, req)
		}
	}

	n := len(rows) - 1
	times := make([]time.Time, n)
	o := make([]float64, n)
	h := make([]float64, n)
	l := make([]float64, n)
	c := make([]float64, n)
	v := make([]float64, n)

	parseTime := func(s string) (time.Time, error) {
		s = strings.TrimSpace(s)
		if t, err := time.Parse("2006-01-02", s); err == nil {
			return t, nil
		}
		return time.Parse(time.RFC3339, s)
	}

	numCols := [5]string{"open", "high", "low", "close", "volume"}

	for i, row := range rows[1:] {
		t, err := parseTime(row[idx["date"]])
		if err != nil {
			return nil, fmt.Errorf("backtest: CSV %s row %d date: %w", path, i+2, err)
		}
		times[i] = t
		dsts := [5]*float64{&o[i], &h[i], &l[i], &c[i], &v[i]}
		for k, name := range numCols {
			val, err := strconv.ParseFloat(strings.TrimSpace(row[idx[name]]), 64)
			if err != nil {
				return nil, fmt.Errorf("backtest: CSV %s row %d %s: %w", path, i+2, name, err)
			}
			*dsts[k] = val
		}
	}
	return FromOHLCV(times, o, h, l, c, v)
}
