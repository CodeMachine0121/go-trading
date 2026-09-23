package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// ContractBacktestAccountDomain is what a contract replay is holding at any moment: the
// cash not put down as margin, the one isolated position that may be open, the round
// trips already finished, and how many openings the venue refused.
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

// SettleFundingWithin pays or receives every funding settlement of this bar on the
// position carried into it. The settlements handed over are the bar's own — from its
// start up to but not including its close — and a position opened at the previous
// bar's close was already held when each of them happened.
//
// A settlement recorded without its mark price is valued at the bar's mark close.
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

// ApplyExitLevels closes the open position if this bar reached its stop, its
// liquidation price or its take profit.
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

// Apply carries out what this bar's signal asks the account to be holding. A position
// already facing the asked-for way stays; one facing the other way is closed first and
// its money returned, and only then is the new one opened from what the account then
// holds — so a reversal the account cannot afford leaves it flat.
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

// settleOpenPosition is the whole of letting go of a position, shared by the two ways
// out — a bar reaching a level and a signal asking for something else.
func (accountDomain *ContractBacktestAccountDomain) settleOpenPosition(
	closedTrade vo.ContractClosedTradeVo,
) {
	accountDomain.closedTrades = append(accountDomain.closedTrades, closedTrade)
	accountDomain.availableCash = accountDomain.availableCash.
		Add(accountDomain.openPosition.CashReturnedFor(closedTrade))
	accountDomain.hasOpenPosition = false
}

// EquityAt is the cash plus the open position valued at that price.
func (accountDomain *ContractBacktestAccountDomain) EquityAt(price decimal.Decimal) decimal.Decimal {
	if !accountDomain.hasOpenPosition {
		return accountDomain.availableCash
	}

	return accountDomain.availableCash.Add(accountDomain.openPosition.ValueAt(price))
}

// ClosedTradeDtos are the finished round trips, earliest first.
func (accountDomain *ContractBacktestAccountDomain) ClosedTradeDtos() []dto.ContractClosedTradeDto {
	closedTradeDtos := make([]dto.ContractClosedTradeDto, 0, len(accountDomain.closedTrades))
	for _, closedTrade := range accountDomain.closedTrades {
		closedTradeDtos = append(closedTradeDtos, closedTrade.ToDto())
	}

	return closedTradeDtos
}

// SummaryDto is the account's half of the report card: everything counted off the
// trades and the one position still open. Equity, return and drawdown come from the
// equity curve, and the maintenance margin basis from the trading rules.
//
// Everything is counted off the trade list rather than tallied as it went, for the
// reason the spot account does: a running total is a second place the same fact
// lives, and the day it disagrees with the list nobody can say which to believe.
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

	// A side that never finished a trade has no win rate, which is what keeps "no
	// trades" from reading as "every trade lost".
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
