package domains

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

// StrategyBotVerdictDomain is one round's conclusion and whether to send it; comparing against the last signal sent (not concluded) means repeated conclusions send once and quiet rounds never displace it.
type StrategyBotVerdictDomain struct {
	verdict        vo.StrategyBotVerdictVo
	lastSentSignal vo.SignalVo
}

// NewStrategyBotVerdictDomain treats both conditions holding as a conflict rather than picking a side.
func NewStrategyBotVerdictDomain(
	buyConditionHolds bool, sellConditionHolds bool, lastSentSignal string,
) StrategyBotVerdictDomain {
	verdict := vo.StrategyBotVerdictNone

	switch {
	case buyConditionHolds && sellConditionHolds:
		verdict = vo.StrategyBotVerdictConflict
	case buyConditionHolds:
		verdict = vo.StrategyBotVerdictBuy
	case sellConditionHolds:
		verdict = vo.StrategyBotVerdictSell
	}

	return StrategyBotVerdictDomain{
		verdict:        verdict,
		lastSentSignal: vo.SignalVo(lastSentSignal),
	}
}

func (strategyBotVerdictDomain StrategyBotVerdictDomain) Verdict() vo.StrategyBotVerdictVo {
	return strategyBotVerdictDomain.verdict
}

// IsConflicting is not a halt; the mark clears on the next non-conflicting round.
func (strategyBotVerdictDomain StrategyBotVerdictDomain) IsConflicting() bool {
	return strategyBotVerdictDomain.verdict == vo.StrategyBotVerdictConflict
}

// Signal reports no signal for conflict and quiet rounds.
func (strategyBotVerdictDomain StrategyBotVerdictDomain) Signal() (vo.SignalVo, bool) {
	switch strategyBotVerdictDomain.verdict {
	case vo.StrategyBotVerdictBuy:
		return vo.SignalBuy, true
	case vo.StrategyBotVerdictSell:
		return vo.SignalSell, true
	default:
		return "", false
	}
}

// ShouldSend suppresses repeats of the last sent signal; an empty last signal (after a start) guarantees the first conclusion is sent.
func (strategyBotVerdictDomain StrategyBotVerdictDomain) ShouldSend() bool {
	signal, hasSignal := strategyBotVerdictDomain.Signal()

	return hasSignal && signal != strategyBotVerdictDomain.lastSentSignal
}
