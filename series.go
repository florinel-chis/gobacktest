package backtest

// Series is a float64 column aligned to bars. The "current bar" is the last
// element; a per-bar view is a zero-copy subslice (s[:n]).
type Series []float64

// Last returns the most recent value (the current bar).
// It panics if s is empty.
func (s Series) Last() float64 { return s[len(s)-1] }

// At returns the value i bars back from the end. At(0) == Last().
// It panics if i is out of range (i<0 or i>=Len()).
func (s Series) At(i int) float64 { return s[len(s)-1-i] }

// Len returns the number of values in the series.
func (s Series) Len() int { return len(s) }
