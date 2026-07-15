package report

import (
	"errors"
	"fmt"
	"html/template"
	"math"
	"os"
	"sort"

	backtest "github.com/florinel-chis/gobacktest"
)

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// heatmapRamp is a validated 13-step sequential blue ramp, light (low) to
// dark (high). Do not reorder — cell text contrast is chosen against it.
var heatmapRamp = []string{
	"#cde2fb", "#b7d3f6", "#9ec5f4", "#86b6ef", "#6da7ec",
	"#5598e7", "#3987e5", "#2a78d6", "#256abf", "#1c5cab",
	"#184f95", "#104281", "#0d366b",
}

const (
	heatmapNABG      = "#e6e6e3"
	heatmapNAFG      = "#6b6b68"
	heatmapBestRing  = "#eb6834"
	heatmapTextLight = "#0b0b0b" // used on the lighter half of the ramp
	heatmapTextDark  = "#ffffff" // used on the darker half of the ramp
)

// Heatmap writes a self-contained, offline HTML heatmap of a 2-parameter
// Optimize result to path. The two parameters form the axes; cell color
// encodes the maximize metric (sequential, low→high); out-of-money combos
// (Value == -math.MaxFloat64, or NaN) render as neutral "N/A"; the best
// combo is ringed. Returns an error if the result has other than exactly 2
// distinct parameters.
func Heatmap(opt *backtest.OptimizeResult, path string, opts ...Option) error {
	if opt == nil {
		return errors.New("report: Heatmap: nil OptimizeResult")
	}

	o := Options{Title: "Optimize Heatmap", MetricName: "value"}
	for _, fn := range opts {
		fn(&o)
	}
	if o.MetricName == "" {
		o.MetricName = "value"
	}

	td, err := buildHeatmap(opt, o)
	if err != nil {
		return err
	}

	tmpl, err := template.New("heatmap").Parse(heatmapTmplText)
	if err != nil {
		return fmt.Errorf("report: parse heatmap template: %w", err)
	}

	f, err := os.Create(path) // #nosec G304 -- output path is a caller-supplied API argument
	if err != nil {
		return fmt.Errorf("report: create file: %w", err)
	}
	defer f.Close()

	if err := tmpl.Execute(f, td); err != nil {
		return fmt.Errorf("report: execute heatmap template: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Internal template schema
// ---------------------------------------------------------------------------

// heatmapCell is one grid cell.
type heatmapCell struct {
	Text  string
	Style template.CSS
	Title string
	Best  bool
}

// heatmapRow is one row of cells with its row-axis label.
type heatmapRow struct {
	Label string
	Cells []heatmapCell
}

// heatmapTemplateData is the data passed to heatmapTmplText.
type heatmapTemplateData struct {
	Title       string
	MetricLabel string
	KeyA        string
	KeyB        string
	ColLabels   []string
	Rows        []heatmapRow
	LegendSteps []string
	MinLabel    string
	MaxLabel    string
	BestSummary string
}

// ---------------------------------------------------------------------------
// buildHeatmap
// ---------------------------------------------------------------------------

func buildHeatmap(opt *backtest.OptimizeResult, o Options) (heatmapTemplateData, error) {
	sampleParams := opt.Best
	if len(opt.Heatmap) > 0 {
		sampleParams = opt.Heatmap[0].Params
	}
	if len(sampleParams) != 2 {
		return heatmapTemplateData{}, fmt.Errorf("report: Heatmap requires exactly 2 parameters, got %d", len(sampleParams))
	}

	keys := make([]string, 0, 2)
	for k := range sampleParams {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	keyA, keyB := keys[0], keys[1]

	rowSet := map[string]any{}
	colSet := map[string]any{}
	type cellKey struct{ a, b string }
	entries := make(map[cellKey]*backtest.HeatmapEntry, len(opt.Heatmap))

	for i := range opt.Heatmap {
		e := &opt.Heatmap[i]
		if len(e.Params) != 2 {
			return heatmapTemplateData{}, fmt.Errorf("report: Heatmap requires exactly 2 parameters, got %d", len(e.Params))
		}
		va, okA := e.Params[keyA]
		vb, okB := e.Params[keyB]
		if !okA || !okB {
			return heatmapTemplateData{}, fmt.Errorf("report: Heatmap: combo missing key %q or %q", keyA, keyB)
		}
		rowSet[fmtParamValue(va)] = va
		colSet[fmtParamValue(vb)] = vb
		entries[cellKey{fmtParamValue(va), fmtParamValue(vb)}] = e
	}

	rowVals := distinctSortedParams(rowSet)
	colVals := distinctSortedParams(colSet)

	// Min/max over finite, non-OOM values only.
	min, max := math.Inf(1), math.Inf(-1)
	hasFinite := false
	for i := range opt.Heatmap {
		v := opt.Heatmap[i].Value
		if math.IsNaN(v) || v == -math.MaxFloat64 {
			continue
		}
		hasFinite = true
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	if !hasFinite {
		min, max = 0, 0
	}

	var bestA, bestB string
	if opt.Best != nil {
		bestA = fmtParamValue(opt.Best[keyA])
		bestB = fmtParamValue(opt.Best[keyB])
	}

	rows := make([]heatmapRow, 0, len(rowVals))
	for _, rv := range rowVals {
		rvKey := fmtParamValue(rv)
		row := heatmapRow{Label: rvKey}
		for _, cv := range colVals {
			cvKey := fmtParamValue(cv)
			isBest := opt.Best != nil && rvKey == bestA && cvKey == bestB

			e, ok := entries[cellKey{rvKey, cvKey}]
			var cell heatmapCell
			if !ok || math.IsNaN(e.Value) || e.Value == -math.MaxFloat64 {
				cell = heatmapCell{
					Text:  "N/A",
					Style: template.CSS(fmt.Sprintf("background:%s;color:%s", heatmapNABG, heatmapNAFG)), // #nosec G203 -- CSS built only from hardcoded color constants; no user input
					Title: fmt.Sprintf("%s=%s, %s=%s, %s=N/A", keyA, rvKey, keyB, cvKey, o.MetricName),
					Best:  isBest,
				}
			} else {
				bg, fg := rampColor(e.Value, min, max)
				cell = heatmapCell{
					Text:  fmt.Sprintf("%.3g", e.Value),
					Style: template.CSS(fmt.Sprintf("background:%s;color:%s", bg, fg)), // #nosec G203 -- CSS built only from hardcoded ramp color constants; no user input
					Title: fmt.Sprintf("%s=%s, %s=%s, %s=%.6g", keyA, rvKey, keyB, cvKey, o.MetricName, e.Value),
					Best:  isBest,
				}
			}
			row.Cells = append(row.Cells, cell)
		}
		rows = append(rows, row)
	}

	colLabels := make([]string, len(colVals))
	for i, cv := range colVals {
		colLabels[i] = fmtParamValue(cv)
	}

	bestSummary := "n/a"
	if opt.Best != nil {
		bestSummary = fmt.Sprintf("%s=%s, %s=%s, %s=%.6g", keyA, bestA, keyB, bestB, o.MetricName, opt.BestValue)
	}

	return heatmapTemplateData{
		Title:       o.Title,
		MetricLabel: o.MetricName,
		KeyA:        keyA,
		KeyB:        keyB,
		ColLabels:   colLabels,
		Rows:        rows,
		LegendSteps: append([]string(nil), heatmapRamp...),
		MinLabel:    fmt.Sprintf("%.3g", min),
		MaxLabel:    fmt.Sprintf("%.3g", max),
		BestSummary: bestSummary,
	}, nil
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

// rampColor maps v into [min,max] onto heatmapRamp and returns the background
// hex color and a legible foreground text color for that step.
func rampColor(v, min, max float64) (bg, fg string) {
	t := 0.5
	if max > min {
		t = (v - min) / (max - min)
	}
	if t < 0 {
		t = 0
	}
	if t > 1 {
		t = 1
	}
	idx := int(t*float64(len(heatmapRamp)-1) + 0.5)
	if idx < 0 {
		idx = 0
	}
	if idx >= len(heatmapRamp) {
		idx = len(heatmapRamp) - 1
	}
	bg = heatmapRamp[idx]
	if t >= 0.5 {
		fg = heatmapTextDark
	} else {
		fg = heatmapTextLight
	}
	return bg, fg
}

// toFloat converts common numeric parameter types to float64.
func toFloat(v any) (float64, bool) {
	switch t := v.(type) {
	case int:
		return float64(t), true
	case int8:
		return float64(t), true
	case int16:
		return float64(t), true
	case int32:
		return float64(t), true
	case int64:
		return float64(t), true
	case uint:
		return float64(t), true
	case uint8:
		return float64(t), true
	case uint16:
		return float64(t), true
	case uint32:
		return float64(t), true
	case uint64:
		return float64(t), true
	case float32:
		return float64(t), true
	case float64:
		return t, true
	default:
		return 0, false
	}
}

// fmtParamValue formats a parameter value for axis labels and dedup keys.
// Integral floats render without a decimal point.
func fmtParamValue(v any) string {
	switch t := v.(type) {
	case float64:
		if t == math.Trunc(t) && !math.IsInf(t, 0) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%g", t)
	case float32:
		return fmtParamValue(float64(t))
	default:
		return fmt.Sprint(v)
	}
}

// distinctSortedParams dedupes a value set (keyed by fmtParamValue) and sorts
// numerically when possible, falling back to a string sort otherwise.
func distinctSortedParams(set map[string]any) []any {
	out := make([]any, 0, len(set))
	for _, v := range set {
		out = append(out, v)
	}
	sort.Slice(out, func(i, j int) bool {
		fi, oki := toFloat(out[i])
		fj, okj := toFloat(out[j])
		if oki && okj {
			return fi < fj
		}
		return fmtParamValue(out[i]) < fmtParamValue(out[j])
	})
	return out
}

// ---------------------------------------------------------------------------
// Template
// ---------------------------------------------------------------------------

const heatmapTmplText = `<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<link rel="icon" href="data:,">
<title>{{.Title}} — gobacktest</title>
<style>
*, *::before, *::after { box-sizing: border-box; margin: 0; padding: 0; }
html, body {
  min-height: 100%;
  background: #fafafa;
  color: #222;
  font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, Helvetica, Arial, sans-serif;
}
.wrap { max-width: 960px; margin: 0 auto; padding: 24px 16px 48px; }
h1 { font-size: 1.1rem; font-weight: 700; letter-spacing: .02em; margin-bottom: 4px; }
.subtitle { color: #666; font-size: .82rem; margin-bottom: 20px; }
.best { color: #b0501f; font-weight: 600; }
table.heatmap { border-collapse: collapse; margin-bottom: 20px; }
table.heatmap th, table.heatmap td {
  text-align: center;
  padding: 8px 12px;
  font-size: .82rem;
  font-variant-numeric: tabular-nums;
  border: 1px solid rgba(0,0,0,.08);
}
table.heatmap th { background: transparent; color: #555; font-weight: 600; }
table.heatmap th.corner { border: none; background: none; }
table.heatmap th.row-axis {
  text-align: right;
  color: #555;
  font-weight: 600;
}
table.heatmap td.cell { cursor: default; }
table.heatmap td.cell.is-best {
  outline: 2px solid ` + heatmapBestRing + `;
  outline-offset: -2px;
  box-shadow: 0 0 0 2px ` + heatmapBestRing + ` inset;
  font-weight: 700;
}
.axis-label { font-size: .78rem; color: #666; margin: 4px 0 10px; }
.legend { display: flex; align-items: center; gap: 8px; font-size: .78rem; color: #555; }
.legend .swatches { display: flex; }
.legend .swatch { width: 18px; height: 14px; }
.legend .swatch:first-child { border-radius: 3px 0 0 3px; }
.legend .swatch:last-child { border-radius: 0 3px 3px 0; }
.legend .na-swatch {
  width: 18px; height: 14px; border-radius: 3px;
  background: ` + heatmapNABG + `;
  margin-left: 12px;
}
.na-label { margin-left: 4px; }

@media (prefers-color-scheme: dark) {
  html, body { background: #1a1a19; color: #fff; }
  .subtitle, .axis-label, .legend, table.heatmap th { color: #b8b8b5; }
  table.heatmap th, table.heatmap td { border-color: rgba(255,255,255,.12); }
  .best { color: #ff9d5c; }
}
</style>
</head>
<body>
<div class="wrap">
  <h1>{{.Title}}</h1>
  <div class="subtitle">Optimize heatmap — metric: <strong>{{.MetricLabel}}</strong> — best: <span class="best">{{.BestSummary}}</span></div>

  <div class="axis-label">columns: {{.KeyB}} &nbsp;·&nbsp; rows: {{.KeyA}}</div>
  <table class="heatmap">
    <thead>
      <tr>
        <th class="corner"></th>
        {{range .ColLabels}}<th>{{.}}</th>{{end}}
      </tr>
    </thead>
    <tbody>
      {{range .Rows}}
      <tr>
        <th class="row-axis">{{.Label}}</th>
        {{range .Cells}}<td class="cell{{if .Best}} is-best{{end}}" style="{{.Style}}" title="{{.Title}}">{{.Text}}</td>{{end}}
      </tr>
      {{end}}
    </tbody>
  </table>

  <div class="legend">
    <span>{{.MinLabel}}</span>
    <span class="swatches">{{range .LegendSteps}}<span class="swatch" style="background:{{.}}"></span>{{end}}</span>
    <span>{{.MaxLabel}}</span>
    <span class="na-swatch"></span><span class="na-label">N/A (out of money)</span>
  </div>
</div>
</body>
</html>
`
