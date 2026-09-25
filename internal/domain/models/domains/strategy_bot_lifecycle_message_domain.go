package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyBotLifecycleMessageDomain writes a bot's started/stopped messages, using a different opening bracket than round messages so they are distinguishable at a glance.
type StrategyBotLifecycleMessageDomain struct {
	botName    string
	symbol     string
	haltReason vo.StrategyBotHaltReasonVo
}

// NewStrategyBotLifecycleMessageDomain expects the owner-facing symbol label (a contract bot's already says perpetual contract).
func NewStrategyBotLifecycleMessageDomain(
	botName string, symbol string, haltReason vo.StrategyBotHaltReasonVo,
) StrategyBotLifecycleMessageDomain {
	return StrategyBotLifecycleMessageDomain{
		botName: botName, symbol: symbol, haltReason: haltReason}
}

// StartedText deliberately omits the bot's condition, which would be long and stale after the next edit.
func (messageDomain StrategyBotLifecycleMessageDomain) StartedText() string {
	return fmt.Sprintf("【已啟動】%s · %s\n它開始盯這一檔了。訊號變了才會再傳訊息給你。",
		messageDomain.botName, messageDomain.symbol)
}

// StoppedText includes the halt reason so a system-halted bot is noticed without opening the list.
func (messageDomain StrategyBotLifecycleMessageDomain) StoppedText() string {
	if messageDomain.haltReason == vo.StrategyBotHaltNone {
		return fmt.Sprintf("【已停止】%s · %s\n它不再盯這一檔了。",
			messageDomain.botName, messageDomain.symbol)
	}

	return fmt.Sprintf("【已停擺】%s · %s\n%s\n它已經停下來了，處理好之後要再按一次啟動。",
		messageDomain.botName, messageDomain.symbol, messageDomain.haltReasonInWords())
}

// haltReasonInWords says what happened, not what to do, since only the owner knows which script or setting to fix.
func (messageDomain StrategyBotLifecycleMessageDomain) haltReasonInWords() string {
	switch messageDomain.haltReason {
	case vo.StrategyBotHaltStrategyScriptUnavailable:
		return "原因：它用到的某一支策略腳本找不到了。"
	case vo.StrategyBotHaltTradingStrategyUnavailable:
		return "原因：它用的那一份交易策略找不到了。"
	case vo.StrategyBotHaltScriptFailed:
		return "原因：它用到的某一支策略腳本算不出來。"
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
