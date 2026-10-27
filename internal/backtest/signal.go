package backtest

import "fmt"

type SignalType uint8

const (
	Hold SignalType = iota
	Buy
	Sell
)

// Signal is produced after CandleIndex closes and can execute only at the next
// candle's open. Missing signals mean Hold. Only one signal per index is allowed.
type Signal struct {
	CandleIndex int
	Type        SignalType
}

func indexSignals(signals []Signal, candleCount int) (map[int]SignalType, error) {
	indexed := make(map[int]SignalType, len(signals))
	for i, signal := range signals {
		if signal.CandleIndex < 0 || signal.CandleIndex >= candleCount {
			return nil, fmt.Errorf("signal %d: candle index is out of range", i)
		}
		if signal.Type != Hold && signal.Type != Buy && signal.Type != Sell {
			return nil, fmt.Errorf("signal %d: unknown signal type", i)
		}
		if _, exists := indexed[signal.CandleIndex]; exists {
			return nil, fmt.Errorf("signal %d: duplicate candle index %d", i, signal.CandleIndex)
		}
		indexed[signal.CandleIndex] = signal.Type
	}
	return indexed, nil
}
