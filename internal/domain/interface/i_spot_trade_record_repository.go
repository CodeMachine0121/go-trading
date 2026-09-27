package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_spot_trade_record_repository.go -destination=mocks/mock_i_spot_trade_record_repository.go -package=mocks

// ISpotTradeRecordRepository stores a spot trade together with its buys, sells, notes and tags; ownership checks are the domain's job.
type ISpotTradeRecordRepository interface {
	// Create reports a second open trade for the same owner and symbol as ErrSpotTradeOpenHoldingExists, decided by the database.
	Create(
		executionContext context.Context, record entities.SpotTradeRecord,
	) (entities.SpotTradeRecord, error)
	// Save keeps the identifiers of fills that survive, so a fill can be amended by the identifier the person saw.
	Save(
		executionContext context.Context, record entities.SpotTradeRecord,
	) (entities.SpotTradeRecord, error)
	FindOne(executionContext context.Context, id uint) (entities.SpotTradeRecord, error)
	// FindPageByOwner orders newest first buy first and also answers how many match without the limit.
	FindPageByOwner(
		executionContext context.Context, ownerID uint, filter vo.TradeListFilterVo,
	) ([]entities.SpotTradeRecord, int64, error)
	// FindClosedByOwner returns closed and reviewed trades, closed no earlier than closedSince when it is given.
	FindClosedByOwner(
		executionContext context.Context, ownerID uint, closedSince *time.Time,
	) ([]entities.SpotTradeRecord, error)
	FindClosedByOwnerAndTradingStrategy(
		executionContext context.Context, ownerID uint, tradingStrategyID uint,
	) ([]entities.SpotTradeRecord, error)
	FindOpenByOwnerSymbol(
		executionContext context.Context, ownerID uint, symbol string,
	) (entities.SpotTradeRecord, bool, error)
	Delete(executionContext context.Context, id uint) error
	CountByTag(executionContext context.Context, tagID uint) (int64, error)
}
