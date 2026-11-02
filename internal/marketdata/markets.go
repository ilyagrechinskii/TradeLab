package marketdata

// Market is a confirmed Binance Spot instrument supported by TradeLab.
type Market struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
}

var supportedMarkets = []Market{
	{Symbol: "BTCUSDT", Name: "Bitcoin"},
	{Symbol: "ETHUSDT", Name: "Ethereum"},
	{Symbol: "SOLUSDT", Name: "Solana"},
	{Symbol: "BNBUSDT", Name: "BNB"},
	{Symbol: "XRPUSDT", Name: "XRP"},
	{Symbol: "ADAUSDT", Name: "Cardano"},
	{Symbol: "DOGEUSDT", Name: "Dogecoin"},
	{Symbol: "LINKUSDT", Name: "Chainlink"},
	{Symbol: "AVAXUSDT", Name: "Avalanche"},
	{Symbol: "LTCUSDT", Name: "Litecoin"},
	{Symbol: "TRXUSDT", Name: "TRON"},
	{Symbol: "DOTUSDT", Name: "Polkadot"},
}

// Markets returns a copy, so callers cannot change the supported instruments.
func Markets() []Market { return append([]Market(nil), supportedMarkets...) }
func SupportedSymbols() []string {
	symbols := make([]string, len(supportedMarkets))
	for i, market := range supportedMarkets {
		symbols[i] = market.Symbol
	}
	return symbols
}
func supportedSymbol(symbol string) bool {
	for _, market := range supportedMarkets {
		if market.Symbol == symbol {
			return true
		}
	}
	return false
}
