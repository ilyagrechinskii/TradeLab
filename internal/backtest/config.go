package backtest

import "fmt"

// Config uses percentage points: FeePercent = 0.1 means a 0.1% fee.
// Only spot trading of one asset against one quote currency is supported.
type Config struct {
	InitialCapital  float64
	FeePercent      float64
	SlippagePercent float64
}

func DefaultConfig() Config {
	return Config{InitialCapital: 10_000, FeePercent: 0.1, SlippagePercent: 0}
}

// Validate requires positive finite capital and finite percentages in [0, 100).
// Run uses Config as supplied; zero fees and zero slippage are valid settings.
func (cfg Config) Validate() error {
	if !finite(cfg.InitialCapital) || cfg.InitialCapital <= 0 {
		return fmt.Errorf("initial capital must be positive and finite")
	}
	if !finite(cfg.FeePercent) || cfg.FeePercent < 0 || cfg.FeePercent >= 100 {
		return fmt.Errorf("fee percent must be finite and in [0, 100)")
	}
	if !finite(cfg.SlippagePercent) || cfg.SlippagePercent < 0 || cfg.SlippagePercent >= 100 {
		return fmt.Errorf("slippage percent must be finite and in [0, 100)")
	}
	return nil
}
