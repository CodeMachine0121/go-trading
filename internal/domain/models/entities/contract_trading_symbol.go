package entities

import "github.com/CodeMachine0121/go-trading/internal/domain/models/dto"

// ContractTradingSymbol is a perpetual contract the system knows about, whether or
// not it currently holds any contract K candles for it.
//
// It is a separate list from TradingSymbol rather than a row in it, because a symbol
// names a different instrument on each venue and TradingSymbol keys on the name
// alone. Sharing the list would make BTCUSDT registrable once, on one venue.
//
// It carries neither a display name nor a market, and both omissions are deliberate.
// The contract venue names nothing — a pair is already its own name — and this list
// serves exactly one venue, so a column recording which one would hold the same value
// on every row.
type ContractTradingSymbol struct {
	Symbol string `gorm:"primaryKey;size:64;not null"`
	// IsWatched says whether the system keeps this contract's candles up to date. It
	// lives here rather than on a list of its own because "the system knows this
	// contract" and "the system is following it" are two facts about one thing.
	IsWatched bool `gorm:"not null;default:false"`
}

// TableName pins the table to ContractTradingSymbols instead of GORM's default.
func (contractTradingSymbol ContractTradingSymbol) TableName() string {
	return "ContractTradingSymbols"
}

// ToDto is the shape this contract leaves the domain in.
func (contractTradingSymbol ContractTradingSymbol) ToDto() dto.ContractTradingSymbolDto {
	return dto.ContractTradingSymbolDto{
		Symbol:    contractTradingSymbol.Symbol,
		IsWatched: contractTradingSymbol.IsWatched,
	}
}
