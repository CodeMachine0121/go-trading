package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
)

// ContractStrategyBotMarketDomain is one perpetual contract as a contract bot sees it
// when the bot is saved: is the system following it, and how much leverage may a
// position on it carry.
//
// A contract bot reads the contract bars that are stored, and only the contracts on the
// contract watchlist have bars arriving. A bot watching any other contract would say
// "hold" every round — which from outside looks exactly like a bot that ran fine and
// concluded nothing. So it is refused where its owner is there to read why, rather than
// discovered weeks later.
//
// Both answers are about the bot being saved, never about a round. A contract taken off
// the watchlist later, or a ladder that tightens later, does not reach back and void a
// bot that was saved under the old answer.
type ContractStrategyBotMarketDomain struct {
	symbol    string
	isWatched bool
	// maximumLeverage is the most any position on this contract may carry — what its
	// smallest tier allows — and hasLadder whether the system knows that at all.
	maximumLeverage int
	hasLadder       bool
}

// NewContractStrategyBotMarketDomain reads the contract as it is stored: its entry in
// the contract trading symbols, whether there was one, and its maintenance margin
// ladder.
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

// Admit refuses a contract bot that would watch a contract nobody is following, or
// carry more leverage than the contract allows any position. A contract with no ladder
// yet names no ceiling, and this does not invent one.
//
// The ceiling is refused in the words a contract replay refuses it in, so the same 150
// typed into either comes back with the same sentence.
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
