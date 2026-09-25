package domains

import (
	"errors"
	"fmt"
	"strings"
)

// ErrTradingStrategyValidation is one sentinel for every rule; the wrapped message says
// which rule failed.
var ErrTradingStrategyValidation = errors.New("trading strategy validation failed")

// ErrTradingStrategyNotFound covers both missing and not-owned so identifiers cannot be probed.
var ErrTradingStrategyNotFound = errors.New("trading strategy not found")

func TradingStrategyNotFound(id uint) error {
	return fmt.Errorf("%w: 找不到識別碼為 %d 的交易策略", ErrTradingStrategyNotFound, id)
}

// ErrTradingStrategyNameConflict applies only within one owner's trading strategies.
var ErrTradingStrategyNameConflict = errors.New("trading strategy name already in use")

// ErrTradingStrategyBotRunning refuses rewrites while a bot is running so each round has an
// unambiguous rule version.
var ErrTradingStrategyBotRunning = errors.New("a bot following this trading strategy is running")

func TradingStrategyBotRunning(runningBotNames []string) error {
	return fmt.Errorf(
		"%w: 這幾台機器人正在用它跑：%s，請先停止它們",
		ErrTradingStrategyBotRunning, strings.Join(runningBotNames, "、"))
}

// ErrTradingStrategyInUse refuses deletes while any bot, running or not, still follows these rules.
var ErrTradingStrategyInUse = errors.New("trading strategy is still in use")

func TradingStrategyInUse(botCount int) error {
	return fmt.Errorf(
		"%w: 還有 %d 台機器人正在用它，請先改掉或刪掉那幾台", ErrTradingStrategyInUse, botCount)
}
