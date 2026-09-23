package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

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

	// The trading specification is what the venue says trading this contract looks
	// like: how finely a price and a quantity move, the smallest order it takes, and
	// what holding and losing a leveraged position costs. It is the contract's own
	// property, so it lives on the contract — once — rather than being repeated on
	// every candle.
	//
	// Every figure is nullable for one reason: a contract registered before the
	// specification was recorded has none until the first refresh, and "not yet
	// recorded" is not a tick size of zero. SpecificationUpdatedAt being set is what
	// says the rest are.
	TickSize               decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	QuantityStep           decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	MinimumQuantity        decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	MinimumNotional        decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	MaintenanceMarginRate  decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	LiquidationFeeRate     decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	FundingIntervalHours   *int
	SpecificationUpdatedAt *time.Time `gorm:"type:timestamptz"`
}

// TableName pins the table to ContractTradingSymbols instead of GORM's default.
func (contractTradingSymbol ContractTradingSymbol) TableName() string {
	return "ContractTradingSymbols"
}

// ToDto is the shape this contract leaves the domain in.
func (contractTradingSymbol ContractTradingSymbol) ToDto() dto.ContractTradingSymbolDto {
	return dto.ContractTradingSymbolDto{
		Symbol:               contractTradingSymbol.Symbol,
		IsWatched:            contractTradingSymbol.IsWatched,
		TradingSpecification: contractTradingSymbol.toTradingSpecificationDto(),
	}
}

// toTradingSpecificationDto hands the specification outwards, or nothing when it has
// not been recorded yet.
func (contractTradingSymbol ContractTradingSymbol) toTradingSpecificationDto() *dto.ContractTradingSpecificationDto {
	if contractTradingSymbol.SpecificationUpdatedAt == nil || contractTradingSymbol.FundingIntervalHours == nil {
		return nil
	}

	return &dto.ContractTradingSpecificationDto{
		TickSize:               contractTradingSymbol.TickSize.Decimal,
		QuantityStep:           contractTradingSymbol.QuantityStep.Decimal,
		MinimumQuantity:        contractTradingSymbol.MinimumQuantity.Decimal,
		MinimumNotional:        contractTradingSymbol.MinimumNotional.Decimal,
		MaintenanceMarginRate:  contractTradingSymbol.MaintenanceMarginRate.Decimal,
		LiquidationFeeRate:     contractTradingSymbol.LiquidationFeeRate.Decimal,
		FundingIntervalHours:   *contractTradingSymbol.FundingIntervalHours,
		SpecificationUpdatedAt: contractTradingSymbol.SpecificationUpdatedAt.UTC(),
	}
}
