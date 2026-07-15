package lib

// BarsSince returns the number of bars since cond was last true, evaluated at the
// final element: 0 if cond ends true, 1 if only the previous bar was true, and so
// on. Returns len(cond) when cond is never true (backtesting.py's barssince uses a
// +inf "never" sentinel; we clamp to the series length). Call it in Next with a
// boolean series built up to the current bar.
func BarsSince(cond []bool) int {
	for i := len(cond) - 1; i >= 0; i-- {
		if cond[i] {
			return len(cond) - 1 - i
		}
	}
	return len(cond)
}
