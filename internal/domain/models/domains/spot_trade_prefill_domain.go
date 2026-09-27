package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

const (
	spotTradePrefillModeNewTrade      = "newTrade"
	spotTradePrefillModeAddBuyFill    = "addBuyFill"
	spotTradePrefillModeAddSellFill   = "addSellFill"
	spotTradePrefillModeNoOpenHolding = "noOpenHolding"
)

// SpotTradePrefillDomain turns one remembered spot bot round into a form to confirm; it reads the round as it was, never the bot's settings today.
type SpotTradePrefillDomain struct {
	round        dto.JournalLinkRoundDto
	marketDomain SpotTradeMarketDomain
}

func NewSpotTradePrefillDomain(round dto.JournalLinkRoundDto, market string) SpotTradePrefillDomain {
	return SpotTradePrefillDomain{round: round, marketDomain: NewSpotTradeMarketDomain(market)}
}

// Prefill adds to the trade already held when there is one; an exit with nothing held says so instead of inventing a sale.
func (prefillDomain SpotTradePrefillDomain) Prefill(openTrade entities.SpotTradeRecord, hasOpenTrade bool) dto.SpotTradePrefillDto {
	round := prefillDomain.round
	tradingStrategyID := round.TradingStrategyID
	isExit := round.Result == string(vo.StrategyBotRoundResultSell)

	prefillDto := dto.SpotTradePrefillDto{
		Mode:                   spotTradePrefillModeNewTrade,
		StrategyBotID:          round.StrategyBotID,
		StrategyBotName:        round.StrategyBotName,
		RunNumber:              round.RunNumber,
		RanAt:                  round.RanAt,
		Signal:                 string(vo.StrategyBotRoundResultBuy),
		Symbol:                 round.Symbol,
		Market:                 string(prefillDomain.marketDomain.Market()),
		Price:                  round.ReferencePrice,
		TradingStrategyID:      &tradingStrategyID,
		PriceNeedsConfirmation: true,
	}

	if !round.ReferencePrice.Valid {
		prefillDto.MissingReferenceReason = missingReferenceRoundPredatesReferencePrices
	}

	if isExit {
		prefillDto.Signal = string(vo.StrategyBotRoundResultSell)
		if !hasOpenTrade {
			prefillDto.Mode = spotTradePrefillModeNoOpenHolding

			return prefillDto
		}

		openTradeID := openTrade.ID
		prefillDto.Mode = spotTradePrefillModeAddSellFill
		prefillDto.TargetTradeID = &openTradeID
		prefillDto.Quantity = decimal.NewNullDecimal(NewSpotTradeRecordDomain(openTrade).Ledger().Position())

		return prefillDto
	}

	prefillDto.PlannedStopLossPrice = round.SuggestedStopLossPrice
	prefillDto.PlannedTakeProfitPrice = round.SuggestedTakeProfitPrice
	if round.SuggestedStake.Valid && round.ReferencePrice.Valid && round.ReferencePrice.Decimal.IsPositive() {
		prefillDto.Quantity = decimal.NewNullDecimal(
			prefillDomain.marketDomain.PrefillQuantityOf(round.SuggestedStake.Decimal, round.ReferencePrice.Decimal))
	}

	if hasOpenTrade {
		openTradeID := openTrade.ID
		prefillDto.Mode = spotTradePrefillModeAddBuyFill
		prefillDto.TargetTradeID = &openTradeID
	}

	return prefillDto
}
