package dto

// KCandleSeriesDto candles are earliest first, and empty buckets are omitted.
type KCandleSeriesDto struct {
	Symbol   string       `json:"symbol"`
	Interval string       `json:"interval"`
	KCandles []KCandleDto `json:"kCandles"`
}
