package report

import (
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"

	backtest "github.com/florinel-chis/gobacktest"
)

// buildTestOptimizeResult constructs a small 2-param (n1 x n2) heatmap with one
// out-of-money cell and a known best combo.
func buildTestOptimizeResult() *backtest.OptimizeResult {
	best := backtest.Params{"n1": 10, "n2": 30}
	return &backtest.OptimizeResult{
		Best:      best,
		BestValue: 2.5,
		BestStats: backtest.Stats{SQN: 2.5},
		Heatmap: []backtest.HeatmapEntry{
			{Params: backtest.Params{"n1": 5, "n2": 20}, Value: 0.5},
			{Params: backtest.Params{"n1": 5, "n2": 30}, Value: -math.MaxFloat64}, // OOM
			{Params: backtest.Params{"n1": 10, "n2": 20}, Value: 1.2},
			{Params: backtest.Params{"n1": 10, "n2": 30}, Value: 2.5}, // best
		},
	}
}

func TestHeatmapGenerates(t *testing.T) {
	opt := buildTestOptimizeResult()
	dir := t.TempDir()
	path := filepath.Join(dir, "heatmap.html")

	if err := Heatmap(opt, path, Metric("SQN"), Title("n1 x n2")); err != nil {
		t.Fatalf("Heatmap: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("stat output: %v", err)
	}
	if info.Size() <= 1024 {
		t.Fatalf("expected output >1KB, got %d bytes", info.Size())
	}

	raw, err := os.ReadFile(path) // #nosec G304 -- test-controlled temp path
	if err != nil {
		t.Fatalf("read output: %v", err)
	}
	html := string(raw)

	if !strings.Contains(html, "<table") {
		t.Error("expected a <table> element")
	}
	if !strings.Contains(html, "n1") || !strings.Contains(html, "n2") {
		t.Error("expected param key names n1 and n2 in output")
	}
	if !strings.Contains(html, "N/A") {
		t.Error("expected N/A for the out-of-money cell")
	}
	if !strings.Contains(html, "2.5") {
		t.Error("expected the best value 2.5 to appear")
	}
	if !strings.Contains(html, "SQN") {
		t.Error("expected the metric label SQN to appear")
	}
}

func TestHeatmapRejectsNon2Param(t *testing.T) {
	dir := t.TempDir()

	// 1 param.
	opt1 := &backtest.OptimizeResult{
		Best:      backtest.Params{"n1": 10},
		BestValue: 1.0,
		Heatmap: []backtest.HeatmapEntry{
			{Params: backtest.Params{"n1": 5}, Value: 0.5},
			{Params: backtest.Params{"n1": 10}, Value: 1.0},
		},
	}
	if err := Heatmap(opt1, filepath.Join(dir, "one.html")); err == nil {
		t.Error("expected error for 1-parameter OptimizeResult")
	}

	// 3 params.
	opt3 := &backtest.OptimizeResult{
		Best:      backtest.Params{"n1": 10, "n2": 20, "n3": 1},
		BestValue: 1.0,
		Heatmap: []backtest.HeatmapEntry{
			{Params: backtest.Params{"n1": 5, "n2": 20, "n3": 1}, Value: 0.5},
			{Params: backtest.Params{"n1": 10, "n2": 20, "n3": 1}, Value: 1.0},
		},
	}
	if err := Heatmap(opt3, filepath.Join(dir, "three.html")); err == nil {
		t.Error("expected error for 3-parameter OptimizeResult")
	}
}
