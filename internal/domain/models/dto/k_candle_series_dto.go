package dto

// KCandleSeriesDto is the only shape in which an aggregated K candle series leaves
// the domain. Interval names the aggregation interval the series was cut at, because
// the same range asked for at two intervals gives two different — and both correct —
// answers, so a reader never has to look back at what was requested. The candles run
// earliest first, and a bucket that held nothing is simply absent.
type KCandleSeriesDto struct {
	Symbol   string       `json:"symbol"`
	Interval string       `json:"interval"`
	KCandles []KCandleDto `json:"kCandles"`
}
