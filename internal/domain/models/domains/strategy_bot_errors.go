package domains

import (
	"errors"
	"fmt"
)

// ErrStrategyBotValidation is the single refusal a caller sees when a bot, one of
// its sources or one of its conditions breaks a rule. One sentinel rather than one
// per rule, so that a controller maps a rejected bot to a status code without
// recognising a dozen sentinels — the wording says which rule; the sentinel says
// whose fault it is.
var ErrStrategyBotValidation = errors.New("strategy bot validation failed")

// ErrStrategyBotNotFound is every reason somebody cannot see a bot: it does not
// exist, or it is not theirs. The two share one sentinel and one sentence on
// purpose. Told apart, anybody could walk the identifiers and learn which bots exist
// in this system and which merely belong to other people.
var ErrStrategyBotNotFound = errors.New("strategy bot not found")

// StrategyBotNotFound is that refusal for one identifier.
func StrategyBotNotFound(id uint) error {
	return fmt.Errorf("%w: 找不到識別碼為 %d 的策略機器人", ErrStrategyBotNotFound, id)
}

// ErrStrategyBotNameConflict is this owner already having a bot by that name.
// Somebody else having one is not a conflict — a name is what its owner recognises a
// bot by, and nobody recognises a stranger's.
var ErrStrategyBotNameConflict = errors.New("strategy bot name already in use")

// ErrStrategyBotRunning is a change refused because the bot is running. Editing a
// bot mid-round would leave nobody able to say which version that round used, so the
// answer is to stop it first rather than to guess.
var ErrStrategyBotRunning = errors.New("strategy bot is running")

// ErrStrategyBotDeliveryNotConfigured is a start refused because its owner has
// nowhere to be spoken to. A bot that cannot send is a bot for which running and
// stopped are the same state.
var ErrStrategyBotDeliveryNotConfigured = errors.New("strategy bot delivery not configured")

// ErrStrategyBotRunningLimitReached is a start refused because this person already
// has as many running bots as they may have. Nothing of theirs is stopped to make
// room: they asked for one more, not for a swap.
var ErrStrategyBotRunningLimitReached = errors.New("strategy bot running limit reached")

// ErrStrategyBotAlreadyRunningARound is a hand-pressed round arriving while one is
// already in flight for that bot.
//
// It is refused rather than queued, for the reason the scan skips a busy bot: the
// waiting round would read the same candles and reach the same answer, and the
// second one to finish would arrive to find the bot already moved on.
var ErrStrategyBotAlreadyRunningARound = errors.New("strategy bot is already running a round")

// StrategyBotAlreadyRunningARound is that refusal. It names no identifier, because
// the person pressing the button is looking at the bot it is about.
func StrategyBotAlreadyRunningARound() error {
	return fmt.Errorf(
		"%w: 這台機器人正在跑一輪，等它跑完再試一次", ErrStrategyBotAlreadyRunningARound)
}
