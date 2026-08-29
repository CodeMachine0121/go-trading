package _interface

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_k_candle_repository.go -destination=mocks/mock_i_k_candle_repository.go -package=mocks

// IKCandleRepository stores and retrieves K candles. Save carries the overwrite
// semantics: storing a candle whose symbol and open time already exist replaces it.
type IKCandleRepository interface {
	Save(kCandle entities.KCandle) (entities.KCandle, error)
	Update(kCandle entities.KCandle) (entities.KCandle, error)
	FindOne(symbol string, openTime time.Time) (entities.KCandle, error)
	FindInRange(query domains.KCandleQueryDomain, limit int) ([]entities.KCandle, error)
	// FindLatest returns at most limit K candles for the symbol, ordered by open
	// time NEWEST FIRST. Note this is the opposite order to FindInRange, because
	// "the latest few" is naturally a descending read.
	FindLatest(symbol string, limit int) ([]entities.KCandle, error)
	Delete(symbol string, openTime time.Time) error
}
