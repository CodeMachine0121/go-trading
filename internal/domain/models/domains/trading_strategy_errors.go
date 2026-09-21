package domains

import (
	"errors"
	"fmt"
	"strings"
)

// ErrTradingStrategyValidation is the single refusal a caller sees when a trading
// strategy, one of its sources or one of its conditions breaks a rule. One sentinel
// rather than one per rule, so that a controller maps a rejected trading strategy to
// a status code without recognising a dozen sentinels — the wording says which rule;
// the sentinel says whose fault it is.
var ErrTradingStrategyValidation = errors.New("trading strategy validation failed")

// ErrTradingStrategyNotFound is every reason somebody cannot see a trading strategy:
// it does not exist, or it is not theirs. The two share one sentinel and one sentence
// on purpose. Told apart, anybody could walk the identifiers and learn which trading
// strategies exist in this system and which merely belong to other people.
var ErrTradingStrategyNotFound = errors.New("trading strategy not found")

// TradingStrategyNotFound is that refusal for one identifier.
func TradingStrategyNotFound(id uint) error {
	return fmt.Errorf("%w: 找不到識別碼為 %d 的交易策略", ErrTradingStrategyNotFound, id)
}

// ErrTradingStrategyNameConflict is this owner already having a trading strategy by
// that name. Somebody else having one is not a conflict — a name is what its owner
// recognises it by, and nobody recognises a stranger's.
var ErrTradingStrategyNameConflict = errors.New("trading strategy name already in use")

// ErrTradingStrategyBotRunning is a rewrite refused because a bot following these
// rules is running. Changing rules mid-round would leave nobody able to say which
// version that round used, so the answer is to stop those bots first rather than to
// guess.
var ErrTradingStrategyBotRunning = errors.New("a bot following this trading strategy is running")

// TradingStrategyBotRunning is that refusal, naming the bots to go and stop. A count
// alone would leave somebody opening every bot they own to find out which one.
func TradingStrategyBotRunning(runningBotNames []string) error {
	return fmt.Errorf(
		"%w: 這幾台機器人正在用它跑：%s，請先停止它們",
		ErrTradingStrategyBotRunning, strings.Join(runningBotNames, "、"))
}

// ErrTradingStrategyBotBorrowing is a rewrite refused because it would take away
// borrowing from rules a stopped bot is already suggesting a loan against.
//
// Saving such a bot is refused, so the only way to arrive at one is from this side:
// save it while the rules could borrow, then change the rules. The bot would then run
// suggesting a loan nobody could replay — the exact state the save gate exists to
// prevent, reached by the back door.
var ErrTradingStrategyBotBorrowing = errors.New(
	"a bot following this trading strategy suggests borrowing")

// TradingStrategyBotBorrowing is that refusal, naming the bots whose suggested
// leverage has to go first.
func TradingStrategyBotBorrowing(borrowingBotNames []string) error {
	return fmt.Errorf(
		"%w: 這幾台機器人正在用它建議槓桿：%s，改成借不到錢的交易模式會讓它們建議一個重演不出來的部位，"+
			"請先把那幾台的槓桿拿掉",
		ErrTradingStrategyBotBorrowing, strings.Join(borrowingBotNames, "、"))
}

// ErrTradingStrategyInUse is a delete refused because bots still follow these rules,
// running or not. Deleting would leave them pointing at something that is gone, and
// a bot that cannot reach its rules is indistinguishable from a broken one.
var ErrTradingStrategyInUse = errors.New("trading strategy is still in use")

// TradingStrategyInUse is that refusal for a number of bots.
func TradingStrategyInUse(botCount int) error {
	return fmt.Errorf(
		"%w: 還有 %d 台機器人正在用它，請先改掉或刪掉那幾台", ErrTradingStrategyInUse, botCount)
}
