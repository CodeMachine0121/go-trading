package domains

import "github.com/CodeMachine0121/go-trading/internal/domain/models/vo"

// StrategyBotVerdictDomain is what one round concluded, and whether it is worth
// saying out loud.
//
// The two live in one model because they change for one reason. "Both conditions
// held" and "neither held" are both silences, and the rule that makes them silent —
// neither is an opinion — is the same rule that says neither may displace the last
// signal actually sent. Split across two models, that rule would be written twice
// and the two copies would eventually disagree.
//
// Comparing against the last signal *sent* rather than the last one *concluded* is
// the whole of "only speak when something changes": a run of identical conclusions
// produces one message, and a quiet round in the middle of that run does not turn
// the next real change into a repeat.
type StrategyBotVerdictDomain struct {
	verdict        vo.StrategyBotVerdictVo
	lastSentSignal vo.SignalVo
}

// NewStrategyBotVerdictDomain reads the two conditions' answers as one conclusion.
//
// Both holding is a conflict rather than a choice. This bot has said it should buy
// and sell in the same breath, and picking a side would hand its owner an opinion
// the system invented — one they would act on and never trace back.
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

// Verdict is what this round concluded.
func (strategyBotVerdictDomain StrategyBotVerdictDomain) Verdict() vo.StrategyBotVerdictVo {
	return strategyBotVerdictDomain.verdict
}

// IsConflicting says the two conditions held at once. It is not a halt — the bot
// keeps running and the mark clears itself on the next round that does not conflict
// — but it is the only way its owner ever learns their conditions overlap.
func (strategyBotVerdictDomain StrategyBotVerdictDomain) IsConflicting() bool {
	return strategyBotVerdictDomain.verdict == vo.StrategyBotVerdictConflict
}

// Signal is the conclusion as a signal, and whether there is one at all. A conflict
// and a quiet round both answer no, because neither is something to say.
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

// ShouldSend says whether this round is worth a message.
//
// A bot waking every five minutes on a condition that holds for an hour concludes
// the same thing twelve times. Sending all twelve would get the bot muted, and a
// muted bot is the same as no bot at all.
//
// An empty last signal — which is what starting a bot leaves behind — differs from
// every conclusion, so the first one after pressing play always goes out. Somebody
// who pressed play and then heard nothing all night cannot tell a quiet market from
// a broken bot.
func (strategyBotVerdictDomain StrategyBotVerdictDomain) ShouldSend() bool {
	signal, hasSignal := strategyBotVerdictDomain.Signal()

	return hasSignal && signal != strategyBotVerdictDomain.lastSentSignal
}
