package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ContractReplayInputDomain is market data prepared outside the stored minute candles, so a replay can run on candles the venue already aggregated.
type ContractReplayInputDomain struct {
	symbol        string
	specification vo.ContractTradingSpecificationVo
	marginTiers   []vo.ContractMaintenanceMarginTierVo
	kCandles      []entities.KCandleContract
	settlements   []entities.ContractFundingRateSettlement
	confirmedAt   time.Time
}

func NewContractReplayInputDomain(
	symbol string,
	specification vo.ContractTradingSpecificationVo,
	marginTiers []vo.ContractMaintenanceMarginTierVo,
	kCandles []entities.KCandleContract,
	settlements []entities.ContractFundingRateSettlement,
	confirmedAt time.Time,
) ContractReplayInputDomain {
	return ContractReplayInputDomain{
		symbol:        symbol,
		specification: specification,
		marginTiers:   marginTiers,
		kCandles:      kCandles,
		settlements:   settlements,
		confirmedAt:   confirmedAt.UTC(),
	}
}

// TradingRules falls back to the specification's base rate when the margin ladder is missing or unreadable, as a replay of a followed contract does.
func (replayInputDomain ContractReplayInputDomain) TradingRules() (ContractTradingRulesDomain, error) {
	specificationDomain, specificationError := NewContractTradingSpecificationDomain(replayInputDomain.specification)
	if specificationError != nil {
		return ContractTradingRulesDomain{}, specificationError
	}
	contractTradingSymbol := specificationDomain.ApplyTo(
		entities.ContractTradingSymbol{Symbol: replayInputDomain.symbol}, replayInputDomain.confirmedAt)

	storedTiers := []entities.ContractMaintenanceMarginTier{}
	if ladderDomain, ladderError := NewContractMaintenanceMarginLadderDomain(vo.ContractMaintenanceMarginLadderVo{
		Symbol: replayInputDomain.symbol, Tiers: replayInputDomain.marginTiers,
	}); ladderError == nil {
		storedTiers = ladderDomain.ToEntities(replayInputDomain.confirmedAt)
	}

	return NewContractTradingRulesDomain(contractTradingSymbol, true, storedTiers)
}

func (replayInputDomain ContractReplayInputDomain) CandlesIn(query KCandleQueryDomain) []entities.KCandleContract {
	inside := make([]entities.KCandleContract, 0, len(replayInputDomain.kCandles))
	for _, kCandle := range replayInputDomain.kCandles {
		if !kCandle.OpenTime.Before(query.StartTime()) && !kCandle.OpenTime.After(query.EndTime()) {
			inside = append(inside, kCandle)
		}
	}

	return inside
}

func (replayInputDomain ContractReplayInputDomain) SettlementsIn(
	query KCandleQueryDomain,
) []entities.ContractFundingRateSettlement {
	inside := make([]entities.ContractFundingRateSettlement, 0, len(replayInputDomain.settlements))
	for _, settlement := range replayInputDomain.settlements {
		if !settlement.SettlementTime.Before(query.StartTime()) && !settlement.SettlementTime.After(query.EndTime()) {
			inside = append(inside, settlement)
		}
	}

	return inside
}

func (replayInputDomain ContractReplayInputDomain) LatestSettlementBefore(
	cutoff time.Time,
) (entities.ContractFundingRateSettlement, bool) {
	latest := entities.ContractFundingRateSettlement{}
	found := false
	for _, settlement := range replayInputDomain.settlements {
		if settlement.SettlementTime.Before(cutoff) && (!found || settlement.SettlementTime.After(latest.SettlementTime)) {
			latest, found = settlement, true
		}
	}

	return latest, found
}
