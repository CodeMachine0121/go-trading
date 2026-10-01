package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// BinanceTradingKey is one person's Binance trading key; both strings are stored sealed and the API key tail kept separately so reads never unseal anything.
type BinanceTradingKey struct {
	ID              uint   `gorm:"primaryKey"`
	UserID          uint   `gorm:"not null;uniqueIndex:idx_binance_trading_keys_user_id"`
	SealedApiKey    string `gorm:"size:512;not null"`
	SealedSecretKey string `gorm:"size:512;not null"`
	ApiKeyTail      string `gorm:"size:8;not null"`
	// The tradable markets are recorded once when stored; Binance is never asked again on read.
	SpotTradingEnabled     bool      `gorm:"not null;default:false"`
	ContractTradingEnabled bool      `gorm:"not null;default:false"`
	CreatedAt              time.Time `gorm:"type:timestamptz;not null"`
	// UpdatedAt is the configured time, and also the token that proves an auto-order switch was checked against this very key.
	UpdatedAt time.Time `gorm:"type:timestamptz;not null"`
}

func (binanceTradingKey BinanceTradingKey) TableName() string {
	return "BinanceTradingKeys"
}

func (binanceTradingKey BinanceTradingKey) ToDto() dto.BinanceTradingKeyDto {
	return dto.BinanceTradingKeyDto{
		Configured:      true,
		ApiKeyTail:      binanceTradingKey.ApiKeyTail,
		TradableMarkets: binanceTradingKey.tradableMarkets(),
		ConfiguredAt:    binanceTradingKey.UpdatedAt.UTC(),
	}
}

func (binanceTradingKey BinanceTradingKey) ToStatusDto() dto.BinanceTradingKeyStatusDto {
	return dto.BinanceTradingKeyStatusDto{
		Configured:      true,
		TradableMarkets: binanceTradingKey.tradableMarkets(),
		ConfiguredAt:    binanceTradingKey.UpdatedAt.UTC(),
	}
}

// tradableMarkets lists spot before contract so every reader sees one order.
func (binanceTradingKey BinanceTradingKey) tradableMarkets() []string {
	tradableMarkets := make([]string, 0, 2)
	if binanceTradingKey.SpotTradingEnabled {
		tradableMarkets = append(tradableMarkets, string(vo.TradableMarketSpot))
	}
	if binanceTradingKey.ContractTradingEnabled {
		tradableMarkets = append(tradableMarkets, string(vo.TradableMarketContract))
	}

	return tradableMarkets
}
