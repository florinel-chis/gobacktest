// Package report generates self-contained HTML backtest reports embedding
// TradingView Lightweight Charts v5 (vendored) via go:embed.
// It is stdlib-only: no third-party Go dependencies.
package report

import (
	"encoding/json"
	"fmt"
	"html/template"
	"math"
	"os"
	"time"

	backtest "github.com/florinel-chis/gobacktest"
)

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// Options holds report configuration.
type Options struct {
	Title string
	// MetricName labels the optimize metric shown by Heatmap's legend and
	// hover text (e.g. "SQN"). Unused by Generate.
	MetricName string
}

// Option is a functional option for Generate and Heatmap.
type Option func(*Options)

// Title sets the report title.
func Title(s string) Option { return func(o *Options) { o.Title = s } }

// Metric labels the optimize metric shown on a Heatmap's legend and cell
// hover text (e.g. "SQN", "Sharpe Ratio"). Defaults to "value" when unset.
func Metric(name string) Option { return func(o *Options) { o.MetricName = name } }

// Generate marshals a backtest run into a self-contained HTML file at path.
// The file embeds TradingView Lightweight Charts v5 (vendored) and the data
// JSON inline, so it works offline with no network access.
func Generate(data *backtest.Data, result *backtest.Result, stats backtest.Stats, path string, opts ...Option) error {
	o := Options{Title: "Backtest Report"}
	for _, fn := range opts {
		fn(&o)
	}

	rd := buildReport(data, result, stats, o)
	jsonBytes, err := json.Marshal(rd)
	if err != nil {
		return fmt.Errorf("report: marshal JSON: %w", err)
	}

	tmpl, err := template.New("report").Parse(tmplText)
	if err != nil {
		return fmt.Errorf("report: parse template: %w", err)
	}

	f, err := os.Create(path) // #nosec G304 -- output path is a caller-supplied API argument
	if err != nil {
		return fmt.Errorf("report: create file: %w", err)
	}
	defer f.Close()

	data2 := struct {
		Title    string
		LWCJS    template.JS
		DataJSON template.JS
	}{
		Title:    o.Title,
		LWCJS:    template.JS(lwcJS),     // #nosec G203 -- vendored Lightweight Charts JS we control, embedded verbatim
		DataJSON: template.JS(jsonBytes), // #nosec G203 -- our own json.Marshal'd backtest data, not user HTML
	}
	if err := tmpl.Execute(f, data2); err != nil {
		return fmt.Errorf("report: execute template: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal JSON schema
// ---------------------------------------------------------------------------

// reportData is the JSON payload embedded in the HTML report.
type reportData struct {
	Title      string        `json:"title"`
	Intraday   bool          `json:"intraday"`
	OHLC       []ohlc        `json:"ohlc"`
	Volume     []vp          `json:"volume"`
	Equity     []vp          `json:"equity"`
	Drawdown   []vp          `json:"drawdown"`
	Indicators []indSeries   `json:"indicators"`
	Trades     []tradeMarker `json:"trades"`
	Stats      []statRow     `json:"stats"`
}

// timeKey values are what Lightweight Charts accepts as series time: a
// "yyyy-mm-dd" string for daily-or-coarser bars, or an int64 Unix timestamp
// for intraday bars (date-only keys would collapse all same-day bars onto a
// single point).
type timeKey = any

// ohlc is one candlestick bar.
type ohlc struct {
	Time  timeKey `json:"time"`
	Open  float64 `json:"open"`
	High  float64 `json:"high"`
	Low   float64 `json:"low"`
	Close float64 `json:"close"`
}

// vp is a value-point used for line / histogram series.
// Color is omitted when empty (volume bars are colored per-bar).
type vp struct {
	Time  timeKey `json:"time"`
	Value float64 `json:"value"`
	Color string  `json:"color,omitempty"`
}

// indSeries is one indicator exported to the report.
type indSeries struct {
	Name    string `json:"name"`
	Overlay bool   `json:"overlay"`
	Color   string `json:"color"`
	Data    []vp   `json:"data"`
}

// tradeMarker carries the entry/exit info for one closed trade.
type tradeMarker struct {
	EntryTime  timeKey `json:"entryTime"`
	ExitTime   timeKey `json:"exitTime"`
	EntryPrice float64 `json:"entryPrice"`
	ExitPrice  float64 `json:"exitPrice"`
	PL         float64 `json:"pl"`
	IsLong     bool    `json:"isLong"`
}

// statRow is one row in the statistics panel.
type statRow struct {
	Label string `json:"label"`
	Value string `json:"value"`
}

// fmtStat formats a stat metric value with format, rendering NaN as "N/A"
// (a degenerate/undefined ratio, e.g. Sharpe when volatility is 0) instead
// of Go's default "NaN" text — consistent with the heatmap's N/A cells.
func fmtStat(v float64, format string) string {
	if math.IsNaN(v) {
		return "N/A"
	}
	return fmt.Sprintf(format, v)
}

// ---------------------------------------------------------------------------
// buildReport
// ---------------------------------------------------------------------------

// palette is assigned to indicators that have no Color set.
var palette = []string{
	"#2962FF", "#ff6d00", "#00bcd4", "#e91e63", "#4caf50",
	"#9c27b0", "#ff9800", "#03a9f4", "#f44336", "#8bc34a",
}

func buildReport(data *backtest.Data, result *backtest.Result, stats backtest.Stats, opts Options) reportData {
	bars := data.Bars()

	// Sub-daily bars need Unix-timestamp time keys — with date-only keys every
	// bar of one session would share the same key and collapse in the chart.
	intraday := false
	for i := 1; i < len(bars); i++ {
		if bars[i].Time.Sub(bars[i-1].Time) < 24*time.Hour {
			intraday = true
			break
		}
	}
	tkey := func(t time.Time) timeKey {
		if intraday {
			return t.Unix()
		}
		return t.Format("2006-01-02")
	}

	// OHLC + Volume
	ohlcSlice := make([]ohlc, len(bars))
	volSlice := make([]vp, len(bars))
	for i, b := range bars {
		ts := tkey(b.Time)
		ohlcSlice[i] = ohlc{
			Time:  ts,
			Open:  b.Open,
			High:  b.High,
			Low:   b.Low,
			Close: b.Close,
		}
		color := "#ef5350" // red (down)
		if b.Close >= b.Open {
			color = "#26a69a" // green (up)
		}
		volSlice[i] = vp{Time: ts, Value: b.Volume, Color: color}
	}

	// Equity curve.
	eqSlice := make([]vp, len(result.EquityCurve))
	for i, p := range result.EquityCurve {
		eqSlice[i] = vp{Time: tkey(p.Time), Value: p.Equity}
	}

	// Drawdown (computed from equity curve; value <= 0 %).
	ddSlice := make([]vp, len(result.EquityCurve))
	runMax := 0.0
	for i, p := range result.EquityCurve {
		if p.Equity > runMax {
			runMax = p.Equity
		}
		dd := 0.0
		if runMax > 0 {
			dd = (p.Equity/runMax - 1) * 100
		}
		ddSlice[i] = vp{Time: tkey(p.Time), Value: dd}
	}

	// Indicators.
	indSlices := make([]indSeries, 0, len(result.Indicators))
	paletteIdx := 0
	for _, ind := range result.Indicators {
		// Skip all-NaN indicators.
		hasValue := false
		for _, v := range ind.Values {
			if !math.IsNaN(v) {
				hasValue = true
				break
			}
		}
		if !hasValue {
			continue
		}

		color := ind.Color
		if color == "" {
			color = palette[paletteIdx%len(palette)]
			paletteIdx++
		}

		pts := make([]vp, 0, len(bars))
		for i, b := range bars {
			if i >= len(ind.Values) {
				break
			}
			v := ind.Values[i]
			if math.IsNaN(v) {
				continue
			}
			pts = append(pts, vp{Time: tkey(b.Time), Value: v})
		}

		indSlices = append(indSlices, indSeries{
			Name:    ind.Name,
			Overlay: ind.Overlay,
			Color:   color,
			Data:    pts,
		})
	}

	// Trades.
	tradeSlice := make([]tradeMarker, 0, len(result.Trades))
	for _, t := range result.Trades {
		tm := tradeMarker{
			EntryPrice: t.EntryPrice,
			ExitPrice:  t.ExitPrice,
			PL:         t.PL,
			IsLong:     t.Size > 0,
		}
		if !t.EntryTime.IsZero() {
			tm.EntryTime = tkey(t.EntryTime)
		}
		if !t.ExitTime.IsZero() {
			tm.ExitTime = tkey(t.ExitTime)
		}
		tradeSlice = append(tradeSlice, tm)
	}

	// Stats rows (ordered, mirroring Stats.String() field order).
	fmtDur := func(d time.Duration) string {
		days := int(d.Hours() / 24)
		return fmt.Sprintf("%d days", days)
	}
	statsRows := []statRow{
		{"Start", stats.Start.Format("2006-01-02")},
		{"End", stats.End.Format("2006-01-02")},
		{"Duration", fmtDur(stats.Duration)},
		{"Exposure Time [%]", fmtStat(stats.ExposureTimePct, "%.2f")},
		{"Equity Final [$]", fmtStat(stats.EquityFinal, "%.2f")},
		{"Equity Peak [$]", fmtStat(stats.EquityPeak, "%.2f")},
		{"Return [%]", fmtStat(stats.ReturnPct, "%.2f")},
		{"Buy & Hold Return [%]", fmtStat(stats.BuyHoldReturnPct, "%.2f")},
		{"Return (Ann.) [%]", fmtStat(stats.ReturnAnnPct, "%.2f")},
		{"Volatility (Ann.) [%]", fmtStat(stats.VolatilityAnnPct, "%.2f")},
		{"CAGR [%]", fmtStat(stats.CAGRPct, "%.2f")},
		{"Sharpe Ratio", fmtStat(stats.SharpeRatio, "%.3f")},
		{"Sortino Ratio", fmtStat(stats.SortinoRatio, "%.3f")},
		{"Calmar Ratio", fmtStat(stats.CalmarRatio, "%.3f")},
		{"Alpha [%]", fmtStat(stats.AlphaPct, "%.2f")},
		{"Beta", fmtStat(stats.Beta, "%.4f")},
		{"Max. Drawdown [%]", fmtStat(stats.MaxDrawdownPct, "%.2f")},
		{"Avg. Drawdown [%]", fmtStat(stats.AvgDrawdownPct, "%.2f")},
		{"Max. Drawdown Duration", fmtDur(stats.MaxDrawdownDur)},
		{"Avg. Drawdown Duration", fmtDur(stats.AvgDrawdownDur)},
		{"# Trades", fmt.Sprintf("%d", stats.NumTrades)},
		{"Win Rate [%]", fmtStat(stats.WinRatePct, "%.2f")},
		{"Best Trade [%]", fmtStat(stats.BestTradePct, "%.2f")},
		{"Worst Trade [%]", fmtStat(stats.WorstTradePct, "%.2f")},
		{"Avg. Trade [%]", fmtStat(stats.AvgTradePct, "%.2f")},
		{"Max. Trade Duration", fmtDur(stats.MaxTradeDur)},
		{"Avg. Trade Duration", fmtDur(stats.AvgTradeDur)},
		{"Profit Factor", fmtStat(stats.ProfitFactor, "%.3f")},
		{"Expectancy [%]", fmtStat(stats.ExpectancyPct, "%.2f")},
		{"SQN", fmtStat(stats.SQN, "%.3f")},
		{"Kelly Criterion", fmtStat(stats.KellyCriterion, "%.4f")},
	}
	if stats.CommissionsTotal != 0 {
		statsRows = append(statsRows, statRow{"Commissions [$]", fmtStat(stats.CommissionsTotal, "%.2f")})
	}

	return reportData{
		Title:      opts.Title,
		Intraday:   intraday,
		OHLC:       ohlcSlice,
		Volume:     volSlice,
		Equity:     eqSlice,
		Drawdown:   ddSlice,
		Indicators: indSlices,
		Trades:     tradeSlice,
		Stats:      statsRows,
	}
}
