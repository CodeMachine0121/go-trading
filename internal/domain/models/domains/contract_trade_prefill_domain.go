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
	strategyBot entities.StrategyBot
	runRecord   entities.StrategyBotRunRecord
}

func NewContractTradePrefillDomain(
	strategyBot entities.StrategyBot, runRecord entities.StrategyBotRunRecord,
) ContractTradePrefillDomain {
	return ContractTradePrefillDomain{strategyBot: strategyBot, runRecord: runRecord}
}

func (prefillDomain ContractTradePrefillDomain) Direction() string {
	return prefillDomain.runRecord.SuggestedDirection
}

// Prefill switches to adding an entry fill when the person already holds the symbol that way, since a second open trade would be refused.
func (prefillDomain ContractTradePrefillDomain) Prefill(
	openTrade entities.ContractTradeRecord, hasOpenTrade bool,
) dto.ContractTradePrefillDto {
	tradingStrategyID := prefillDomain.strategyBot.TradingStrategyID
	prefillDto := dto.ContractTradePrefillDto{
		Mode:                        contractTradePrefillModeNewTrade,
		StrategyBotID:               prefillDomain.strategyBot.ID,
		StrategyBotName:             prefillDomain.strategyBot.Name,
		RunNumber:                   prefillDomain.runRecord.RunNumber,
		RanAt:                       prefillDomain.runRecord.RanAt.UTC(),
		Symbol:                      prefillDomain.strategyBot.Symbol,
		Direction:                   prefillDomain.runRecord.SuggestedDirection,
		Leverage:                    prefillDomain.runRecord.SuggestedLeverage,
		PlannedStopLossPrice:        prefillDomain.runRecord.SuggestedStopLossPrice,
		PlannedTakeProfitPrice:      prefillDomain.runRecord.SuggestedTakeProfitPrice,
		TradingStrategyID:           &tradingStrategyID,
		EntryPrice:                  prefillDomain.runRecord.ReferencePrice,
		Quantity:                    prefillDomain.runRecord.SuggestedQuantity,
		EntryPriceNeedsConfirmation: true,
	}

	if !prefillDomain.runRecord.ReferencePrice.Valid {
		prefillDto.MissingReferenceReason = missingReferenceRoundPredatesReferencePrices
	}

	if hasOpenTrade {
		openTradeID := openTrade.ID
		prefillDto.Mode = contractTradePrefillModeAddEntryFill
		prefillDto.TargetTradeID = &openTradeID
	}

	return prefillDto
}
