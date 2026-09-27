package domains

import "github.com/CodeMachine0121/go-trading/internal/domain/models/entities"

// TradingStrategyNamesDomain names the strategy a trade followed, or says it was deleted; unreadable names mark nothing as deleted.
type TradingStrategyNamesDomain struct {
	namesByID map[uint]string
	readable  bool
}

func NewTradingStrategyNamesDomain(tradingStrategies []entities.TradingStrategy, readable bool) TradingStrategyNamesDomain {
	namesByID := map[uint]string{}
	for _, tradingStrategy := range tradingStrategies {
		namesByID[tradingStrategy.ID] = tradingStrategy.Name
	}

	return TradingStrategyNamesDomain{namesByID: namesByID, readable: readable}
}

// Describe is empty and not deleted for a trade that followed no strategy.
func (namesDomain TradingStrategyNamesDomain) Describe(tradingStrategyID *uint) (string, bool) {
	if tradingStrategyID == nil {
		return "", false
	}

	name, isKnown := namesDomain.namesByID[*tradingStrategyID]

	return name, namesDomain.readable && !isKnown
}
