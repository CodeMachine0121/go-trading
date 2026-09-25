package dto

// KCandleContractSeriesDto is not stored; candles are earliest first.
type KCandleContractSeriesDto struct {
	Symbol   string               `json:"symbol"`
	Interval string               `json:"interval"`
	KCandles []KCandleContractDto `json:"kCandles"`
}
