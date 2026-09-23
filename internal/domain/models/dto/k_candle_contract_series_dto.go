package dto

// KCandleContractSeriesDto is one aggregated contract K candle series: the contract,
// the interval actually used, and the merged candles earliest first. It is not stored.
type KCandleContractSeriesDto struct {
	Symbol   string               `json:"symbol"`
	Interval string               `json:"interval"`
	KCandles []KCandleContractDto `json:"kCandles"`
}
