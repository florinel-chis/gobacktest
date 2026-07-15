package backtest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFromCSV(t *testing.T) {
	csv := "Date,Open,High,Low,Close,Volume\n" +
		"2024-01-02,185.05,186.33,181.83,183.56,82488700\n" +
		"2024-01-03,182.15,183.79,181.37,182.18,58414500\n"
	dir := t.TempDir()
	p := filepath.Join(dir, "x.csv")
	if err := os.WriteFile(p, []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := FromCSV(p)
	if err != nil {
		t.Fatalf("FromCSV error: %v", err)
	}
	if d.Len() != 2 {
		t.Fatalf("Len = %d, want 2", d.Len())
	}
	if d.Close().Last() != 182.18 {
		t.Fatalf("Close.Last = %v, want 182.18", d.Close().Last())
	}
	if d.Volume().At(1) != 82488700 {
		t.Fatalf("Volume.At(1) = %v, want 82488700", d.Volume().At(1))
	}
	if y := d.Time()[0].Year(); y != 2024 {
		t.Fatalf("Time[0].Year = %d, want 2024", y)
	}
}

func TestFromCSVMissingColumn(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.csv")
	os.WriteFile(p, []byte("Date,Open,High,Low\n2024-01-02,1,2,3\n"), 0o644)
	if _, err := FromCSV(p); err == nil {
		t.Fatal("FromCSV missing Close/Volume should error")
	}
}

func TestFromCSVRFC3339Date(t *testing.T) {
	csv := "Date,Open,High,Low,Close,Volume\n" +
		"2024-01-02T00:00:00Z,1,2,0.5,1.5,100\n"
	dir := t.TempDir()
	p := filepath.Join(dir, "rfc.csv")
	if err := os.WriteFile(p, []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}
	d, err := FromCSV(p)
	if err != nil {
		t.Fatalf("FromCSV RFC3339 error: %v", err)
	}
	if d.Len() != 1 || d.Time()[0].Year() != 2024 {
		t.Fatalf("RFC3339 parse failed: len=%d year=%d", d.Len(), d.Time()[0].Year())
	}
}

func TestFromCSVMalformedNumber(t *testing.T) {
	csv := "Date,Open,High,Low,Close,Volume\n2024-01-02,1,2,0.5,notanumber,100\n"
	dir := t.TempDir()
	p := filepath.Join(dir, "bad.csv")
	if err := os.WriteFile(p, []byte(csv), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := FromCSV(p)
	if err == nil {
		t.Fatal("FromCSV should error on non-numeric close")
	}
	if !strings.Contains(err.Error(), "close") {
		t.Fatalf("error should name the column; got: %v", err)
	}
}
