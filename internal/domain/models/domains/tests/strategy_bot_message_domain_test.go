package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
)

func aBotRound() dto.StrategyBotRoundDto {
	return dto.StrategyBotRoundDto{
		BotName:        "早盤突破",
		Symbol:         "BTCUSDT",
		Verdict:        string(vo.SignalSell),
		ReferencePrice: decimal.RequireFromString("64180.50"),
		ReferenceTime:  time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC),
		HasReference:   true,
		SourceSignals: []dto.StrategyBotSourceSignalDto{
			{Label: "均線黃金交叉", AggregationInterval: "1h", Signal: string(vo.SignalSell)},
			{Label: "動能背離", AggregationInterval: "5m", Signal: string(vo.SignalSell)},
			{Label: "量能異常", AggregationInterval: "5m", Signal: string(vo.SignalHold)},
		},
	}
}

func TestStrategyBotMessageSaysTheSignalTheBotAndTheSymbolFirst(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(aBotRound()).Text()

	// A phone's notification list shows the first line and maybe the second, so the
	// three things that decide whether to open it go first.
	firstLine := message[:len("【賣出】早盤突破 · BTCUSDT")]
	assert.Equal(t, "【賣出】早盤突破 · BTCUSDT", firstLine)
}

func TestStrategyBotMessageCarriesEverythingAReaderNeedsToJudgeIt(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(aBotRound()).Text()

	assert.Contains(t, message, "早盤突破")
	assert.Contains(t, message, "BTCUSDT")
	assert.Contains(t, message, "賣出")
	assert.Contains(t, message, "64180.5")
	assert.Contains(t, message, "2026-09-16 13:00 UTC")
	// Without the per-source lines, somebody reading "sell" has only the option of
	// believing it.
	assert.Contains(t, message, "均線黃金交叉（1h）：賣出")
	assert.Contains(t, message, "動能背離（5m）：賣出")
	assert.Contains(t, message, "量能異常（5m）：持有")
}

func TestStrategyBotMessageNeverCallsTheReferencePriceAFill(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(aBotRound()).Text()

	// Nothing here buys anything. Calling it an entry price would have somebody
	// believing a trade was placed.
	assert.Contains(t, message, "收盤價")
	assert.NotContains(t, message, "成交價")
	assert.NotContains(t, message, "進場價")
}

func TestStrategyBotMessageSaysSoWhenThereIsNoPriceToQuote(t *testing.T) {
	botRound := aBotRound()
	botRound.HasReference = false

	message := domains.NewStrategyBotMessageDomain(botRound).Text()

	// Silence in that slot would read as a price of nothing.
	assert.Contains(t, message, "讀不到這個交易標的的最新 K 線")
	assert.NotContains(t, message, "64180.5")
}

func TestStrategyBotMessageWritesEachSignalTheWayAPersonReadsIt(t *testing.T) {
	testCases := []struct {
		verdict      string
		expectedWord string
	}{
		{verdict: string(vo.SignalBuy), expectedWord: "【買入】"},
		{verdict: string(vo.SignalSell), expectedWord: "【賣出】"},
		{verdict: string(vo.SignalHold), expectedWord: "【持有】"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.verdict, func(t *testing.T) {
			botRound := aBotRound()
			botRound.Verdict = testCase.verdict

			assert.Contains(t,
				domains.NewStrategyBotMessageDomain(botRound).Text(), testCase.expectedWord)
		})
	}
}

func TestStrategyBotMessageWritesAnUnrecognisedSignalOutAsItStands(t *testing.T) {
	botRound := aBotRound()
	botRound.Verdict = "shrug"

	// A message is the last place to quietly turn something the system did not
	// understand into one of the three things it did.
	assert.Contains(t, domains.NewStrategyBotMessageDomain(botRound).Text(), "【shrug】")
}
