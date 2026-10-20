package domains

import (
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// StrategyBotLifecycleMessageDomain writes a bot's started/stopped messages and a watched symbol's halt notice, each opening with its own bracket so they are distinguishable from round messages at a glance.
type StrategyBotLifecycleMessageDomain struct {
	botName    string
	symbols    []string
	market     StrategyBotMarketDomain
	haltReason vo.StrategyBotHaltReasonVo
}

// NewStrategyBotLifecycleMessageDomain takes the symbols the message speaks for: all the bot watches, or the one that halted.
func NewStrategyBotLifecycleMessageDomain(
	botName string, marketDataKind string, symbols []string, haltReason vo.StrategyBotHaltReasonVo,
) StrategyBotLifecycleMessageDomain {
	return StrategyBotLifecycleMessageDomain{
		botName: botName, symbols: symbols,
		market: NewStrategyBotMarketDomain(marketDataKind, ""), haltReason: haltReason}
}

// StartedText deliberately omits the bot's condition, which would be long and stale after the next edit.
func (messageDomain StrategyBotLifecycleMessageDomain) StartedText() string {
	if len(messageDomain.symbols) == 1 {
		return fmt.Sprintf("【已啟動】%s · %s\n它開始盯這一檔了。訊號變了才會再傳訊息給你。",
			messageDomain.botName, messageDomain.symbolsLabel())
	}

	return fmt.Sprintf("【已啟動】%s · %s\n它開始盯這 %d 檔了。哪一檔的訊號變了，才會再傳那一檔的訊息給你。",
		messageDomain.botName, messageDomain.symbolsLabel(), len(messageDomain.symbols))
}

// StoppedText includes the halt reason so a system-halted bot is noticed without opening the list.
func (messageDomain StrategyBotLifecycleMessageDomain) StoppedText() string {
	if messageDomain.haltReason != vo.StrategyBotHaltNone {
		return fmt.Sprintf("【已停擺】%s · %s\n%s\n它已經停下來了，處理好之後要再按一次啟動。",
			messageDomain.botName, messageDomain.symbolsLabel(), messageDomain.haltReasonInWords())
	}

	if len(messageDomain.symbols) == 1 {
		return fmt.Sprintf("【已停止】%s · %s\n它不再盯這一檔了。",
			messageDomain.botName, messageDomain.symbolsLabel())
	}

	return fmt.Sprintf("【已停止】%s · %s\n它不再盯這 %d 檔了。",
		messageDomain.botName, messageDomain.symbolsLabel(), len(messageDomain.symbols))
}

// WatchedSymbolHaltedText tells the owner one symbol stopped while the rest carry on.
func (messageDomain StrategyBotLifecycleMessageDomain) WatchedSymbolHaltedText() string {
	return fmt.Sprintf("【標的停擺】%s · %s\n%s\n其他標的照常。處理好之後停止再啟動這台機器人，這一檔就會恢復。",
		messageDomain.botName, messageDomain.symbolsLabel(), messageDomain.haltReasonInWords())
}

func (messageDomain StrategyBotLifecycleMessageDomain) symbolsLabel() string {
	return messageDomain.market.SymbolsLabel(messageDomain.symbols)
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
	case vo.StrategyBotHaltOrderFlowRefused:
		return "原因：它用到的某一支策略腳本要的訂單流超過一次能給的範圍（天數或價位數）。"
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
