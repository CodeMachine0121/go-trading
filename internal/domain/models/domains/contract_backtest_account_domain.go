package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// ContractBacktestAccountDomain holds a contract replay's free cash, at most one isolated position, finished round trips and the count of openings the venue's rules blocked.
type ContractBacktestAccountDomain struct {
	positionTerms       ContractPositionTermsDomain
	availableCash       decimal.Decimal
	openPosition        ContractBacktestPositionDomain
	hasOpenPosition     bool
	positionOpenCount   int
	blockedOpeningCount int
	closedTrades        []vo.ContractClosedTradeVo
}

func NewContractBacktestAccountDomain(
	initialCapital decimal.Decimal, positionTerms ContractPositionTermsDomain,
) *ContractBacktestAccountDomain {
	return &ContractBacktestAccountDomain{
		positionTerms: positionTerms,
		availableCash: initialCapital,
		closedTrades:  make([]vo.ContractClosedTradeVo, 0),
	}
}

// SettleFundingWithin applies the bar's own settlements (from its open up to but excluding its close) to the position carried into it; a settlement without a mark price uses the bar's mark close.
func (accountDomain *ContractBacktestAccountDomain) SettleFundingWithin(
	bucket dto.KCandleContractDto, settlements []entities.ContractFundingRateSettlement,
) {
	if !accountDomain.hasOpenPosition {
		return
	}

	for _, settlement := range settlements {
		markPrice := bucket.MarkClose
		if settlement.MarkPrice.Valid {
			markPrice = settlement.MarkPrice.Decimal
		}

		accountDomain.openPosition.SettleFunding(settlement.FundingRate, markPrice)
	}
}

// ApplyExitLevels closes the open position if the bar reached its stop, liquidation price or take profit.
func (accountDomain *ContractBacktestAccountDomain) ApplyExitLevels(
	bucket dto.KCandleContractDto, candleTime time.Time,
) {
	if !accountDomain.hasOpenPosition {
		return
	}

	closedTrade, isExited := accountDomain.openPosition.ExitOn(bucket, candleTime)
	if !isExited {
		return
	}

	accountDomain.settleOpenPosition(closedTrade)
}

// Apply keeps a position already facing the target way, otherwise closes it first and opens from the resulting cash, so an unaffordable reversal leaves the account flat.
func (accountDomain *ContractBacktestAccountDomain) Apply(
	targetPosition vo.TargetPositionVo, candleTime time.Time, closePrice decimal.Decimal,
) {
	if targetPosition == vo.TargetPositionUnchanged {
		return
	}

	targetDirection := vo.PositionDirectionLong
	if targetPosition == vo.TargetPositionShort {
		targetDirection = vo.PositionDirectionShort
	}
	wantsPosition := targetPosition != vo.TargetPositionFlat

	if accountDomain.hasOpenPosition {
		if wantsPosition && accountDomain.openPosition.Direction() == targetDirection {
			return
		}

		accountDomain.settleOpenPosition(
			accountDomain.openPosition.ClosedBySignalAt(candleTime, closePrice))
	}

	if !wantsPosition {
		return
	}

	openedPosition, openingOutcome := accountDomain.positionTerms.OpenFor(
		targetDirection, candleTime, closePrice, accountDomain.availableCash)
	if openingOutcome == vo.ContractOpeningBlockedByTradingRules {
		accountDomain.blockedOpeningCount++
	}
	if openingOutcome != vo.ContractOpeningOpened {
		return
	}

	accountDomain.availableCash = accountDomain.availableCash.
		Sub(openedPosition.OpeningMargin()).Sub(openedPosition.EntryCost())
	accountDomain.openPosition = openedPosition
	accountDomain.hasOpenPosition = true
	accountDomain.positionOpenCount++
}

// settleOpenPosition is shared by level exits and signal exits.
func (accountDomain *ContractBacktestAccountDomain) settleOpenPosition(
	closedTrade vo.ContractClosedTradeVo,
) {
	accountDomain.closedTrades = append(accountDomain.closedTrades, closedTrade)
	accountDomain.availableCash = accountDomain.availableCash.
		Add(accountDomain.openPosition.CashReturnedFor(closedTrade))
	accountDomain.hasOpenPosition = false
}

func (accountDomain *ContractBacktestAccountDomain) EquityAt(price decimal.Decimal) decimal.Decimal {
	if !accountDomain.hasOpenPosition {
		return accountDomain.availableCash
	}

	return accountDomain.availableCash.Add(accountDomain.openPosition.ValueAt(price))
}

// ClosedTradeDtos returns finished round trips earliest first.
func (accountDomain *ContractBacktestAccountDomain) ClosedTradeDtos() []dto.ContractClosedTradeDto {
	closedTradeDtos := make([]dto.ContractClosedTradeDto, 0, len(accountDomain.closedTrades))
	for _, closedTrade := range accountDomain.closedTrades {
		closedTradeDtos = append(closedTradeDtos, closedTrade.ToDto())
	}

	return closedTradeDtos
}

// SummaryDto covers trade-derived figures only (equity, return and drawdown come from the curve), all counted from the trade list rather than running totals.
func (accountDomain *ContractBacktestAccountDomain) SummaryDto() dto.ContractBacktestSummaryDto {
	summaryDto := dto.ContractBacktestSummaryDto{
		PositionOpenCount:    accountDomain.positionOpenCount,
		BlockedOpeningCount:  accountDomain.blockedOpeningCount,
		TotalTransactionCost: decimal.Zero,
		TotalFundingFee:      decimal.Zero,
	}

	winCountByDirection := map[vo.PositionDirectionVo]int{}
	outcomes := make([]vo.TradeOutcomeVo, 0, len(accountDomain.closedTrades))
	for _, closedTrade := range accountDomain.closedTrades {
		outcomes = append(outcomes, closedTrade.ToOutcomeVo())
		summaryDto.TotalTransactionCost = summaryDto.TotalTransactionCost.
			Add(closedTrade.EntryCost).Add(closedTrade.ExitCost)
		summaryDto.TotalFundingFee = summaryDto.TotalFundingFee.Add(closedTrade.FundingFee)

		switch closedTrade.ExitReason {
		case vo.TradeExitReasonStopLoss:
			summaryDto.StopLossExitCount++
		case vo.TradeExitReasonTakeProfit:
			summaryDto.TakeProfitExitCount++
		case vo.TradeExitReasonLiquidation:
			summaryDto.LiquidationExitCount++
		}

		if closedTrade.Direction == vo.PositionDirectionShort {
			summaryDto.ShortTradeCount++
		} else {
			summaryDto.LongTradeCount++
		}
		if closedTrade.IsWin() {
			winCountByDirection[closedTrade.Direction]++
		}
	}

	if accountDomain.hasOpenPosition {
		summaryDto.TotalTransactionCost = summaryDto.TotalTransactionCost.
			Add(accountDomain.openPosition.EntryCost())
		summaryDto.TotalFundingFee = summaryDto.TotalFundingFee.
			Add(accountDomain.openPosition.FundingFeePaid())
	}

	summaryDto.BacktestTradeStatisticsDto = NewBacktestTradeStatisticsDomain(outcomes).ToDto()

	// A side with no finished trades has no win rate, so "no trades" is not read as "every trade lost".
	shareOf := func(winCount int, tradeCount int) *float64 {
		if tradeCount == 0 {
			return nil
		}

		share := float64(winCount) / float64(tradeCount)

		return &share
	}

	summaryDto.WinRate = shareOf(
		winCountByDirection[vo.PositionDirectionLong]+winCountByDirection[vo.PositionDirectionShort],
		len(accountDomain.closedTrades))
	summaryDto.LongWinRate = shareOf(
		winCountByDirection[vo.PositionDirectionLong], summaryDto.LongTradeCount)
	summaryDto.ShortWinRate = shareOf(
		winCountByDirection[vo.PositionDirectionShort], summaryDto.ShortTradeCount)

	return summaryDto
}
