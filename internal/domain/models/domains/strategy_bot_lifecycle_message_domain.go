package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyBotLifecycleMessageDomain writes the two messages a bot sends about
// itself: it has started watching, or it has stopped.
//
// They exist because pressing play and then walking away is the whole point of a
// bot, and until now the first thing that ever confirmed the walking-away worked was
// a trading signal that might be hours off. A line saying "I am watching" turns a
// button press into something that happened.
//
// It is a model of its own rather than another method on the round message, because
// the two say different kinds of thing: one is about the market, the other is about
// the bot. A reader glancing at their phone has to be able to tell them apart in the
// first three characters, which is why these open with a different bracket.
type StrategyBotLifecycleMessageDomain struct {
	botName    string
	symbol     string
	haltReason vo.StrategyBotHaltReasonVo
}

// NewStrategyBotLifecycleMessageDomain takes the bot the message is about, and why
// it stopped if the system stopped it.
func NewStrategyBotLifecycleMessageDomain(
	botName string, symbol string, haltReason vo.StrategyBotHaltReasonVo,
) StrategyBotLifecycleMessageDomain {
	return StrategyBotLifecycleMessageDomain{
		botName: botName, symbol: symbol, haltReason: haltReason}
}

// StartedText is what a bot says when it begins watching.
//
// It says what it is watching and not what it will do, because what it will do is
// the condition its owner just wrote — repeating it back would be long, and wrong
// the moment they edit it.
func (messageDomain StrategyBotLifecycleMessageDomain) StartedText() string {
	return fmt.Sprintf("【已啟動】%s · %s\n它開始盯這一檔了。訊號變了才會再傳訊息給你。",
		messageDomain.botName, messageDomain.symbol)
}

// StoppedText is what a bot says when it stops.
//
// A halt says why. That is the whole reason this message is worth sending at all:
// until now the only place a halted bot could be noticed was the list, and nobody
// opens a list to check on something they believe is running.
func (messageDomain StrategyBotLifecycleMessageDomain) StoppedText() string {
	if messageDomain.haltReason == vo.StrategyBotHaltNone {
		return fmt.Sprintf("【已停止】%s · %s\n它不再盯這一檔了。",
			messageDomain.botName, messageDomain.symbol)
	}

	return fmt.Sprintf("【已停擺】%s · %s\n%s\n它已經停下來了，處理好之後要再按一次啟動。",
		messageDomain.botName, messageDomain.symbol, messageDomain.haltReasonInWords())
}

// haltReasonInWords is the reason as a person reads it. It says what happened, not
// what to do: what to do depends on which of their strategies or settings it was,
// and only they know that.
func (messageDomain StrategyBotLifecycleMessageDomain) haltReasonInWords() string {
	switch messageDomain.haltReason {
	case vo.StrategyBotHaltStrategyUnavailable:
		return "原因：它用到的某一支策略找不到了。"
	case vo.StrategyBotHaltScriptFailed:
		return "原因：它用到的某一支策略算不出來。"
	case vo.StrategyBotHaltCredentialRejected:
		return "原因：機器人金鑰不被接受。"
	case vo.StrategyBotHaltDestinationNotFound:
		return "原因：找不到這個聊天室。"
	case vo.StrategyBotHaltDeliveryNotConfigured:
		return "原因：Telegram 投遞設定被移除了。"
	default:
		return "原因：系統把它停下來了。"
	}
}
