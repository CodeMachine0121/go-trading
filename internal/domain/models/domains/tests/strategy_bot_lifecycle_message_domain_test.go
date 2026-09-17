package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

func aLifecycleMessage(haltReason vo.StrategyBotHaltReasonVo) domains.StrategyBotLifecycleMessageDomain {
	return domains.NewStrategyBotLifecycleMessageDomain("早盤突破", "BTCUSDT", haltReason)
}

func TestStrategyBotLifecycleMessageSaysWhichBotAndWhichSymbol(t *testing.T) {
	started := aLifecycleMessage(vo.StrategyBotHaltNone).StartedText()

	assert.Contains(t, started, "早盤突破")
	assert.Contains(t, started, "BTCUSDT")
}

func TestStrategyBotLifecycleMessageIsToldApartFromARoundInTheFirstCharacters(t *testing.T) {
	// 一個人瞄一眼通知列，要在前三個字就分得出「這是機器人自己的動靜」
	// 還是「這是一個該做點什麼的訊號」。
	assert.Contains(t, aLifecycleMessage(vo.StrategyBotHaltNone).StartedText(), "【已啟動】")
	assert.Contains(t, aLifecycleMessage(vo.StrategyBotHaltNone).StoppedText(), "【已停止】")
	assert.Contains(t, aLifecycleMessage(vo.StrategyBotHaltScriptFailed).StoppedText(), "【已停擺】")
}

func TestStrategyBotLifecycleMessageDistinguishesBeingStoppedFromBreaking(t *testing.T) {
	// 擁有者按下停止，沒有什麼要處理的；系統把它停下來，就有。
	stopped := aLifecycleMessage(vo.StrategyBotHaltNone).StoppedText()
	assert.NotContains(t, stopped, "原因")
	assert.NotContains(t, stopped, "再按一次啟動")

	halted := aLifecycleMessage(vo.StrategyBotHaltScriptFailed).StoppedText()
	assert.Contains(t, halted, "原因")
	assert.Contains(t, halted, "再按一次啟動")
}

func TestStrategyBotLifecycleMessageSaysWhichOfTheFiveHaltsItWas(t *testing.T) {
	testCases := []struct {
		haltReason   vo.StrategyBotHaltReasonVo
		expectedWord string
	}{
		{vo.StrategyBotHaltStrategyScriptUnavailable, "找不到了"},
		{vo.StrategyBotHaltScriptFailed, "算不出來"},
		{vo.StrategyBotHaltCredentialRejected, "金鑰不被接受"},
		{vo.StrategyBotHaltDestinationNotFound, "找不到這個聊天室"},
		{vo.StrategyBotHaltDeliveryNotConfigured, "投遞設定被移除"},
	}

	for _, testCase := range testCases {
		t.Run(string(testCase.haltReason), func(t *testing.T) {
			// 五者要做的事完全不同，所以這一句非得說得出是哪一種不可。
			assert.Contains(t,
				aLifecycleMessage(testCase.haltReason).StoppedText(), testCase.expectedWord)
		})
	}
}

func TestStrategyBotLifecycleMessageDoesNotRepeatTheConditionBackAtItsOwner(t *testing.T) {
	// 它剛寫完那個條件。重述一遍很長，而且他下次編輯之後就是錯的。
	started := aLifecycleMessage(vo.StrategyBotHaltNone).StartedText()

	assert.NotContains(t, started, "買入")
	assert.NotContains(t, started, "賣出")
}
