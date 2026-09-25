package domains

import (
	"errors"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyBotRoundFailureDomain decides whether a failed round skips (retrying could succeed) or halts the bot (it can't); unrecognised failures skip because halting is the destructive answer.
type StrategyBotRoundFailureDomain struct {
	haltReason vo.StrategyBotHaltReasonVo
}

func NewStrategyBotRoundFailureDomain(roundError error) StrategyBotRoundFailureDomain {
	switch {
	// Unpublished and deleted scripts share one reason so it doesn't reveal whether someone else's script exists.
	case errors.Is(roundError, ErrStrategyScriptNotFound),
		errors.Is(roundError, ErrStrategyScriptNotPublished):
		return StrategyBotRoundFailureDomain{haltReason: vo.StrategyBotHaltStrategyScriptUnavailable}

	case errors.Is(roundError, ErrTradingStrategyNotFound):
		return StrategyBotRoundFailureDomain{
			haltReason: vo.StrategyBotHaltTradingStrategyUnavailable}

	case errors.Is(roundError, ErrIndicatorScriptFailed),
		errors.Is(roundError, ErrIndicatorParameterNotDeclared):
		return StrategyBotRoundFailureDomain{haltReason: vo.StrategyBotHaltScriptFailed}

	case errors.Is(roundError, ErrTelegramDeliveryNotConfigured):
		return StrategyBotRoundFailureDomain{
			haltReason: vo.StrategyBotHaltDeliveryNotConfigured}

	default:
		return StrategyBotRoundFailureDomain{haltReason: vo.StrategyBotHaltNone}
	}
}

// NewStrategyBotDeliveryFailureDomain halts on a rejected token or unknown chat (the owner must fix them) and skips on unreachable Telegram.
func NewStrategyBotDeliveryFailureDomain(
	failureReason vo.DeliveryFailureReasonVo,
) StrategyBotRoundFailureDomain {
	switch failureReason {
	case vo.DeliveryFailureCredentialRejected:
		return StrategyBotRoundFailureDomain{haltReason: vo.StrategyBotHaltCredentialRejected}
	case vo.DeliveryFailureDestinationNotFound:
		return StrategyBotRoundFailureDomain{haltReason: vo.StrategyBotHaltDestinationNotFound}
	default:
		return StrategyBotRoundFailureDomain{haltReason: vo.StrategyBotHaltNone}
	}
}

// ToOutcomeDto hands out the combined outcome so callers can't pair a halt with no reason.
func (strategyBotRoundFailureDomain StrategyBotRoundFailureDomain) ToOutcomeDto() dto.StrategyBotRoundOutcomeDto {
	if !strategyBotRoundFailureDomain.HaltsTheBot() {
		return dto.StrategyBotRoundOutcomeDto{Kind: strategyBotRoundSkipped}
	}

	return dto.StrategyBotRoundOutcomeDto{
		Kind:       strategyBotRoundHalted,
		HaltReason: string(strategyBotRoundFailureDomain.haltReason),
	}
}

func (strategyBotRoundFailureDomain StrategyBotRoundFailureDomain) HaltsTheBot() bool {
	return strategyBotRoundFailureDomain.haltReason != vo.StrategyBotHaltNone
}

// HaltReason is empty for a failure that only skips; halting is derived from it so the two can't disagree.
func (strategyBotRoundFailureDomain StrategyBotRoundFailureDomain) HaltReason() vo.StrategyBotHaltReasonVo {
	return strategyBotRoundFailureDomain.haltReason
}
