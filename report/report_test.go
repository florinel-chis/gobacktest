package report

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	backtest "github.com/florinel-chis/gobacktest"
)

func TestBuildReport(t *testing.T) {
	d, _ := backtest.FromOHLCV(
		[]time.Time{time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)},
		[]float64{10, 11}, []float64{12, 13}, []float64{9, 10}, []float64{11, 12}, []float64{100, 200})
	res := &backtest.Result{
		StartBar:    0,
		EquityCurve: []backtest.EquityPoint{{Time: d.TimeAt(0), Equity: 1000}, {Time: d.TimeAt(1), Equity: 1010}},
		FinalEquity: 1010,
	}
	st := backtest.Stats{NumTrades: 0, ReturnPct: 1.0}
	rd := buildReport(d, res, st, Options{Title: "X"})
	if len(rd.OHLC) != 2 || rd.OHLC[0].Time != "2024-01-02" || rd.OHLC[1].Close != 12 {
		t.Fatalf("ohlc wrong: %+v", rd.OHLC)
	}
	if len(rd.Equity) != 2 || rd.Equity[1].Value != 1010 {
		t.Fatalf("equity wrong: %+v", rd.Equity)
	}
	// drawdown[1] from peak 1010 -> 0; drawdown is <= 0
	if rd.Drawdown[1].Value > 0 {
		t.Fatalf("drawdown should be <=0: %v", rd.Drawdown[1].Value)
	}
	if len(rd.Stats) == 0 {
		t.Fatal("stats rows empty")
	}
}

// TestBuildReportVolumeColor verifies that down-bars get red and up-bars get green volume colors.
func TestBuildReportVolumeColor(t *testing.T) {
	// bar 0: close(9) < open(10) → red; bar 1: close(13) >= open(11) → green
	d, _ := backtest.FromOHLCV(
		[]time.Time{time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)},
		[]float64{10, 11}, []float64{12, 13}, []float64{8, 10}, []float64{9, 13}, []float64{100, 200})
	res := &backtest.Result{
		EquityCurve: []backtest.EquityPoint{{Time: d.TimeAt(0), Equity: 1000}, {Time: d.TimeAt(1), Equity: 1010}},
		FinalEquity: 1010,
	}
	rd := buildReport(d, res, backtest.Stats{}, Options{Title: "vol-color"})
	if len(rd.Volume) != 2 {
		t.Fatalf("expected 2 volume bars, got %d", len(rd.Volume))
	}
	if rd.Volume[0].Color != "#ef5350" {
		t.Errorf("down-bar color: want #ef5350, got %q", rd.Volume[0].Color)
	}
	if rd.Volume[1].Color != "#26a69a" {
		t.Errorf("up-bar color: want #26a69a, got %q", rd.Volume[1].Color)
	}
}

// TestBuildReportTradeMarkers verifies that a single closed long trade maps to one
// tradeMarker with the correct entry/exit times and IsLong=true.
func TestBuildReportTradeMarkers(t *testing.T) {
	t0 := time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)
	d, _ := backtest.FromOHLCV(
		[]time.Time{t0, t1},
		[]float64{10, 11}, []float64{12, 13}, []float64{9, 10}, []float64{11, 12}, []float64{100, 200})
	res := &backtest.Result{
		EquityCurve: []backtest.EquityPoint{{Time: t0, Equity: 1000}, {Time: t1, Equity: 1050}},
		FinalEquity: 1050,
		Trades: []backtest.Trade{
			{
				Size:       10, // positive → long
				EntryPrice: 10,
				ExitPrice:  11,
				EntryTime:  t0,
				ExitTime:   t1,
				PL:         10,
			},
		},
	}
	rd := buildReport(d, res, backtest.Stats{}, Options{Title: "trade-marker"})
	if len(rd.Trades) != 1 {
		t.Fatalf("expected 1 trade marker, got %d", len(rd.Trades))
	}
	tm := rd.Trades[0]
	if !tm.IsLong {
		t.Error("expected IsLong=true for positive Size trade")
	}
	if tm.EntryTime != "2024-01-02" {
		t.Errorf("EntryTime: want 2024-01-02, got %q", tm.EntryTime)
	}
	if tm.ExitTime != "2024-01-03" {
		t.Errorf("ExitTime: want 2024-01-03, got %q", tm.ExitTime)
	}
}

// TestBuildReportAllNaNIndicatorSkipped verifies that an IndicatorSeries whose
// Values are all NaN contributes no data points to the report.
func TestBuildReportAllNaNIndicatorSkipped(t *testing.T) {
	d, _ := backtest.FromOHLCV(
		[]time.Time{time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)},
		[]float64{10, 11}, []float64{12, 13}, []float64{9, 10}, []float64{11, 12}, []float64{100, 200})
	res := &backtest.Result{
		EquityCurve: []backtest.EquityPoint{{Time: d.TimeAt(0), Equity: 1000}, {Time: d.TimeAt(1), Equity: 1010}},
		FinalEquity: 1010,
		Indicators: []backtest.IndicatorSeries{
			{Name: "AllNaN", Overlay: true, Color: "#ff0000", Values: []float64{math.NaN(), math.NaN()}},
		},
	}
	rd := buildReport(d, res, backtest.Stats{}, Options{Title: "nan-ind"})
	if len(rd.Indicators) != 0 {
		t.Errorf("all-NaN indicator should be skipped; got %d indicator(s)", len(rd.Indicators))
	}
}

// TestBuildReportIntradayTimestamps verifies that intraday bars keep distinct
// time keys. Date-only formatting would collapse all same-day bars onto one
// point, so sub-daily data must be emitted as Unix timestamps (which
// Lightweight Charts accepts natively) and flagged so the time axis shows
// clock times.
func TestBuildReportIntradayTimestamps(t *testing.T) {
	t0 := time.Date(2024, 1, 2, 9, 0, 0, 0, time.UTC)
	times := []time.Time{t0, t0.Add(time.Hour), t0.Add(2 * time.Hour)}
	d, _ := backtest.FromOHLCV(times,
		[]float64{10, 11, 12}, []float64{12, 13, 14}, []float64{9, 10, 11},
		[]float64{11, 12, 13}, []float64{100, 200, 300})
	res := &backtest.Result{
		EquityCurve: []backtest.EquityPoint{
			{Time: times[0], Equity: 1000}, {Time: times[1], Equity: 1010}, {Time: times[2], Equity: 1020},
		},
		FinalEquity: 1020,
		Trades: []backtest.Trade{
			{Size: 1, EntryPrice: 10, ExitPrice: 13, PL: 3, EntryTime: times[0], ExitTime: times[2]},
		},
	}
	rd := buildReport(d, res, backtest.Stats{}, Options{Title: "intraday"})
	if !rd.Intraday {
		t.Fatal("Intraday flag should be true for hourly bars")
	}
	seen := map[any]bool{}
	for _, o := range rd.OHLC {
		if seen[o.Time] {
			t.Fatalf("duplicate time key %v — intraday bars collapsed", o.Time)
		}
		seen[o.Time] = true
		if _, ok := o.Time.(int64); !ok {
			t.Fatalf("intraday time should be a Unix timestamp, got %T (%v)", o.Time, o.Time)
		}
	}
	if rd.Trades[0].EntryTime != any(times[0].Unix()) || rd.Trades[0].ExitTime != any(times[2].Unix()) {
		t.Fatalf("trade marker times not Unix timestamps: %+v", rd.Trades[0])
	}

	// Daily data keeps the yyyy-mm-dd string keys and Intraday=false.
	daily, _ := backtest.FromOHLCV(
		[]time.Time{time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)},
		[]float64{10, 11}, []float64{12, 13}, []float64{9, 10}, []float64{11, 12}, []float64{100, 200})
	dres := &backtest.Result{
		EquityCurve: []backtest.EquityPoint{{Time: daily.TimeAt(0), Equity: 1000}, {Time: daily.TimeAt(1), Equity: 1010}},
		FinalEquity: 1010,
	}
	drd := buildReport(daily, dres, backtest.Stats{}, Options{Title: "daily"})
	if drd.Intraday {
		t.Fatal("Intraday flag should be false for daily bars")
	}
	if drd.OHLC[0].Time != "2024-01-02" {
		t.Fatalf("daily time key = %v, want 2024-01-02", drd.OHLC[0].Time)
	}
}

func TestGenerateWritesSelfContainedHTML(t *testing.T) {
	d, _ := backtest.FromOHLCV(
		[]time.Time{time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)},
		[]float64{10, 11}, []float64{12, 13}, []float64{9, 10}, []float64{11, 12}, []float64{100, 200})
	res := &backtest.Result{
		StartBar:    0,
		EquityCurve: []backtest.EquityPoint{{Time: d.TimeAt(0), Equity: 1000}, {Time: d.TimeAt(1), Equity: 1010}},
		FinalEquity: 1010,
	}
	st := backtest.Stats{NumTrades: 0, ReturnPct: 1.0}

	p := filepath.Join(t.TempDir(), "report.html")
	if err := Generate(d, res, st, p, Title("Demo")); err != nil {
		t.Fatal(err)
	}
	b, _ := os.ReadFile(p)
	html := string(b)
	for _, want := range []string{"LightweightCharts", "createChart", "\"ohlc\"", "id=\"chart\"", "Demo"} {
		if !strings.Contains(html, want) {
			t.Fatalf("report missing %q", want)
		}
	}
	if len(b) < 150_000 { // embedded JS makes it large
		t.Fatalf("report too small (%d bytes) — JS not embedded?", len(b))
	}
}

func TestBuildReportIndicatorsOverlayAndSubpane(t *testing.T) {
	d, _ := backtest.FromOHLCV(
		[]time.Time{time.Date(2024, 1, 2, 0, 0, 0, 0, time.UTC), time.Date(2024, 1, 3, 0, 0, 0, 0, time.UTC)},
		[]float64{10, 11}, []float64{12, 13}, []float64{9, 10}, []float64{11, 12}, []float64{100, 200})
	res := &backtest.Result{
		StartBar:    0,
		EquityCurve: []backtest.EquityPoint{{Time: d.TimeAt(0), Equity: 1000}, {Time: d.TimeAt(1), Equity: 1010}},
		FinalEquity: 1010,
		Indicators: []backtest.IndicatorSeries{
			{Name: "SMA", Overlay: true, Color: "#2962FF", Values: []float64{10, 11}},
			{Name: "RSI", Overlay: false, Color: "#e91e63", Values: []float64{math.NaN(), 55}},
		},
	}
	rd := buildReport(d, res, backtest.Stats{}, Options{Title: "X"})
	if len(rd.Indicators) != 2 {
		t.Fatalf("indicators=%d want 2 (overlay + subpane)", len(rd.Indicators))
	}
	var sma, rsi *indSeries
	for i := range rd.Indicators {
		switch rd.Indicators[i].Name {
		case "SMA":
			sma = &rd.Indicators[i]
		case "RSI":
			rsi = &rd.Indicators[i]
		}
	}
	if sma == nil || !sma.Overlay {
		t.Fatalf("SMA should be overlay: %+v", sma)
	}
	if rsi == nil || rsi.Overlay {
		t.Fatalf("RSI should be non-overlay (subpane): %+v", rsi)
	}
	// RSI's leading NaN bar is skipped -> 1 data point.
	if len(rsi.Data) != 1 || rsi.Data[0].Value != 55 {
		t.Fatalf("RSI data should skip NaN warmup -> [{,55}], got %+v", rsi.Data)
	}
}
