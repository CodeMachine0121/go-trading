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
	// 開頭三個字就要分得出是機器人自己的動靜還是該處理的訊號。
	assert.Contains(t, aLifecycleMessage(vo.StrategyBotHaltNone).StartedText(), "【已啟動】")
	assert.Contains(t, aLifecycleMessage(vo.StrategyBotHaltNone).StoppedText(), "【已停止】")
	assert.Contains(t, aLifecycleMessage(vo.StrategyBotHaltScriptFailed).StoppedText(), "【已停擺】")
}

func TestStrategyBotLifecycleMessageDistinguishesBeingStoppedFromBreaking(t *testing.T) {
	// 擁有者手動停止無需處理；系統停下的才需要。
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
			assert.Contains(t,
				aLifecycleMessage(testCase.haltReason).StoppedText(), testCase.expectedWord)
		})
	}
}

func TestStrategyBotLifecycleMessageDoesNotRepeatTheConditionBackAtItsOwner(t *testing.T) {
	// 不重述條件：太長，且下次編輯後就過時。
	started := aLifecycleMessage(vo.StrategyBotHaltNone).StartedText()

	assert.NotContains(t, started, "買入")
	assert.NotContains(t, started, "賣出")
}
