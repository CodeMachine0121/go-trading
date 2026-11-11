package domains

import (
	"errors"
	"fmt"
	"strings"
)

// ErrStrategyScriptValidation wraps a message naming the broken rule.
var ErrStrategyScriptValidation = errors.New("strategy script validation failed")

// ErrStrategyScriptNameConflict is deliberately not a validation failure: the content is fine but the name is taken.
var ErrStrategyScriptNameConflict = errors.New("strategy script name already in use")

var ErrStrategyScriptNotFound = errors.New("strategy script not found")

// ErrStrategyScriptFromMarketplace refuses changing a copy adopted from the marketplace, whose algorithm is its author's.
var ErrStrategyScriptFromMarketplace = errors.New("strategy script was adopted from the marketplace")

func StrategyScriptFromMarketplaceNotRewritable() error {
	return fmt.Errorf("%w: 從市集加入的策略腳本不能改寫", ErrStrategyScriptFromMarketplace)
}

func StrategyScriptFromMarketplaceNotRepublishable() error {
	return fmt.Errorf("%w: 從市集加入的策略腳本不能再發佈", ErrStrategyScriptFromMarketplace)
}

// ErrStrategyScriptNotYours refuses building rules on someone else's script, so no author can change what another
// person's bot runs.
var ErrStrategyScriptNotYours = errors.New("strategy script is not yours")

func StrategyScriptNotYours(id uint) error {
	return fmt.Errorf("%w: 識別碼為 %d 的策略腳本不是你的，請先把它加入你的策略腳本", ErrStrategyScriptNotYours, id)
}

// ErrStrategyScriptBotRunning refuses a rewrite while the owner's running bots use the script, since a round must not straddle two versions of it.
var ErrStrategyScriptBotRunning = errors.New("strategy script is used by a running bot")

func StrategyScriptBotRunning(runningBotNames []string) error {
	return fmt.Errorf(
		"%w: 這幾台機器人正在用它跑：%s，請先停止它們",
		ErrStrategyScriptBotRunning, strings.Join(runningBotNames, "、"))
}

// StrategyScriptNotFound is the shared wording for both the store and the service so the refusal reads identically.
func StrategyScriptNotFound(id uint) error {
	return fmt.Errorf("%w: 找不到識別碼為 %d 的策略腳本", ErrStrategyScriptNotFound, id)
}

// ErrStrategyScriptNotPublished is internal only: callers convert it to not-found so a private script's existence isn't revealed.
var ErrStrategyScriptNotPublished = errors.New("strategy script not published")

// ErrRunSubjectAmbiguous marks a run that both named a strategy script and supplied an algorithm, or neither.
var ErrRunSubjectAmbiguous = errors.New("run subject ambiguous")

// ErrStrategyScriptMarketDataKindMismatch marks a well-formed script written for the other market data kind.
var ErrStrategyScriptMarketDataKindMismatch = errors.New("strategy script market data kind mismatch")

// ErrForeignStrategyScriptFailed marks a script failure whose wording came out of someone else's script, so
// only a person, never the assistant, may read it.
var ErrForeignStrategyScriptFailed = errors.New("foreign strategy script failed")

// foreignStrategyScriptFailureError reads exactly like its cause, so people see the same message as before.
type foreignStrategyScriptFailureError struct {
	cause error
}

func (foreignFailure *foreignStrategyScriptFailureError) Error() string {
	return foreignFailure.cause.Error()
}

func (foreignFailure *foreignStrategyScriptFailureError) Unwrap() []error {
	return []error{foreignFailure.cause, ErrForeignStrategyScriptFailed}
}

// ErrStrategyScriptUsageNotCalculation refuses a chart script where something has to be computed from it.
var ErrStrategyScriptUsageNotCalculation = errors.New("strategy script is for the chart")

// ErrStrategyScriptUsageNotRewritable refuses a usage carried by a rewrite; switching usage is its own action because marketplace copies cannot be rewritten.
var ErrStrategyScriptUsageNotRewritable = errors.New("strategy script usage is switched, not rewritten")

func StrategyScriptUsageNotRewritable() error {
	return fmt.Errorf("%w: 改寫策略腳本時不能一併改用途，請改用切換用途", ErrStrategyScriptUsageNotRewritable)
}

// ErrStrategyScriptUsageInUse refuses switching a script to the chart while trading strategies or screening profiles compute with it.
var ErrStrategyScriptUsageInUse = errors.New("strategy script usage is in use")

func StrategyScriptUsageInUse(tradingStrategyNames []string, screeningProfileNames []string) error {
	users := make([]string, 0, len(tradingStrategyNames)+len(screeningProfileNames))
	for _, tradingStrategyName := range tradingStrategyNames {
		users = append(users, fmt.Sprintf("交易策略「%s」", tradingStrategyName))
	}
	for _, screeningProfileName := range screeningProfileNames {
		users = append(users, fmt.Sprintf("篩選設定「%s」", screeningProfileName))
	}

	return fmt.Errorf("%w: %s 正在用這支策略腳本計算，不能切成圖表用",
		ErrStrategyScriptUsageInUse, strings.Join(users, "、"))
}
