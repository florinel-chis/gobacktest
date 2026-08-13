package strategies

import backtest "github.com/florinel-chis/gobacktest"

// SignalStrategy backtests an externally-computed per-bar signal. It goes long
// when Signal[bar] > 0 and exits Hold bars later at market. The decision of WHEN
// to enter — a moving-average cross, an oversold reading, or a machine-learning
// forecast such as a candlestick foundation model — lives entirely in how the
// Signal slice is built offline; the engine only executes and scores it. This
// makes it the bridge for testing any external predictor as a trading rule: emit
// one number per bar, run it here, read the stats against buy-and-hold.
//
// Point-in-time discipline is the caller's responsibility. Signal[i] must be
// computable from information available at the close of bar i only (no future
// bars). Because an order enqueued in Next fills on the following bar, a signal
// known at the close of bar i enters at bar i+1, so a correctly-built signal has
// no lookahead.
//
// Signal length must equal the data length. Zero values: Hold 1, Size 1.
type SignalStrategy struct {
	Signal []float64 // per-bar entry trigger; a value > 0 opens a long
	Hold   int       // bars to hold each trade (coerced to >= 1)
	Size   float64   // fraction of equity in (0,1], or absolute units >= 1

	sig   *backtest.Indicator
	entry int
	inpos bool
}

// Init registers the signal as a data column and seeds defaults.
func (s *SignalStrategy) Init(st *backtest.State) {
	if s.Hold < 1 {
		s.Hold = 1
	}
	if s.Size == 0 {
		s.Size = 1
	}
	s.sig = st.I("signalStrategy", func() []float64 { return s.Signal })
	s.entry = -1
	s.inpos = false
}

// Next opens a long on a positive signal and closes it Hold bars later. Only one
// position is held at a time; signals arriving while in a trade are ignored.
func (s *SignalStrategy) Next(st *backtest.State) {
	i := s.sig.Len() - 1
	if s.inpos {
		if i-s.entry >= s.Hold {
			st.Position().Close()
			s.inpos = false
			s.entry = -1
		}
		return
	}
	if s.sig.Last() > 0 && !st.Position().IsLong() {
		st.Buy(backtest.Order{Size: s.Size})
		s.inpos = true
		s.entry = i
	}
}
