package dto

// TradingSymbolDto is the only shape in which a tradable symbol leaves the domain.
// It is an object rather than a bare name because the next thing anyone will ask for
// is what else we know about it — how many candles, how recent the latest one is —
// and adding a field is a compatible change while turning names into objects is not.
type TradingSymbolDto struct {
	Symbol string `json:"symbol"`
}
