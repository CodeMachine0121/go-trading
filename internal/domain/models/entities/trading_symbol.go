package entities

import "time"

// TradingSymbol is a market the system knows about, whether or not it currently
// holds any K candles for it. It is a plain data model: fields, persistence mapping
// and shape conversion only, no business rules.
//
// The name is the key. A market is one market however many times it is registered,
// so there is nothing for a surrogate key to distinguish.
type TradingSymbol struct {
	Symbol string `gorm:"primaryKey;size:64;not null"`
	// Market is which market this symbol belongs to, and therefore where its candles
	// come from and which rules it runs by. Rows registered before markets existed
	// carry nothing here, which reads as the market this system had at the time.
	Market string `gorm:"size:32"`
	// IsWatched says whether the system keeps this symbol's candles up to date. It
	// lives here rather than on a list of its own because "the system knows this
	// market" and "the system is following it" are two facts about one thing, and
	// keeping them apart gave them two chances to disagree.
	IsWatched bool `gorm:"not null;default:false"`
	// DisplayName is what the venue calls this symbol — 台積電 for 2330. Empty for a
	// market that names nothing: a crypto pair is already its own name, and a
	// translated one would be a label the venue has never used.
	//
	// It is stored rather than looked up when needed, because it comes from the same
	// answer that proves the code real, and because a watchlist has to read the same
	// out of hours as it does at ten in the morning — a name fetched on demand would
	// vanish whenever the source did.
	DisplayName string `gorm:"size:128"`
	// RegisteredAt is when this symbol was first registered. It is what decides the
	// order symbols are considered in when a market can only follow a few of them at
	// once — earliest registered, first served.
	//
	// Rows registered before this was recorded all carry the zero value, so they tie;
	// name settles a tie, which keeps the order settled rather than arbitrary.
	RegisteredAt time.Time `gorm:"type:timestamptz"`
}

// TableName pins the table to TradingSymbols instead of GORM's default trading_symbols.
func (tradingSymbol TradingSymbol) TableName() string {
	return "TradingSymbols"
}
