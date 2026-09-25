package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// strategyBotRunningLimit caps each person's running bots, since every running bot is a fixed recurring script-and-network cost.
const strategyBotRunningLimit = 10

// StrategyBotRunStateDomain handles run-state transitions of an existing bot; each method returns the whole record so a transition can't be half-applied.
type StrategyBotRunStateDomain struct {
	bot entities.StrategyBot
}

func NewStrategyBotRunStateDomain(bot entities.StrategyBot) StrategyBotRunStateDomain {
	return StrategyBotRunStateDomain{bot: bot}
}

func (strategyBotRunStateDomain StrategyBotRunStateDomain) IsRunning() bool {
	return vo.StrategyBotRunStateVo(strategyBotRunStateDomain.bot.RunState) == vo.StrategyBotRunning
}

// RequireEditable refuses editing a running bot so every round uses a well-defined version.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) RequireEditable() error {
	if !strategyBotRunStateDomain.IsRunning() {
		return nil
	}

	return fmt.Errorf(
		"%w: 這台機器人正在執行中，要先停止它才改得動", ErrStrategyBotRunning)
}

// RequireStartable refuses a bot with no delivery setting (it would reach nobody) or one past the running limit (no other bot is stopped to make room).
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

// Start makes the bot due immediately and clears the last sent signal and halt reason, so the first conclusion after a start is always sent.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) Start(now time.Time) entities.StrategyBot {
	startedBot := strategyBotRunStateDomain.bot
	startedBot.RunState = string(vo.StrategyBotRunning)
	startedBot.NextRunAt = now.UTC()
	startedBot.LastSentSignal = ""
	startedBot.HaltReason = string(vo.StrategyBotHaltNone)
	startedBot.Conflicting = false

	return startedBot
}

func (strategyBotRunStateDomain StrategyBotRunStateDomain) Stop() entities.StrategyBot {
	stoppedBot := strategyBotRunStateDomain.bot
	stoppedBot.RunState = string(vo.StrategyBotStopped)
	stoppedBot.HaltReason = string(vo.StrategyBotHaltNone)

	return stoppedBot
}

// Halt keeps the reason until the next start; the list is the only place it can be seen since some halts are delivery failures.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) Halt(
	haltReason vo.StrategyBotHaltReasonVo,
) entities.StrategyBot {
	haltedBot := strategyBotRunStateDomain.bot
	haltedBot.RunState = string(vo.StrategyBotStopped)
	haltedBot.HaltReason = string(haltReason)

	return haltedBot
}

// RoundFinished schedules from now so missed rounds are never made up, and moves the last sent signal only when a message was actually delivered.
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

// RoundSkipped only moves the clock, leaving the conflict mark and last sent signal untouched.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) RoundSkipped(
	now time.Time,
) entities.StrategyBot {
	skippedBot := strategyBotRunStateDomain.bot
	skippedBot.NextRunAt = now.UTC().Add(
		time.Duration(skippedBot.TriggerIntervalMinutes) * time.Minute)

	return skippedBot
}
