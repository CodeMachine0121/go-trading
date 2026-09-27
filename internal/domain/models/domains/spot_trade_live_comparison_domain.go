package domains

import (
	"slices"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// SpotTradeLiveComparisonDomain groups a spot strategy's closed live trades by symbol and sets each group beside a replay of its stretch.
type SpotTradeLiveComparisonDomain struct {
	closedTrades []dto.SpotTradeRecordDto
}

func NewSpotTradeLiveComparisonDomain(closedTrades []dto.SpotTradeRecordDto) SpotTradeLiveComparisonDomain {
	return SpotTradeLiveComparisonDomain{closedTrades: closedTrades}
}

// Plan replays each symbol from its first buy to its last sell, all in and without costs, since spot keeps no fee rates.
func (comparisonDomain SpotTradeLiveComparisonDomain) Plan() dto.SpotTradeComparisonPlanDto {
	tradesBySymbol := map[string][]dto.SpotTradeRecordDto{}
	symbols := []string{}
	for _, trade := range comparisonDomain.closedTrades {
		if _, isGrouped := tradesBySymbol[trade.Symbol]; !isGrouped {
			symbols = append(symbols, trade.Symbol)
		}
		tradesBySymbol[trade.Symbol] = append(tradesBySymbol[trade.Symbol], trade)
	}
	slices.Sort(symbols)

	groups := make([]dto.SpotTradeComparisonGroupDto, 0, len(symbols))
	for _, symbol := range symbols {
		trades := tradesBySymbol[symbol]
		startTime := trades[0].OpenedAt
		endTime := *trades[0].ClosedAt
		for _, trade := range trades {
			if trade.OpenedAt.Before(startTime) {
				startTime = trade.OpenedAt
			}
			if trade.ClosedAt.After(endTime) {
				endTime = *trade.ClosedAt
			}
		}

		groups = append(groups, dto.SpotTradeComparisonGroupDto{
			Symbol: symbol,
			Market: trades[0].Market,
			Live:   comparisonDomain.figuresOf(trades),
			BacktestRequest: dto.TradingStrategyBacktestRequestDto{
				Symbol:             symbol,
				StartTime:          startTime,
				EndTime:            endTime,
				InitialCapital:     liveComparisonInitialCapital,
				PositionSizingMode: string(vo.PositionSizingModeAllIn),
			},
		})
	}

	allTrades := NewSpotTradeResultsDomain(comparisonDomain.closedTrades).Tally()

	return dto.SpotTradeComparisonPlanDto{
		Groups:                         groups,
		AverageEntrySlippagePercentage: allTrades.AverageEntrySlippagePercentage(),
		EntrySlippageTradeCount:        allTrades.EntrySlippageTradeCount(),
	}
}

// ComposeForDeletedTradingStrategy keeps the live figures and says why nothing was replayed.
func (comparisonDomain SpotTradeLiveComparisonDomain) ComposeForDeletedTradingStrategy(
	groups []dto.SpotTradeComparisonGroupDto,
) []dto.SpotTradeLiveComparisonRowDto {
	attempts := make([]dto.SpotTradeBacktestAttemptDto, len(groups))
	for index := range attempts {
		attempts[index].FailureReason = "交易策略已刪除，無法重演"
	}

	return comparisonDomain.Compose(groups, attempts)
}

// Compose lines each group up with how its replay went, in the same order.
func (comparisonDomain SpotTradeLiveComparisonDomain) Compose(
	groups []dto.SpotTradeComparisonGroupDto, attempts []dto.SpotTradeBacktestAttemptDto,
) []dto.SpotTradeLiveComparisonRowDto {
	rows := make([]dto.SpotTradeLiveComparisonRowDto, 0, len(groups))
	for index, group := range groups {
		row := dto.SpotTradeLiveComparisonRowDto{
			Symbol:    group.Symbol,
			Market:    group.Market,
			StartTime: group.BacktestRequest.StartTime,
			EndTime:   group.BacktestRequest.EndTime,
			Live:      group.Live,
		}

		if index < len(attempts) {
			attempt := attempts[index]
			if attempt.FailureReason != "" {
				row.BacktestUnavailableReason = attempt.FailureReason
			} else {
				row.Backtest = &dto.SpotTradeComparisonFiguresDto{
					ClosedTradeCount: len(attempt.Result.ClosedTrades),
					WinRate:          attempt.Result.Summary.WinRate,
				}
			}
		}

		rows = append(rows, row)
	}

	return rows
}

func (comparisonDomain SpotTradeLiveComparisonDomain) figuresOf(trades []dto.SpotTradeRecordDto) dto.SpotTradeLiveFiguresDto {
	tally := NewSpotTradeResultsDomain(trades).Tally()

	return dto.SpotTradeLiveFiguresDto{
		SpotTradeComparisonFiguresDto: dto.SpotTradeComparisonFiguresDto{
			ClosedTradeCount: tally.TradeCount(),
			WinRate:          tally.WinRate(),
		},
		AverageEntrySlippagePercentage: tally.AverageEntrySlippagePercentage(),
		EntrySlippageTradeCount:        tally.EntrySlippageTradeCount(),
	}
}
