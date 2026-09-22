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

// aBotRound is one round of a bot, which is the shape every assertion in this file is
// read in. There is one set of rules this system replays, so there is one wording.
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
	assert.Equal(t, "🔴【出場】早盤突破 · BTCUSDT", firstLine)
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
		{verdict: string(vo.SignalSell), expectedFirstLine: "🔴【出場】早盤突破 · BTCUSDT"},
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
	assert.Contains(t, message, "出場")
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
		// A spot sell asks for nothing to be held, so the act is getting out — not
		// selling, which is what a reader already holding nothing cannot go and do.
		{verdict: string(vo.SignalSell), expectedWord: "【出場】"},
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

// A conclusion is read as an instruction, and 賣出 is one a reader holding nothing
// cannot carry out — a spot sell reaches them just as often while they are flat,
// because the signal does not know what they hold. 出場 is the one wording both
// readers can act on: close what is open, or find there was nothing to close.
//
// The source lines are untouched, and that is the point of asserting both here: the
// scripts still testify in 買入／賣出／持有, and only the headline speaks of acts.
func TestStrategyBotMessageTellsASpotAccountToGetOutRatherThanToSell(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(aBotRound()).Text()

	assert.Contains(t, message, "【出場】")
	assert.NotContains(t, message, "【賣出】")
	assert.Contains(t, message, "均線黃金交叉（1h）：賣出")
}

func TestStrategyBotMessageWritesAnUnrecognisedSignalOutAsItStands(t *testing.T) {
	botRound := aBotRound()
	botRound.Verdict = "shrug"

	// A message is the last place to quietly turn something the system did not
	// understand into one of the three things it did.
	assert.Contains(t, domains.NewStrategyBotMessageDomain(botRound).Text(), "【shrug】")
}

// aSuggestedPosition is what a round came to suggest: five thousand down, out at
// 62255.085 or 67389.525.
func aSuggestedPosition() dto.PositionPlanDto {
	return dto.PositionPlanDto{
		Stake:           decimal.NewFromInt(5000),
		Affordable:      true,
		StopLossPrice:   decimal.RequireFromString("62255.085"),
		LossAtStop:      decimal.NewFromInt(450),
		HasStopLoss:     true,
		TakeProfitPrice: decimal.RequireFromString("67389.525"),
		GainAtTarget:    decimal.NewFromInt(750),
		HasTakeProfit:   true,
	}
}

// suggestingBotRound is a round that suggests opening, which is the shape every figure
// below is read in.
func suggestingBotRound() dto.StrategyBotRoundDto {
	botRound := aBotRound()
	botRound.Verdict = string(vo.SignalBuy)
	botRound.PositionPlan = aSuggestedPosition()
	botRound.HasPositionPlan = true

	return botRound
}

// The four figures its reader would otherwise work out on a phone while doing
// something else. They are asserted as strings rather than described, because the
// arithmetic is the whole point and a formula restated here would agree with itself.
func TestStrategyBotMessageSaysWhatToPutDownAndWhereToGetOut(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(suggestingBotRound()).Text()

	assert.Contains(t, message, "📐 建議部位")
	assert.Contains(t, message, "開倉金額 5000")
	assert.Contains(t, message, "止損 62255.085（往下，虧 450）")
	assert.Contains(t, message, "止盈 67389.525（往上，賺 750）")
}

// Which way each exit lies is written out, never left for the reader: 62255.085 reads
// like a perfectly ordinary price whichever side it was meant for. The stop is always
// the one below and the target always the one above, because a suggested position only
// ever faces one way.
func TestStrategyBotMessageSaysWhichWayEachExitLies(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(suggestingBotRound()).Text()

	assert.Contains(t, message, "止損 62255.085（往下，虧 450）")
	assert.Contains(t, message, "止盈 67389.525（往上，賺 750）")
}

// The two things this suggestion owes its reader. Nothing here places an order, and
// every report card they have ever seen was produced without these exits — so a
// strategy that looks profitable there has never been measured with the stop this
// message is asking them to place.
func TestStrategyBotMessageAdmitsWhatTheSuggestionIsNot(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(suggestingBotRound()).Text()

	assert.Contains(t, message, "這個系統不下單")
	assert.Contains(t, message, "回測要算進止損止盈，重演時把這兩個距離填上")
}

func TestStrategyBotMessagePrintsOnlyTheFiguresItWasGiven(t *testing.T) {
	testCases := []struct {
		name            string
		adjust          func(positionPlan *dto.PositionPlanDto)
		expectedPresent []string
		expectedAbsent  []string
	}{
		{
			name: "a stop on its own",
			adjust: func(positionPlan *dto.PositionPlanDto) {
				positionPlan.HasTakeProfit = false
			},
			expectedPresent: []string{"・止損 62255.085", "回測要算進止損止盈，重演時把這兩個距離填上"},
			// The bullet, not the word: the warning line names both exits.
			expectedAbsent: []string{"・止盈"},
		},
		{
			name: "a target on its own",
			adjust: func(positionPlan *dto.PositionPlanDto) {
				positionPlan.HasStopLoss = false
			},
			expectedPresent: []string{"・止盈 67389.525", "回測要算進止損止盈，重演時把這兩個距離填上"},
			expectedAbsent:  []string{"・止損"},
		},
		{
			name: "a size with neither exit",
			adjust: func(positionPlan *dto.PositionPlanDto) {
				positionPlan.HasStopLoss = false
				positionPlan.HasTakeProfit = false
			},
			// Nothing to warn about when no exit was suggested at all.
			expectedPresent: []string{"開倉金額 5000"},
			expectedAbsent:  []string{"・止損", "・止盈", "回測要算進止損止盈，重演時把這兩個距離填上"},
		},
		{
			name: "a stake the capital cannot cover",
			adjust: func(positionPlan *dto.PositionPlanDto) {
				positionPlan.Affordable = false
				positionPlan.Stake = decimal.NewFromInt(8000)
			},
			// Named, not printed plain: printed plain, somebody would place it.
			expectedPresent: []string{"部位資金不足，押不下 8000"},
			expectedAbsent:  []string{"開倉金額 8000", "・止損", "・止盈"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			botRound := suggestingBotRound()
			testCase.adjust(&botRound.PositionPlan)

			message := domains.NewStrategyBotMessageDomain(botRound).Text()

			for _, expected := range testCase.expectedPresent {
				assert.Contains(t, message, expected)
			}

			for _, absent := range testCase.expectedAbsent {
				assert.NotContains(t, message, absent)
			}
		})
	}
}

// A round with nothing to suggest says nothing about it — and the message it sends is
// the one it sent before suggestions existed. Every other assertion in this file is
// written against exactly that round, and not one of them changed.
func TestStrategyBotMessageStaysSilentWhenThereIsNothingToSuggest(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(aBotRound()).Text()

	assert.NotContains(t, message, "建議部位")
	assert.NotContains(t, message, "開倉金額")
	assert.NotContains(t, message, "・止損")
	assert.NotContains(t, message, "回測要算進止損止盈，重演時把這兩個距離填上")
}

// What to do comes before the working: somebody skimming this is deciding whether to
// act, and the evidence is for whoever then wants to check.
func TestStrategyBotMessagePutsTheSuggestionBeforeTheWorking(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(suggestingBotRound()).Text()

	assert.Less(t,
		strings.Index(message, "📐 建議部位"), strings.Index(message, "📊 各來源怎麼說"))
	assert.Less(t, strings.Index(message, "💰 參考價"), strings.Index(message, "📐 建議部位"))
}
