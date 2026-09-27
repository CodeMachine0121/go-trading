package domains

import (
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

const (
	contractTradePrefillModeNewTrade             = "newTrade"
	contractTradePrefillModeAddEntryFill         = "addEntryFill"
	missingReferenceRoundPredatesReferencePrices = "roundPredatesReferencePrices"
)

// ContractTradePrefillDomain turns one remembered bot round into a form to confirm; it reads the round as it was, never the bot's settings today.
type ContractTradePrefillDomain struct {
	round dto.JournalLinkRoundDto
}

func NewContractTradePrefillDomain(round dto.JournalLinkRoundDto) ContractTradePrefillDomain {
	return ContractTradePrefillDomain{round: round}
}

func (prefillDomain ContractTradePrefillDomain) Direction() string {
	return prefillDomain.round.SuggestedDirection
}

// Prefill switches to adding an entry fill when the person already holds the symbol that way, since a second open trade would be refused.
func (prefillDomain ContractTradePrefillDomain) Prefill(
	openTrade entities.ContractTradeRecord, hasOpenTrade bool,
) dto.ContractTradePrefillDto {
	round := prefillDomain.round
	tradingStrategyID := round.TradingStrategyID
	prefillDto := dto.ContractTradePrefillDto{
		Mode:                        contractTradePrefillModeNewTrade,
		StrategyBotID:               round.StrategyBotID,
		StrategyBotName:             round.StrategyBotName,
		RunNumber:                   round.RunNumber,
		RanAt:                       round.RanAt,
		Symbol:                      round.Symbol,
		Direction:                   round.SuggestedDirection,
		Leverage:                    round.SuggestedLeverage,
		PlannedStopLossPrice:        round.SuggestedStopLossPrice,
		PlannedTakeProfitPrice:      round.SuggestedTakeProfitPrice,
		TradingStrategyID:           &tradingStrategyID,
		EntryPrice:                  round.ReferencePrice,
		Quantity:                    round.SuggestedQuantity,
		EntryPriceNeedsConfirmation: true,
	}

	if !round.ReferencePrice.Valid {
		prefillDto.MissingReferenceReason = missingReferenceRoundPredatesReferencePrices
	}

	if hasOpenTrade {
		openTradeID := openTrade.ID
		prefillDto.Mode = contractTradePrefillModeAddEntryFill
		prefillDto.TargetTradeID = &openTradeID
	}

	return prefillDto
}
