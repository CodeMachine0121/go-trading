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

	// Phone notifications show only the first line or two, so the deciding facts go first.
	firstLine := strings.Split(message, "\n")[0]
	assert.Equal(t, "🔴【出場】早盤突破 · BTCUSDT", firstLine)
}

// The direction must be readable from the words alone, not only from the coloured mark.
func TestStrategyBotMessageMarksTheDirectionWithoutRelyingOnTheMark(t *testing.T) {
	testCases := []struct {
		verdict           string
		expectedFirstLine string
	}{
		{verdict: string(vo.SignalBuy), expectedFirstLine: "🟢【買入】早盤突破 · BTCUSDT"},
		{verdict: string(vo.SignalSell), expectedFirstLine: "🔴【出場】早盤突破 · BTCUSDT"},
		{verdict: string(vo.SignalHold), expectedFirstLine: "⚪【持有】早盤突破 · BTCUSDT"},
		// Unrecognised verdicts get no colour.
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

// A source reading 持有 under a 買入 headline must be labelled as the working so it is not
// mistaken for the answer.
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
	assert.Contains(t, message, "均線黃金交叉（1h）：賣出")
	assert.Contains(t, message, "動能背離（5m）：賣出")
	assert.Contains(t, message, "量能異常（5m）：持有")
}

func TestStrategyBotMessageNeverCallsTheReferencePriceAFill(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(aBotRound()).Text()

	// It is labelled a close price, not an entry price, because nothing is traded.
	assert.Contains(t, message, "收盤價")
	assert.NotContains(t, message, "成交價")
	assert.NotContains(t, message, "進場價")
}

func TestStrategyBotMessageSaysSoWhenThereIsNoPriceToQuote(t *testing.T) {
	botRound := aBotRound()
	botRound.HasReference = false

	message := domains.NewStrategyBotMessageDomain(botRound).Text()

	// An empty slot would read as a price of nothing.
	assert.Contains(t, message, "讀不到這個交易標的的最新 K 線")
	assert.NotContains(t, message, "64180.5")
}

func TestStrategyBotMessageWritesEachSignalTheWayAPersonReadsIt(t *testing.T) {
	testCases := []struct {
		verdict      string
		expectedWord string
	}{
		{verdict: string(vo.SignalBuy), expectedWord: "【買入】"},
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

// A spot sell headline says 出場 because a flat reader cannot sell; source lines still use 買入／賣出／持有.
func TestStrategyBotMessageTellsASpotAccountToGetOutRatherThanToSell(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(aBotRound()).Text()

	assert.Contains(t, message, "【出場】")
	assert.NotContains(t, message, "【賣出】")
	assert.Contains(t, message, "均線黃金交叉（1h）：賣出")
}

func TestStrategyBotMessageWritesAnUnrecognisedSignalOutAsItStands(t *testing.T) {
	botRound := aBotRound()
	botRound.Verdict = "shrug"

	// Unrecognised verdicts are shown as-is, never mapped onto a known one.
	assert.Contains(t, domains.NewStrategyBotMessageDomain(botRound).Text(), "【shrug】")
}

// aSuggestedPosition stakes 5000 with exits at 62255.085 and 67389.525.
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

func suggestingBotRound() dto.StrategyBotRoundDto {
	botRound := aBotRound()
	botRound.Verdict = string(vo.SignalBuy)
	botRound.PositionPlan = aSuggestedPosition()
	botRound.HasPositionPlan = true

	return botRound
}

// Figures are asserted as literal strings so the test does not restate the formula.
func TestStrategyBotMessageSaysWhatToPutDownAndWhereToGetOut(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(suggestingBotRound()).Text()

	assert.Contains(t, message, "📐 建議部位")
	assert.Contains(t, message, "開倉金額 5000")
	assert.Contains(t, message, "止損 62255.085（往下，虧 450）")
	assert.Contains(t, message, "止盈 67389.525（往上，賺 750）")
}

// The stop is always below and the target above, since a suggestion only faces one way.
func TestStrategyBotMessageSaysWhichWayEachExitLies(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(suggestingBotRound()).Text()

	assert.Contains(t, message, "止損 62255.085（往下，虧 450）")
	assert.Contains(t, message, "止盈 67389.525（往上，賺 750）")
}

// Removed lines about trading mode and borrowing are asserted absent, since deleting their
// assertions alone would not catch them coming back.
func TestStrategyBotMessageSaysNothingAboutModesOrBorrowing(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(suggestingBotRound()).Text()

	for _, goneWording := range []string{"交易模式", "槓桿", "名目", "強制平倉", "維持保證金"} {
		assert.NotContains(t, message, goneWording)
	}

	// Guards against the absences passing because the message emptied out.
	assert.Contains(t, message, "📐 建議部位（這個系統不下單）")
	assert.Contains(t, message, "開倉金額 5000")
	assert.Contains(t, message, "📊 各來源怎麼說")
}

// The message must say no order is placed and that report cards were produced without these exits.
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
			expectedPresent: []string{"開倉金額 5000"},
			expectedAbsent:  []string{"・止損", "・止盈", "回測要算進止損止盈，重演時把這兩個距離填上"},
		},
		{
			name: "a stake the capital cannot cover",
			adjust: func(positionPlan *dto.PositionPlanDto) {
				positionPlan.Affordable = false
				positionPlan.Stake = decimal.NewFromInt(8000)
			},
			// Shown as a shortfall, not a plain figure, so nobody places it.
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

// Rounds with nothing to suggest produce the same message as before suggestions existed.
func TestStrategyBotMessageStaysSilentWhenThereIsNothingToSuggest(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(aBotRound()).Text()

	assert.NotContains(t, message, "建議部位")
	assert.NotContains(t, message, "開倉金額")
	assert.NotContains(t, message, "・止損")
	assert.NotContains(t, message, "回測要算進止損止盈，重演時把這兩個距離填上")
}

// The suggestion precedes the working because readers skim to decide whether to act.
func TestStrategyBotMessagePutsTheSuggestionBeforeTheWorking(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(suggestingBotRound()).Text()

	assert.Less(t,
		strings.Index(message, "📐 建議部位"), strings.Index(message, "📊 各來源怎麼說"))
	assert.Less(t, strings.Index(message, "💰 參考價"), strings.Index(message, "📐 建議部位"))
}
