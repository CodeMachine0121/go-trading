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

// Named wherever the headline's verb does not already say everything the act
// involves, and nowhere else.
//
// Cash for goods is the only mode where it does: 買入 there is handing money over for
// a thing, full stop. Shorting rules may be asking the reader to open rather than
// close. Borrowing rules turn that same 買入 into a position held on somebody else's
// money — same three characters, a risk an order of magnitude apart.
func TestStrategyBotMessageNamesTheModeWhereTheVerbDoesNotSayEverything(t *testing.T) {
	assert.Contains(t,
		domains.NewStrategyBotMessageDomain(shortableBotRound()).Text(), "⚙️ 交易模式 多空反手")
	assert.Contains(t,
		domains.NewStrategyBotMessageDomain(borrowingBotRound()).Text(), "⚙️ 交易模式 槓桿做多")
	assert.NotContains(t,
		domains.NewStrategyBotMessageDomain(aBotRound()).Text(), "交易模式")
}

// borrowingBotRound is one round of a bot following rules that never short and may
// borrow — the same sell the spot round carries, so that what differs between the two
// messages is only what the mode changes.
func borrowingBotRound() dto.StrategyBotRoundDto {
	botRound := aBotRound()
	botRound.TradingMode = string(vo.TradingModeLeveragedLong)

	return botRound
}

// A reader who cannot short is told the same two words whether or not the position is
// borrowed, because borrowing does not give them a sell they can carry out.
//
// This is the half that must not follow the mode line: it would have been easy to
// word the borrowing modes together and hand somebody 做空 on rules that never short.
func TestStrategyBotMessageStillTellsABorrowingLongAccountToGetOut(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(borrowingBotRound()).Text()

	assert.Contains(t, message, "【出場】")
	assert.NotContains(t, message, "【賣出】")
	assert.NotContains(t, message, "【做空】")
	// And the scripts still testify in signals, as under every other mode.
	assert.Contains(t, message, "均線黃金交叉（1h）：賣出")
}

func TestStrategyBotMessageTellsABorrowingLongAccountToBuyRatherThanGoLong(t *testing.T) {
	botRound := borrowingBotRound()
	botRound.Verdict = string(vo.SignalBuy)

	message := domains.NewStrategyBotMessageDomain(botRound).Text()

	assert.Contains(t, message, "【買入】")
	assert.NotContains(t, message, "【做多】")
}

// The mode is a block of its own, like everything else under the headline. Glued to
// the reference moment it reads as one paragraph with it — as though the mode were
// something about that price rather than about the rules that judged the round.
func TestStrategyBotMessageGivesTheModeLineItsOwnBlock(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(shortableBotRound()).Text()

	assert.Contains(t, message, "\n\n⚙️ 交易模式 多空反手")
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

// aSuggestedPosition is what a round came to suggest: five thousand down, fifteen
// thousand at risk, out at 62255.085 or 67389.525.
func aSuggestedPosition() dto.PositionPlanDto {
	return dto.PositionPlanDto{
		Stake:           decimal.NewFromInt(5000),
		Affordable:      true,
		Notional:        decimal.NewFromInt(15000),
		Leveraged:       true,
		StopLossPrice:   decimal.RequireFromString("62255.085"),
		LossAtStop:      decimal.NewFromInt(450),
		HasStopLoss:     true,
		TakeProfitPrice: decimal.RequireFromString("67389.525"),
		GainAtTarget:    decimal.NewFromInt(750),
		HasTakeProfit:   true,
	}
}

// suggestingBotRound is a round that suggests a long position under rules that can
// short, which is the shape every figure below is read in.
func suggestingBotRound() dto.StrategyBotRoundDto {
	botRound := shortableBotRound()
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
	assert.Contains(t, message, "保證金 5000")
	assert.Contains(t, message, "名目 15000")
	assert.Contains(t, message, "止損 62255.085（往下，虧 450）")
	assert.Contains(t, message, "止盈 67389.525（往上，賺 750）")
}

// Which way each exit lies is written out, never left for the reader. A short's stop
// sits above the price, and 66105.915 reads like a perfectly ordinary price whichever
// side it was meant for.
func TestStrategyBotMessageSaysWhichWayEachExitLies(t *testing.T) {
	botRound := suggestingBotRound()
	botRound.Verdict = string(vo.SignalSell)
	botRound.PositionPlan.SuggestsShort = true
	botRound.PositionPlan.StopLossPrice = decimal.RequireFromString("66105.915")
	botRound.PositionPlan.TakeProfitPrice = decimal.RequireFromString("60971.475")

	message := domains.NewStrategyBotMessageDomain(botRound).Text()

	assert.Contains(t, message, "止損 66105.915（往上，虧 450）")
	assert.Contains(t, message, "止盈 60971.475（往下，賺 750）")
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
			name: "no leverage leaves out the notional",
			adjust: func(positionPlan *dto.PositionPlanDto) {
				positionPlan.Leveraged = false
			},
			// Repeating the stake under a second label would read as a mistake.
			expectedPresent: []string{"保證金 5000"},
			expectedAbsent:  []string{"名目"},
		},
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
			expectedPresent: []string{"保證金 5000"},
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
			expectedAbsent:  []string{"保證金 8000", "名目", "・止損", "・止盈"},
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
	assert.NotContains(t, message, "保證金")
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

// shortOnlyBotRound is one round of a bot following rules that only ever short — the
// same round the others carry, so that what differs between the messages is only what
// the mode changes.
func shortOnlyBotRound() dto.StrategyBotRoundDto {
	botRound := aBotRound()
	botRound.TradingMode = string(vo.TradingModeShortOnly)

	return botRound
}

// The mode where the two halves of the verb rule finally disagree.
//
// Every mode before it could go long, so "cannot short" was enough to decide both
// words. This one shorts and cannot go long, and reading only the first half hands
// its reader 做多 — an act their account will never carry out, in a message that
// looks entirely ordinary.
func TestStrategyBotMessageTellsAShortOnlyAccountToGetOutRatherThanGoLong(t *testing.T) {
	testCases := []struct {
		verdict           string
		expectedFirstLine string
	}{
		// Not 做多: these rules never hold one. Not 買入 either — that reaches them
		// just as often while they hold nothing to close.
		// Red rather than green. Green is this system's colour for going long, and
		// these rules never do — the reader who glances at the mark alone must not
		// come away having read an entry.
		{verdict: string(vo.SignalBuy), expectedFirstLine: "🔴【出場】早盤突破 · BTCUSDT"},
		// The same act long-short names, because at this moment it is the same act.
		{verdict: string(vo.SignalSell), expectedFirstLine: "🔴【做空】早盤突破 · BTCUSDT"},
		{verdict: string(vo.SignalHold), expectedFirstLine: "⚪【持有】早盤突破 · BTCUSDT"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.verdict, func(t *testing.T) {
			botRound := shortOnlyBotRound()
			botRound.Verdict = testCase.verdict

			firstLine := strings.Split(domains.NewStrategyBotMessageDomain(botRound).Text(), "\n")[0]

			assert.Equal(t, testCase.expectedFirstLine, firstLine)
		})
	}
}

// It borrows, and borrowing is what earns a mode this line: the position can be taken
// away from its holder at a price, and nothing in 做空 says so.
func TestStrategyBotMessageNamesTheShortOnlyMode(t *testing.T) {
	assert.Contains(t,
		domains.NewStrategyBotMessageDomain(shortOnlyBotRound()).Text(), "⚙️ 交易模式 只做空")
}

// The scripts still testify in their own three words. Only the conclusion speaks of
// acts — otherwise a reader cannot work back from it to what produced it.
func TestStrategyBotMessageQuotesTheScriptsUnderShortOnlyRules(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(shortOnlyBotRound()).Text()

	assert.Contains(t, message, "【做空】")
	assert.Contains(t, message, "：賣出")
	assert.NotContains(t, message, "：做空")
}

// suggestingShortOnlyBotRound is a round that suggests a short under rules that can
// only ever take one, and does so without borrowing — the pair of answers no other
// mode produces.
func suggestingShortOnlyBotRound() dto.StrategyBotRoundDto {
	botRound := shortOnlyBotRound()
	botRound.Verdict = string(vo.SignalSell)
	botRound.PositionPlan = aSuggestedPosition()
	botRound.PositionPlan.Leveraged = false
	botRound.PositionPlan.SuggestsShort = true
	botRound.HasPositionPlan = true

	return botRound
}

// Naming the mode and printing the multiplier are two different questions, and this
// is where they answer differently: short-only always needs naming — the position is
// on borrowed goods whatever the multiplier says — while a plan that multiplies
// nothing has no notional to print.
//
// The plan is real here rather than absent. A round with no plan at all prints no
// notional either, so asserting its absence against one would pass on any logic.
func TestStrategyBotMessageNamesShortOnlyEvenWithNoMultiplier(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(suggestingShortOnlyBotRound()).Text()

	assert.Contains(t, message, "⚙️ 交易模式 只做空")
	assert.Contains(t, message, "保證金 5000")
	assert.NotContains(t, message, "名目")
}

// A short's stop sits above the price it was opened at, and this mode opens nothing
// else. Written out in words because 62255.085 reads like an ordinary price
// whichever side it was meant for.
func TestStrategyBotMessagePutsAShortOnlyStopAbove(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(suggestingShortOnlyBotRound()).Text()

	assert.Contains(t, message, "止損 62255.085（往上，虧 450）")
	assert.Contains(t, message, "止盈 67389.525（往下，賺 750）")
}
