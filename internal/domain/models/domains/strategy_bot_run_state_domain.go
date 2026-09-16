package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// strategyBotRunningLimit is how many bots one person may have running at once.
//
// Every running bot is a fixed repeating cost — up to ten interpreted scripts and an
// outbound call, every interval, forever. Without a ceiling there is no ceiling, and
// the one that is hit first is this one.
const strategyBotRunningLimit = 10

// StrategyBotRunStateDomain is the life of a bot that already exists: whether it may
// be started, edited or picked up, and what each of those does to it.
//
// It is separate from StrategyBotDomain because the two answer to different things.
// One is about whether a bot is well formed, and runs whenever somebody saves;
// the other is about what a well-formed bot is currently doing, and runs whenever
// somebody presses a button or a round finishes. Merged, every save would have to
// carry a run state it has no business touching.
//
// Every method hands back the whole record rather than mutating in place, so that a
// caller cannot half-apply a transition and then fail before storing it.
type StrategyBotRunStateDomain struct {
	bot entities.StrategyBot
}

// NewStrategyBotRunStateDomain reads one stored bot.
func NewStrategyBotRunStateDomain(bot entities.StrategyBot) StrategyBotRunStateDomain {
	return StrategyBotRunStateDomain{bot: bot}
}

// IsRunning says whether the scan is entitled to pick this bot up.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) IsRunning() bool {
	return vo.StrategyBotRunStateVo(strategyBotRunStateDomain.bot.RunState) == vo.StrategyBotRunning
}

// RequireEditable refuses a change to a running bot.
//
// Not a convenience: a bot edited mid-round leaves nobody able to say which version
// that round used, and the round in question is the one that sends somebody a
// message they will act on. Stopping first makes the answer say itself.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) RequireEditable() error {
	if !strategyBotRunStateDomain.IsRunning() {
		return nil
	}

	return fmt.Errorf(
		"%w: 這台機器人正在執行中，要先停止它才改得動", ErrStrategyBotRunning)
}

// RequireStartable refuses a start that must not happen.
//
// Having nowhere to be spoken to is refused rather than allowed to fail later,
// because a bot that cannot send is a bot for which running and stopped are the same
// state — it would look started, do the work, and reach nobody.
//
// Being at the limit refuses this one rather than making room by stopping another.
// They asked for one more bot, not for a swap, and a system that silently turned one
// of theirs off would be picking which.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) RequireStartable(
	hasDeliverySetting bool, runningBotCount int,
) error {
	if !hasDeliverySetting {
		return fmt.Errorf(
			"%w: 要先完成 Telegram 設定，這台機器人才送得出訊息",
			ErrStrategyBotDeliveryNotConfigured)
	}

	if runningBotCount >= strategyBotRunningLimit {
		return fmt.Errorf(
			"%w: 同時執行中的機器人上限是 %d 台",
			ErrStrategyBotRunningLimitReached, strategyBotRunningLimit)
	}

	return nil
}

// Start puts this bot to work, due immediately.
//
// Due immediately because waiting a whole interval would leave a bot set to sixty
// minutes doing nothing for an hour after its owner pressed play, with no way to
// tell that apart from being broken.
//
// The last sent signal is cleared, which is what makes the first conclusion after a
// start always go out. Starting is starting over: somebody who stopped a bot,
// changed their mind and started it again is owed the current picture, not silence
// because the answer happens to match one from yesterday.
//
// The halt reason goes too. They read it and pressed play, so it is dealt with.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) Start(now time.Time) entities.StrategyBot {
	startedBot := strategyBotRunStateDomain.bot
	startedBot.RunState = string(vo.StrategyBotRunning)
	startedBot.NextRunAt = now.UTC()
	startedBot.LastSentSignal = ""
	startedBot.HaltReason = string(vo.StrategyBotHaltNone)
	startedBot.Conflicting = false

	return startedBot
}

// Stop is its owner taking it off duty. It carries no halt reason, because there is
// nothing wrong with it.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) Stop() entities.StrategyBot {
	stoppedBot := strategyBotRunStateDomain.bot
	stoppedBot.RunState = string(vo.StrategyBotStopped)
	stoppedBot.HaltReason = string(vo.StrategyBotHaltNone)

	return stoppedBot
}

// Halt is the system stopping it, and recording what somebody has to go and fix.
//
// The reason stays on the bot until it is started again. There is no way to tell its
// owner at the moment it happens: two of the four reasons are the message path
// itself being broken, so the only place a halt can be seen is the list.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) Halt(
	haltReason vo.StrategyBotHaltReasonVo,
) entities.StrategyBot {
	haltedBot := strategyBotRunStateDomain.bot
	haltedBot.RunState = string(vo.StrategyBotStopped)
	haltedBot.HaltReason = string(haltReason)

	return haltedBot
}

// RoundFinished books in a round that ran to a conclusion.
//
// The next round is measured from now rather than from when this one was due, which
// is the whole of "missed rounds are never made up". A system that was down for two
// hours wakes a five-minute bot once, not twenty-four times: a signal is about the
// present, and twenty-three answers about the past are twenty-three messages nobody
// wants.
//
// The last sent signal moves only when a message actually arrived. A conclusion that
// could not be delivered has not been said, so the next round says it again; and a
// quiet or conflicting round leaves it alone, so the silence in between never eats
// the change that follows it.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) RoundFinished(
	now time.Time, sentSignal vo.SignalVo, conflicting bool,
) entities.StrategyBot {
	finishedBot := strategyBotRunStateDomain.bot
	finishedBot.NextRunAt = now.UTC().Add(
		time.Duration(finishedBot.TriggerIntervalMinutes) * time.Minute)
	finishedBot.Conflicting = conflicting

	if sentSignal != "" {
		finishedBot.LastSentSignal = string(sentSignal)
	}

	return finishedBot
}

// RoundSkipped books in a round that could not reach a conclusion for a reason that
// may well have gone away by the next one.
//
// It moves the clock and nothing else. Leaving the conflict mark and the last sent
// signal where they are is what keeps a closed weekend from looking like a change of
// mind on Monday.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) RoundSkipped(
	now time.Time,
) entities.StrategyBot {
	skippedBot := strategyBotRunStateDomain.bot
	skippedBot.NextRunAt = now.UTC().Add(
		time.Duration(skippedBot.TriggerIntervalMinutes) * time.Minute)

	return skippedBot
}
