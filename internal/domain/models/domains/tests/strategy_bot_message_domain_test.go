package domains_test

import (
	"strings"
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
	firstLine := strings.Split(message, "\n")[0]
	assert.Equal(t, "🔴【賣出】早盤突破 · BTCUSDT", firstLine)
}

// The mark is there to be skimmed, so what it must never do is be the only thing
// saying which way this went — a reader who cannot see colour, or whose device draws
// these differently, reads the words.
func TestStrategyBotMessageMarksTheDirectionWithoutRelyingOnTheMark(t *testing.T) {
	testCases := []struct {
		verdict           string
		expectedFirstLine string
	}{
		{verdict: string(vo.SignalBuy), expectedFirstLine: "🟢【買入】早盤突破 · BTCUSDT"},
		{verdict: string(vo.SignalSell), expectedFirstLine: "🔴【賣出】早盤突破 · BTCUSDT"},
		{verdict: string(vo.SignalHold), expectedFirstLine: "⚪【持有】早盤突破 · BTCUSDT"},
		// Nothing the system recognised, so nothing it is willing to colour.
		{verdict: "shrug", expectedFirstLine: "⚪【shrug】早盤突破 · BTCUSDT"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.verdict, func(t *testing.T) {
			botRound := aBotRound()
			botRound.Verdict = testCase.verdict

			message := domains.NewStrategyBotMessageDomain(botRound).Text()

			assert.Equal(t, testCase.expectedFirstLine, strings.Split(message, "\n")[0])
		})
	}
}

// The failure this whole layout exists to stop: a source reading 持有 under a
// headline reading 買入 is the working, not a contradiction, and without a label
// saying so the last line a reader's eye lands on looks like the answer.
func TestStrategyBotMessageSaysOutLoudThatTheSourceLinesAreTheWorking(t *testing.T) {
	botRound := aBotRound()
	botRound.Verdict = string(vo.SignalBuy)
	botRound.SourceSignals = []dto.StrategyBotSourceSignalDto{
		{Label: "A", AggregationInterval: "1m", Signal: string(vo.SignalHold)},
	}

	message := domains.NewStrategyBotMessageDomain(botRound).Text()

	headlineIndex := strings.Index(message, "【買入】")
	labelIndex := strings.Index(message, "各來源怎麼說")
	sourceIndex := strings.Index(message, "A（1m）：持有")

	assert.NotEqual(t, -1, labelIndex, "沒有那句標題的話，底下那行看起來就是答案")
	assert.Less(t, headlineIndex, labelIndex, "結論在最前面")
	assert.Less(t, labelIndex, sourceIndex, "標題要在它說明的那幾行之前")
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
