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

// Start makes the bot due immediately and clears every watched symbol's last sent signal and halt, so the first conclusion after a start is always sent.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) Start(now time.Time) entities.StrategyBot {
	startedBot := strategyBotRunStateDomain.bot
	startedBot.RunState = string(vo.StrategyBotRunning)
	startedBot.NextRunAt = now.UTC()
	startedBot.HaltReason = string(vo.StrategyBotHaltNone)

	startedBot.WatchedSymbols = make([]entities.StrategyBotWatchedSymbol, 0, len(strategyBotRunStateDomain.bot.WatchedSymbols))
	for _, watchedSymbol := range strategyBotRunStateDomain.bot.WatchedSymbols {
		startedBot.WatchedSymbols = append(startedBot.WatchedSymbols,
			NewStrategyBotWatchedSymbolDomain(watchedSymbol).ClearedForStart())
	}

	return startedBot
}

func (strategyBotRunStateDomain StrategyBotRunStateDomain) Stop() entities.StrategyBot {
	stoppedBot := strategyBotRunStateDomain.bot
	stoppedBot.RunState = string(vo.StrategyBotStopped)
	stoppedBot.HaltReason = string(vo.StrategyBotHaltNone)

	return stoppedBot
}

// Halt keeps the reason until the next start; the list is the only place it can be seen since some halts are delivery failures.
// It moves the due time to now, so a round some replica is still working on can no longer be booked onto the stopped bot.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) Halt(
	haltReason vo.StrategyBotHaltReasonVo, now time.Time,
) entities.StrategyBot {
	haltedBot := strategyBotRunStateDomain.bot
	haltedBot.RunState = string(vo.StrategyBotStopped)
	haltedBot.HaltReason = string(haltReason)
	haltedBot.NextRunAt = now.UTC()

	return haltedBot
}

// Rescheduled schedules from now so missed rounds are never made up.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) Rescheduled(now time.Time) entities.StrategyBot {
	rescheduledBot := strategyBotRunStateDomain.bot
	rescheduledBot.NextRunAt = now.UTC().Add(
		time.Duration(rescheduledBot.TriggerIntervalMinutes) * time.Minute)

	return rescheduledBot
}

// WithWatchedSymbol replaces the watched symbol of the same symbol, leaving the stored bot it was read from untouched.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) WithWatchedSymbol(
	changedSymbol entities.StrategyBotWatchedSymbol,
) StrategyBotRunStateDomain {
	changedBot := strategyBotRunStateDomain.bot
	changedBot.WatchedSymbols = make([]entities.StrategyBotWatchedSymbol, 0, len(strategyBotRunStateDomain.bot.WatchedSymbols))
	for _, watchedSymbol := range strategyBotRunStateDomain.bot.WatchedSymbols {
		if watchedSymbol.Symbol == changedSymbol.Symbol {
			watchedSymbol = changedSymbol
		}
		changedBot.WatchedSymbols = append(changedBot.WatchedSymbols, watchedSymbol)
	}

	return StrategyBotRunStateDomain{bot: changedBot}
}

// WatchedSymbol is the domain of one of the bot's symbols; false when the bot does not watch it.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) WatchedSymbol(
	symbol string,
) (StrategyBotWatchedSymbolDomain, bool) {
	watchedSymbol, isWatched := strategyBotRunStateDomain.bot.WatchedSymbol(symbol)

	return NewStrategyBotWatchedSymbolDomain(watchedSymbol), isWatched
}

// AllWatchedSymbolsHalted is the moment a bot has nothing left to watch and stops as a whole.
func (strategyBotRunStateDomain StrategyBotRunStateDomain) AllWatchedSymbolsHalted() bool {
	for _, watchedSymbol := range strategyBotRunStateDomain.bot.WatchedSymbols {
		if !NewStrategyBotWatchedSymbolDomain(watchedSymbol).IsHalted() {
			return false
		}
	}

	return len(strategyBotRunStateDomain.bot.WatchedSymbols) > 0
}
