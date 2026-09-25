package entities

import "time"

// TradingSymbol is a market the system knows about, keyed by name, whether or not it holds any K candles for it.
type TradingSymbol struct {
	Symbol string `gorm:"primaryKey;size:64;not null"`
	// Market is empty on rows registered before markets existed, which reads as the original market.
	Market    string `gorm:"size:32"`
	IsWatched bool   `gorm:"not null;default:false"`
	// DisplayName is the venue's name for the symbol (台積電 for 2330), empty for crypto pairs; it is stored so watchlists read the same when the source is down.
	DisplayName string `gorm:"size:128"`
	// RegisteredAt orders symbols first-come-first-served when a market can follow only a few; zero-valued legacy rows tie and are settled by name.
	RegisteredAt time.Time `gorm:"type:timestamptz"`
}

func (tradingSymbol TradingSymbol) TableName() string {
	return "TradingSymbols"
}
