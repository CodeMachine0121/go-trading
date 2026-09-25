package dto

// KCandleHistorySyncDto has no interval because only one-minute candles are stored;
// LookbackDays has no default, so zero is refused.
type KCandleHistorySyncDto struct {
	Symbol       string
	LookbackDays int
}
