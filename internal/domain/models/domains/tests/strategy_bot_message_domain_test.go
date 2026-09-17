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

// aBotRound is one round of a bot following rules written for an account that cannot
// short.
//
// Spot rather than the default, because the assertions in this file are what hold a
// spot message's wording to the letter. Every one of them was written before a mode
// was a thing, and not one of them changed — which is the whole of what "unchanged"
// means here. What a shortable account is told is asserted on its own below.
func aBotRound() dto.StrategyBotRoundDto {
	return dto.StrategyBotRoundDto{
		BotName:        "早盤突破",
		Symbol:         "BTCUSDT",
		Verdict:        string(vo.SignalSell),
		TradingMode:    string(vo.TradingModeSpot),
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

// shortableBotRound is the same round under rules that can short. The market did not
// change; what its reader has to go and do did.
func shortableBotRound() dto.StrategyBotRoundDto {
	botRound := aBotRound()
	botRound.TradingMode = string(vo.TradingModeLongShort)

	return botRound
}

// A conclusion is read as an instruction. Somebody whose account can short and is told
// 賣出 has to translate it themselves — and the round they are most likely to misread
// is the one they are reading on a phone while doing something else.
func TestStrategyBotMessageTellsAShortableAccountTheActRatherThanTheSignal(t *testing.T) {
	testCases := []struct {
		verdict           string
		expectedFirstLine string
	}{
		{verdict: string(vo.SignalBuy), expectedFirstLine: "🟢【做多】早盤突破 · BTCUSDT"},
		{verdict: string(vo.SignalSell), expectedFirstLine: "🔴【做空】早盤突破 · BTCUSDT"},
		// Nothing to do is nothing to do, whichever account is reading.
		{verdict: string(vo.SignalHold), expectedFirstLine: "⚪【持有】早盤突破 · BTCUSDT"},
		// A message is the last place to guess which way somebody should trade, so an
		// opinion nobody can read gets no direction invented for it.
		{verdict: "shrug", expectedFirstLine: "⚪【shrug】早盤突破 · BTCUSDT"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.verdict, func(t *testing.T) {
			botRound := shortableBotRound()
			botRound.Verdict = testCase.verdict

			message := domains.NewStrategyBotMessageDomain(botRound).Text()

			assert.Equal(t, testCase.expectedFirstLine, strings.Split(message, "\n")[0])
		})
	}
}

// The source lines are the strategy scripts' own testimony, and a script only ever
// says buy, sell or hold. Rewriting them as 做多／做空 would put words in their mouths
// and leave nobody able to work back from the conclusion to what produced it — which
// is the only reason those lines are in the message at all.
func TestStrategyBotMessageLeavesTheSourceLinesSpeakingInSignals(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(shortableBotRound()).Text()

	assert.Contains(t, message, "【做空】")
	assert.Contains(t, message, "均線黃金交叉（1h）：賣出")
	assert.Contains(t, message, "量能異常（5m）：持有")
	assert.NotContains(t, message, "均線黃金交叉（1h）：做空")
}

// Named only when the headline has stopped quoting the signal, which is the one case a
// reader needs it: they are being told to open a short, and which account these rules
// were written for is what makes that the right act.
func TestStrategyBotMessageNamesTheModeOnlyWhenItRestatesTheSignal(t *testing.T) {
	assert.Contains(t,
		domains.NewStrategyBotMessageDomain(shortableBotRound()).Text(), "⚙️ 交易模式 多空反手")
	assert.NotContains(t,
		domains.NewStrategyBotMessageDomain(aBotRound()).Text(), "交易模式")
}

// Rules stored before a mode was a thing read as the one they were actually replayed
// under, so a bot following them words its message that way too.
func TestStrategyBotMessageWordsARoundWithNoStatedModeAsShortable(t *testing.T) {
	botRound := aBotRound()
	botRound.TradingMode = ""

	assert.Contains(t, domains.NewStrategyBotMessageDomain(botRound).Text(), "【做空】")
}

// A mode nothing can read leaves the message quoting the signal and naming nobody. It
// is unreachable through the save gate, which refuses such a mode before it is ever
// stored — written down because the alternative to a rule is an accident.
func TestStrategyBotMessageQuotesTheSignalWhenItCannotReadTheMode(t *testing.T) {
	botRound := aBotRound()
	botRound.TradingMode = "dayTrade"

	message := domains.NewStrategyBotMessageDomain(botRound).Text()

	assert.Contains(t, message, "【賣出】")
	assert.NotContains(t, message, "交易模式")
}

// Only the act changed, not the market. A price that read differently under one mode
// would mean the two messages disagreed about something neither of them decides.
func TestStrategyBotMessageQuotesThePriceIdenticallyUnderEitherMode(t *testing.T) {
	spotMessage := domains.NewStrategyBotMessageDomain(aBotRound()).Text()
	shortableMessage := domains.NewStrategyBotMessageDomain(shortableBotRound()).Text()

	for _, message := range []string{spotMessage, shortableMessage} {
		assert.Contains(t, message, "💰 參考價 64180.5")
		assert.Contains(t, message, "2026-09-16 13:00 UTC 那一根一分鐘 K 線的收盤價")
	}

	spotWithoutPrice := aBotRound()
	spotWithoutPrice.HasReference = false
	shortableWithoutPrice := shortableBotRound()
	shortableWithoutPrice.HasReference = false

	// The same sentence word for word, so nothing downstream has to write two.
	assert.Contains(t,
		domains.NewStrategyBotMessageDomain(spotWithoutPrice).Text(),
		"💰 參考價 目前讀不到這個交易標的的最新 K 線")
	assert.Contains(t,
		domains.NewStrategyBotMessageDomain(shortableWithoutPrice).Text(),
		"💰 參考價 目前讀不到這個交易標的的最新 K 線")
}
