package domains

import (
	"errors"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyBotRoundFailureDomain decides whether a failed round on one watched symbol skips (retrying could succeed), halts that symbol alone (it can't, and the cause may lie in that symbol's market) or halts the whole bot (it can't on any symbol); unrecognised failures skip because halting is the destructive answer.
type StrategyBotRoundFailureDomain struct {
	haltReason vo.StrategyBotHaltReasonVo
	// haltsOnlyTheWatchedSymbol marks a failure that a script can meet on one market's data and not another's.
	haltsOnlyTheWatchedSymbol bool
}

func NewStrategyBotRoundFailureDomain(roundError error) StrategyBotRoundFailureDomain {
	switch {
	// Unpublished and deleted scripts share one reason so it doesn't reveal whether someone else's script exists.
	// A script that is not the bot owner's reads the same: a bot never runs someone else's rules.
	case errors.Is(roundError, ErrStrategyScriptNotFound),
		errors.Is(roundError, ErrStrategyScriptNotPublished),
		errors.Is(roundError, ErrStrategyScriptNotYours):
		return StrategyBotRoundFailureDomain{haltReason: vo.StrategyBotHaltStrategyScriptUnavailable}

	case errors.Is(roundError, ErrTradingStrategyNotFound):
		return StrategyBotRoundFailureDomain{
			haltReason: vo.StrategyBotHaltTradingStrategyUnavailable}

	case errors.Is(roundError, ErrIndicatorScriptFailed),
		errors.Is(roundError, ErrIndicatorParameterNotDeclared):
		return StrategyBotRoundFailureDomain{haltReason: vo.StrategyBotHaltScriptFailed, haltsOnlyTheWatchedSymbol: true}

	// Halted rather than skipped, both for reaching past 31 days and for too many price levels: a price step too fine for the symbol would otherwise skip every round without the owner ever hearing why. A source not answering or not ready in time skips.
	case errors.Is(roundError, ErrContractOrderFlowRefused):
		return StrategyBotRoundFailureDomain{haltReason: vo.StrategyBotHaltOrderFlowRefused, haltsOnlyTheWatchedSymbol: true}

	default:
		return StrategyBotRoundFailureDomain{haltReason: vo.StrategyBotHaltNone}
	}
}

// ToOutcomeDto hands out the combined outcome for the symbol so callers can't pair a halt with no reason.
func (strategyBotRoundFailureDomain StrategyBotRoundFailureDomain) ToOutcomeDto(
	symbol string,
) dto.StrategyBotWatchedSymbolRoundOutcomeDto {
	switch {
	case strategyBotRoundFailureDomain.haltReason == vo.StrategyBotHaltNone:
		return dto.StrategyBotWatchedSymbolRoundOutcomeDto{Symbol: symbol, Kind: strategyBotRoundSkipped}
	case strategyBotRoundFailureDomain.haltsOnlyTheWatchedSymbol:
		return dto.StrategyBotWatchedSymbolRoundOutcomeDto{
			Symbol: symbol, Kind: strategyBotRoundWatchedSymbolHalted,
			HaltReason: string(strategyBotRoundFailureDomain.haltReason),
		}
	default:
		return dto.StrategyBotWatchedSymbolRoundOutcomeDto{
			Symbol: symbol, Kind: strategyBotRoundHalted,
			HaltReason: string(strategyBotRoundFailureDomain.haltReason),
		}
	}
}

// HaltReason is empty for a failure that only skips; halting is derived from it so the two can't disagree.
func (strategyBotRoundFailureDomain StrategyBotRoundFailureDomain) HaltReason() vo.StrategyBotHaltReasonVo {
	return strategyBotRoundFailureDomain.haltReason
}
