package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
)

// ContractStrategyBotMarketDomain checks at save time that a contract bot watches a followed contract within its leverage ceiling, since an unwatched contract has no bars and would silently hold forever.
// Later watchlist or ladder changes do not void an already saved bot.
type ContractStrategyBotMarketDomain struct {
	symbol    string
	isWatched bool
	// maximumLeverage is the smallest tier's limit; hasLadder says whether it is known at all.
	maximumLeverage int
	hasLadder       bool
}

func NewContractStrategyBotMarketDomain(
	symbol string,
	contractTradingSymbol entities.ContractTradingSymbol,
	isRegistered bool,
	maintenanceMarginTiers []entities.ContractMaintenanceMarginTier,
) ContractStrategyBotMarketDomain {
	maximumLeverage := 0
	for _, maintenanceMarginTier := range maintenanceMarginTiers {
		maximumLeverage = max(maximumLeverage, maintenanceMarginTier.MaximumLeverage)
	}

	return ContractStrategyBotMarketDomain{
		symbol:          symbol,
		isWatched:       isRegistered && contractTradingSymbol.IsWatched,
		maximumLeverage: maximumLeverage,
		hasLadder:       len(maintenanceMarginTiers) > 0,
	}
}

// Admit refuses an unwatched contract or leverage above the ladder's ceiling, using the same wording as a contract replay; no ladder means no ceiling.
func (contractMarketDomain ContractStrategyBotMarketDomain) Admit(leverage decimal.Decimal) error {
	if !contractMarketDomain.isWatched {
		return fmt.Errorf(
			"%w: %s 不在合約追蹤名單上，系統沒有在收它的合約行情——請先把它加進合約追蹤名單",
			ErrStrategyBotValidation, contractMarketDomain.symbol)
	}

	if contractMarketDomain.hasLadder &&
		leverage.GreaterThan(decimal.NewFromInt(int64(contractMarketDomain.maximumLeverage))) {
		return fmt.Errorf("%w: 這個合約標的最高只能開 %d 倍槓桿",
			ErrStrategyBotValidation, contractMarketDomain.maximumLeverage)
	}

	return nil
}
