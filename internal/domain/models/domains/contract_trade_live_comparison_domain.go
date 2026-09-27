package domains

import (
	"slices"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// liveComparisonInitialCapital is a fixed stake chosen only so win rates can be compared; it says nothing about the person's money.
var liveComparisonInitialCapital = decimal.NewFromInt(10000)

// ContractTradeLiveComparisonDomain groups a strategy's closed live trades by symbol and sets each group beside a replay of its stretch.
type ContractTradeLiveComparisonDomain struct {
	closedTrades []dto.ContractTradeRecordDto
	takerFeeRate decimal.Decimal
}

func NewContractTradeLiveComparisonDomain(
	closedTrades []dto.ContractTradeRecordDto, takerFeeRate decimal.Decimal,
) ContractTradeLiveComparisonDomain {
	return ContractTradeLiveComparisonDomain{closedTrades: closedTrades, takerFeeRate: takerFeeRate}
}

// Plan replays each symbol from its first entry to its last exit at the leverage of its latest trade, all in and paying the taker rate both ways.
func (comparisonDomain ContractTradeLiveComparisonDomain) Plan() dto.ContractTradeComparisonPlanDto {
	tradesBySymbol := map[string][]dto.ContractTradeRecordDto{}
	symbols := []string{}
	for _, trade := range comparisonDomain.closedTrades {
		if _, isGrouped := tradesBySymbol[trade.Symbol]; !isGrouped {
			symbols = append(symbols, trade.Symbol)
		}
		tradesBySymbol[trade.Symbol] = append(tradesBySymbol[trade.Symbol], trade)
	}
	slices.Sort(symbols)

	groups := make([]dto.ContractTradeComparisonGroupDto, 0, len(symbols))
	for _, symbol := range symbols {
		trades := tradesBySymbol[symbol]
		startTime := trades[0].OpenedAt
		endTime := *trades[0].ClosedAt
		latestTrade := trades[0]
		for _, trade := range trades {
			if trade.OpenedAt.Before(startTime) {
				startTime = trade.OpenedAt
			}
			if trade.ClosedAt.After(endTime) {
				endTime = *trade.ClosedAt
			}
			if trade.ClosedAt.After(*latestTrade.ClosedAt) {
				latestTrade = trade
			}
		}

		groups = append(groups, dto.ContractTradeComparisonGroupDto{
			Symbol: symbol,
			Live:   comparisonDomain.figuresOf(trades),
			BacktestRequest: dto.ContractTradingStrategyBacktestRequestDto{
				Symbol:              symbol,
				StartTime:           startTime,
				EndTime:             endTime,
				InitialCapital:      liveComparisonInitialCapital,
				PositionSizingMode:  string(vo.PositionSizingModeAllIn),
				EntryCostPercentage: comparisonDomain.takerFeeRate,
				ExitCostPercentage:  comparisonDomain.takerFeeRate,
				Leverage:            latestTrade.Leverage,
			},
		})
	}

	allTrades := NewContractTradeResultsDomain(comparisonDomain.closedTrades).Tally()

	return dto.ContractTradeComparisonPlanDto{
		Groups:                         groups,
		AverageEntrySlippagePercentage: allTrades.AverageEntrySlippagePercentage(),
		EntrySlippageTradeCount:        allTrades.EntrySlippageTradeCount(),
	}
}

// ComposeForDeletedTradingStrategy keeps the live figures and says why nothing was replayed.
func (comparisonDomain ContractTradeLiveComparisonDomain) ComposeForDeletedTradingStrategy(
	groups []dto.ContractTradeComparisonGroupDto,
) []dto.ContractTradeLiveComparisonRowDto {
	attempts := make([]dto.ContractTradeBacktestAttemptDto, len(groups))
	for index := range attempts {
		attempts[index].FailureReason = "交易策略已刪除，無法重演"
	}

	return comparisonDomain.Compose(groups, attempts)
}

// Compose lines each group up with how its replay went, in the same order.
func (comparisonDomain ContractTradeLiveComparisonDomain) Compose(
	groups []dto.ContractTradeComparisonGroupDto, attempts []dto.ContractTradeBacktestAttemptDto,
) []dto.ContractTradeLiveComparisonRowDto {
	rows := make([]dto.ContractTradeLiveComparisonRowDto, 0, len(groups))
	for index, group := range groups {
		row := dto.ContractTradeLiveComparisonRowDto{
			Symbol:    group.Symbol,
			StartTime: group.BacktestRequest.StartTime,
			EndTime:   group.BacktestRequest.EndTime,
			Leverage:  group.BacktestRequest.Leverage,
			Live:      group.Live,
		}

		if index < len(attempts) {
			attempt := attempts[index]
			if attempt.FailureReason != "" {
				row.BacktestUnavailableReason = attempt.FailureReason
			} else {
				summary := attempt.Result.Summary
				row.Backtest = &dto.ContractTradeComparisonFiguresDto{
					ClosedTradeCount: len(attempt.Result.ClosedTrades),
					WinRate:          summary.WinRate,
					LongWinRate:      summary.LongWinRate,
					ShortWinRate:     summary.ShortWinRate,
				}
			}
		}

		rows = append(rows, row)
	}

	return rows
}

func (comparisonDomain ContractTradeLiveComparisonDomain) figuresOf(
	trades []dto.ContractTradeRecordDto,
) dto.ContractTradeLiveFiguresDto {
	tally := NewContractTradeResultsDomain(trades).Tally()

	return dto.ContractTradeLiveFiguresDto{
		ContractTradeComparisonFiguresDto: dto.ContractTradeComparisonFiguresDto{
			ClosedTradeCount: tally.TradeCount(),
			WinRate:          tally.WinRate(),
			LongWinRate:      tally.OfDirection(vo.PositionDirectionLong).WinRate(),
			ShortWinRate:     tally.OfDirection(vo.PositionDirectionShort).WinRate(),
		},
		AverageEntrySlippagePercentage: tally.AverageEntrySlippagePercentage(),
		EntrySlippageTradeCount:        tally.EntrySlippageTradeCount(),
	}
}
