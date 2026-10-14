package _interface

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

//go:generate go tool mockgen -source=i_contract_trade_history_proxy.go -destination=mocks/mock_i_contract_trade_history_proxy.go -package=mocks

// IContractTradeHistoryProxy reads a perpetual contract's individual trades, handing each to visit oldest first rather than collecting a day of them in memory.
type IContractTradeHistoryProxy interface {
	// VisitDailyTrades streams one UTC day's published archive; a day with no file yet returns found = false with no error.
	VisitDailyTrades(
		executionContext context.Context, symbol string, day time.Time, visit func(vo.ContractAggregatedTradeVo),
	) (found bool, visitError error)
	// VisitTradesBetween asks the live venue for every trade from start (included) to end (excluded).
	VisitTradesBetween(
		executionContext context.Context, symbol string, start time.Time, end time.Time,
		visit func(vo.ContractAggregatedTradeVo),
	) error
}
