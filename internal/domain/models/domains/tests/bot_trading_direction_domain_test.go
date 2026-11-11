package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func declaring(directions ...string) *[]string {
	return &directions
}

func TestStrategyBotTradingDirectionSettlesWhatAContractBotFaces(t *testing.T) {
	testCases := []struct {
		name         string
		declared     *[]string
		expectedMode string
	}{
		{name: "long alone is long only", declared: declaring("long"), expectedMode: "longOnly"},
		{name: "short alone is short only", declared: declaring("short"), expectedMode: "shortOnly"},
		{name: "both is long-short", declared: declaring("long", "short"), expectedMode: "longShort"},
		{name: "nothing said is long-short", declared: nil, expectedMode: "longShort"},
		{name: "letter case and blanks are forgiven", declared: declaring(" Short "), expectedMode: "shortOnly"},
	}

	contractKind, kindError := domains.NewMarketDataKindDomain("contractKCandle")
	require.NoError(t, kindError)
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tradingDirection, settleError := domains.NewStrategyBotTradingDirectionDomain(contractKind, testCase.declared, "")

			require.NoError(t, settleError)
			assert.Equal(t, testCase.expectedMode, tradingDirection.ContractTradingMode())
		})
	}
}

func TestStrategyBotTradingDirectionRefusals(t *testing.T) {
	testCases := []struct {
		name            string
		marketDataKind  string
		declared        *[]string
		expectedMessage string
	}{
		{name: "a contract bot facing nothing", marketDataKind: "contractKCandle", declared: declaring(),
			expectedMessage: "至少要選一個交易方向"},
		{name: "a direction that is not one", marketDataKind: "contractKCandle", declared: declaring("long", "up"),
			expectedMessage: `不認得 "up"`},
		{name: "a spot bot given a direction", marketDataKind: "kCandle", declared: declaring("long"),
			expectedMessage: "現貨機器人沒有交易方向"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			kind, kindError := domains.NewMarketDataKindDomain(testCase.marketDataKind)
			require.NoError(t, kindError)

			_, settleError := domains.NewStrategyBotTradingDirectionDomain(kind, testCase.declared, "")

			require.ErrorIs(t, settleError, domains.ErrStrategyBotValidation)
			assert.Contains(t, settleError.Error(), testCase.expectedMessage)
		})
	}
}

func TestStrategyBotTradingDirectionOfASpotBotIsNone(t *testing.T) {
	spotKind, kindError := domains.NewMarketDataKindDomain("kCandle")
	require.NoError(t, kindError)

	for _, declared := range []*[]string{nil, declaring()} {
		tradingDirection, settleError := domains.NewStrategyBotTradingDirectionDomain(spotKind, declared, "")

		require.NoError(t, settleError)
		assert.Empty(t, tradingDirection.ContractTradingMode())
		assert.False(t, tradingDirection.Differs("longOnly"), "a spot bot never turns around")
	}
}

func TestStrategyBotTradingDirectionTellsAChange(t *testing.T) {
	testCases := []struct {
		name         string
		declared     *[]string
		storedMode   string
		expectChange bool
	}{
		{name: "the same direction", declared: declaring("long"), storedMode: "longOnly", expectChange: false},
		{name: "turned around", declared: declaring("short"), storedMode: "longOnly", expectChange: true},
		{name: "both against a bot not yet upgraded", declared: declaring("long", "short"), storedMode: "", expectChange: false},
		{name: "long against a bot not yet upgraded", declared: declaring("long"), storedMode: "", expectChange: true},
	}

	contractKind, kindError := domains.NewMarketDataKindDomain("contractKCandle")
	require.NoError(t, kindError)
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tradingDirection, settleError := domains.NewStrategyBotTradingDirectionDomain(contractKind, testCase.declared, "")
			require.NoError(t, settleError)

			assert.Equal(t, testCase.expectChange, tradingDirection.Differs(testCase.storedMode))
		})
	}
}

func TestContractTradingModeFacingTranslatesDirectionsOnce(t *testing.T) {
	testCases := []struct {
		facesLong    bool
		facesShort   bool
		expectedMode vo.ContractTradingModeVo
	}{
		{facesLong: true, facesShort: false, expectedMode: vo.ContractTradingModeLongOnly},
		{facesLong: false, facesShort: true, expectedMode: vo.ContractTradingModeShortOnly},
		{facesLong: true, facesShort: true, expectedMode: vo.ContractTradingModeLongShort},
		{facesLong: false, facesShort: false, expectedMode: vo.ContractTradingModeLongShort},
	}

	for _, testCase := range testCases {
		tradingMode := domains.NewContractTradingModeDomainFacing(testCase.facesLong, testCase.facesShort)

		assert.Equal(t, testCase.expectedMode, tradingMode.Value())
	}
}

func TestStrategyBotNotificationsSettleOverTheDefaults(t *testing.T) {
	off, on := false, true
	testCases := []struct {
		name     string
		declared dto.StrategyBotNotificationsWriteDto
		expected dto.StrategyBotNotificationsDto
	}{
		{name: "nothing said: buy and sell on, hold off",
			declared: dto.StrategyBotNotificationsWriteDto{},
			expected: dto.StrategyBotNotificationsDto{Buy: true, Sell: true, Hold: false}},
		{name: "each said overrides its own",
			declared: dto.StrategyBotNotificationsWriteDto{Buy: &off, Hold: &on},
			expected: dto.StrategyBotNotificationsDto{Buy: false, Sell: true, Hold: true}},
		{name: "all off",
			declared: dto.StrategyBotNotificationsWriteDto{Buy: &off, Sell: &off, Hold: &off},
			expected: dto.StrategyBotNotificationsDto{}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			notifications := domains.NewStrategyBotNotificationDomainSettling(testCase.declared, nil)

			assert.Equal(t, testCase.expected, notifications.ToDto())
			assert.Equal(t, testCase.expected.Buy, notifications.NotifiesSignal(vo.SignalBuy))
			assert.Equal(t, testCase.expected.Sell, notifications.NotifiesSignal(vo.SignalSell))
			assert.Equal(t, testCase.expected.Hold, notifications.NotifiesHold())
			assert.False(t, notifications.NotifiesSignal(vo.SignalHold), "hold is never a signal to send")
		})
	}
}

func TestStrategyBotVerdictTellsAReturnToHolding(t *testing.T) {
	testCases := []struct {
		name            string
		buyHolds        bool
		sellHolds       bool
		lastOpinion     string
		expectedOpinion vo.SignalVo
		expectReturn    bool
	}{
		{name: "a hold after a buy", lastOpinion: "buy", expectedOpinion: vo.SignalHold, expectReturn: true},
		{name: "a hold after a sell", lastOpinion: "sell", expectedOpinion: vo.SignalHold, expectReturn: true},
		{name: "a hold after a hold", lastOpinion: "hold", expectedOpinion: vo.SignalHold, expectReturn: false},
		{name: "a hold just after a start", lastOpinion: "", expectedOpinion: vo.SignalHold, expectReturn: true},
		{name: "a buy is no return to holding", buyHolds: true, lastOpinion: "hold", expectedOpinion: vo.SignalBuy},
		{name: "a conflict has no opinion", buyHolds: true, sellHolds: true, lastOpinion: "buy", expectedOpinion: ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			verdict := domains.NewStrategyBotVerdictDomain(testCase.buyHolds, testCase.sellHolds, "")

			assert.Equal(t, testCase.expectedOpinion, verdict.Opinion())
			assert.Equal(t, testCase.expectReturn, verdict.ReturnsToHoldFrom(testCase.lastOpinion))
		})
	}
}

func TestStrategyBotWatchedSymbolRemembersItsLastOpinion(t *testing.T) {
	roundDueAt := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	testCases := []struct {
		name                string
		lastOpinion         string
		opinion             vo.SignalVo
		expectedLastOpinion string
	}{
		{name: "a hold is remembered", lastOpinion: "buy", opinion: vo.SignalHold, expectedLastOpinion: "hold"},
		{name: "a conflict leaves a run of holds alone", lastOpinion: "hold", opinion: "", expectedLastOpinion: "hold"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			concluded := domains.NewStrategyBotWatchedSymbolDomain(entities.StrategyBotWatchedSymbol{
				Symbol: "BTCUSDT", LastOpinion: testCase.lastOpinion, LastSentSignal: "buy",
			}).Concluded(roundDueAt, "", testCase.opinion, testCase.opinion == "")

			assert.Equal(t, testCase.expectedLastOpinion, concluded.LastOpinion)
			assert.Equal(t, "buy", concluded.LastSentSignal, "an opinion never moves the last sent signal")
		})
	}

	t.Run("a start forgets it", func(t *testing.T) {
		cleared := domains.NewStrategyBotWatchedSymbolDomain(entities.StrategyBotWatchedSymbol{
			Symbol: "BTCUSDT", LastOpinion: "hold",
		}).ClearedForStart()

		assert.Empty(t, cleared.LastOpinion)
	})
}

func TestStrategyBotWatchedSymbolMustBeFlatForADirectionChange(t *testing.T) {
	testCases := []struct {
		name                  string
		positionDirection     string
		hasUnsettledAutoOrder bool
		expectedError         error
		expectedMessage       string
	}{
		{name: "flat and settled", positionDirection: "", expectedError: nil},
		{name: "flat after a close", positionDirection: "flat", expectedError: nil},
		{name: "holding a long", positionDirection: "long",
			expectedError: domains.ErrStrategyBotWatchedSymbolHoldsPosition, expectedMessage: "BTCUSDT 還有機器人持倉，要先讓它空手才能改交易方向"},
		{name: "an auto order not yet done", positionDirection: "", hasUnsettledAutoOrder: true,
			expectedError: domains.ErrStrategyBotWatchedSymbolAutoOrderOutstanding, expectedMessage: "BTCUSDT 還有沒做完的自動下單，要等它做完才能改交易方向"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			flatError := domains.NewStrategyBotWatchedSymbolDomain(entities.StrategyBotWatchedSymbol{
				Symbol: "BTCUSDT", AutoOrderPositionDirection: testCase.positionDirection,
			}).RequireFlatForDirectionChange(testCase.hasUnsettledAutoOrder)

			if testCase.expectedError == nil {
				assert.NoError(t, flatError)
				return
			}
			require.ErrorIs(t, flatError, testCase.expectedError)
			assert.Contains(t, flatError.Error(), testCase.expectedMessage)
		})
	}
}

func TestStrategyBotMessageSaysAReturnToHolding(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(dto.StrategyBotRoundDto{
		BotName: "突破", Symbol: "BTCUSDT", MarketDataKind: "contractKCandle", ContractTradingMode: "longOnly",
		ReturnsToHold: true, RunNumber: 7,
		SourceSignals: []dto.StrategyBotSourceSignalDto{{Label: "A", AggregationInterval: "1h", Signal: "hold"}},
		PositionPlan:  dto.PositionPlanDto{}, HasPositionPlan: true, JournalLinkUrl: "https://example.com/journal",
	}).Text()

	assert.Contains(t, message, "⚪【回到持有】突破 · BTCUSDT 永續合約")
	assert.Contains(t, message, "買入條件與賣出條件都不成立")
	assert.Contains(t, message, "　・A（1h）：持有")
	assert.Contains(t, message, "🔖 Run 7")
	assert.NotContains(t, message, "參考價")
	assert.NotContains(t, message, "建議部位")
	assert.NotContains(t, message, "記到交易日誌")
}

func TestStrategyBotRoundOutcomeQueuesItsMessageUnderTheSignalItSpeaksOf(t *testing.T) {
	testCases := []struct {
		name           string
		sentSignal     string
		opinion        string
		expectedSignal vo.SignalVo
	}{
		{name: "a decided buy", sentSignal: "buy", opinion: "buy", expectedSignal: vo.SignalBuy},
		{name: "a return to holding", sentSignal: "", opinion: "hold", expectedSignal: vo.SignalHold},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			outcome := domains.NewStrategyBotWatchedSymbolRoundOutcomeDomain(dto.StrategyBotWatchedSymbolRoundOutcomeDto{
				Symbol: "BTCUSDT", Kind: "concluded", SentSignal: testCase.sentSignal, Opinion: testCase.opinion,
			})

			assert.Equal(t, testCase.expectedSignal, outcome.MessageSignal())
		})
	}
}

func TestStrategyBotTradingDirectionKeepsTheRetainedOneWhenNothingIsSaid(t *testing.T) {
	testCases := []struct {
		name         string
		declared     *[]string
		retainedMode string
		expectedMode string
	}{
		{name: "nothing said keeps short only", declared: nil, retainedMode: "shortOnly", expectedMode: "shortOnly"},
		{name: "what is said wins over what is retained", declared: declaring("long"), retainedMode: "shortOnly", expectedMode: "longOnly"},
		{name: "a bot not yet upgraded reads as both", declared: nil, retainedMode: "", expectedMode: "longShort"},
		{name: "an unreadable retained mode reads as both", declared: nil, retainedMode: "sideways", expectedMode: "longShort"},
	}

	contractKind, kindError := domains.NewMarketDataKindDomain("contractKCandle")
	require.NoError(t, kindError)
	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tradingDirection, settleError := domains.NewStrategyBotTradingDirectionDomain(
				contractKind, testCase.declared, testCase.retainedMode)

			require.NoError(t, settleError)
			assert.Equal(t, testCase.expectedMode, tradingDirection.ContractTradingMode())
		})
	}
}

func TestStrategyBotNotificationsKeepTheRetainedOnesWhenNothingIsSaid(t *testing.T) {
	on := true
	retained := dto.StrategyBotNotificationsDto{Buy: false, Sell: false, Hold: false}

	notifications := domains.NewStrategyBotNotificationDomainSettling(
		dto.StrategyBotNotificationsWriteDto{Hold: &on}, &retained)

	assert.Equal(t, dto.StrategyBotNotificationsDto{Buy: false, Sell: false, Hold: true}, notifications.ToDto())
}
