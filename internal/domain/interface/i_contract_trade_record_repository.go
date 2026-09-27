package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_contract_trade_record_repository.go -destination=mocks/mock_i_contract_trade_record_repository.go -package=mocks

// IContractTradeRecordRepository stores a trade together with its fills, notes and tags; ownership checks are the domain's job.
type IContractTradeRecordRepository interface {
	// Create reports a second open trade for the same owner, symbol and direction as ErrContractTradeOpenPositionExists, decided by the database.
	Create(
		executionContext context.Context, record entities.ContractTradeRecord,
	) (entities.ContractTradeRecord, error)
	// Save keeps the identifiers of fills that survive, so a fill can be amended by the identifier the person saw.
	Save(
		executionContext context.Context, record entities.ContractTradeRecord,
	) (entities.ContractTradeRecord, error)
	FindOne(executionContext context.Context, id uint) (entities.ContractTradeRecord, error)
	// FindPageByOwner orders newest first entry first and also answers how many match without the limit.
	FindPageByOwner(
		executionContext context.Context, ownerID uint, filter vo.ContractTradeListFilterVo,
	) ([]entities.ContractTradeRecord, int64, error)
	// FindClosedByOwner returns closed and reviewed trades, closed no earlier than closedSince when it is given.
	FindClosedByOwner(
		executionContext context.Context, ownerID uint, closedSince *time.Time,
	) ([]entities.ContractTradeRecord, error)
	FindClosedByOwnerAndTradingStrategy(
		executionContext context.Context, ownerID uint, tradingStrategyID uint,
	) ([]entities.ContractTradeRecord, error)
	FindOpenByOwnerSymbolDirection(
		executionContext context.Context, ownerID uint, symbol string, direction string,
	) (entities.ContractTradeRecord, bool, error)
	Delete(executionContext context.Context, id uint) error
	CountByTag(executionContext context.Context, tagID uint) (int64, error)
}
