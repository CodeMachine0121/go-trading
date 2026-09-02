package entities

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// TradingSymbol is a market the system knows about, whether or not it currently
// holds any K candles for it. It is a plain data model: fields, persistence mapping
// and shape conversion only, no business rules.
//
// The name is the key. A market is one market however many times it is registered,
// so there is nothing for a surrogate key to distinguish.
type TradingSymbol struct {
	Symbol string `gorm:"primaryKey;size:64;not null"`
}

// TableName pins the table to TradingSymbols instead of GORM's default trading_symbols.
func (tradingSymbol TradingSymbol) TableName() string {
	return "TradingSymbols"
}

// ToDto converts this record into the shape the domain hands outwards.
func (tradingSymbol TradingSymbol) ToDto() dto.TradingSymbolDto {
	return dto.TradingSymbolDto{Symbol: tradingSymbol.Symbol}
}
