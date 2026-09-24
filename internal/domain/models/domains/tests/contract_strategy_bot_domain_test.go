package domains_test

import (
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aContractBotWrite is a contract bot as it arrives to be saved: a name, a contract,
// the rules it names and how often it wakes.
func aContractBotWrite() dto.StrategyBotWriteDto {
	return dto.StrategyBotWriteDto{
		OwnerID:                1,
		Name:                   "合約突破",
		Symbol:                 "BTCUSDT",
		MarketDataKind:         string(vo.MarketDataKindContractKCandle),
		TradingStrategyID:      9,
		TriggerIntervalMinutes: 5,
	}
}

func TestStrategyBotDomainReadsWhichKindOfMarketABotEats(t *testing.T) {
	t.Run("saying nothing is a spot bot", func(t *testing.T) {
		writeDto := aContractBotWrite()
		writeDto.MarketDataKind = ""

		bot, buildError := domains.NewStrategyBotDomain(writeDto)

		require.NoError(t, buildError)
		assert.False(t, bot.WatchesContracts())
		assert.Equal(t, string(vo.MarketDataKindKCandle), bot.ToEntity().MarketDataKind)
	})

	t.Run("a contract bot is stored as one", func(t *testing.T) {
		bot, buildError := domains.NewStrategyBotDomain(aContractBotWrite())

		require.NoError(t, buildError)
		assert.True(t, bot.WatchesContracts())
		assert.Equal(t, string(vo.MarketDataKindContractKCandle), bot.ToEntity().MarketDataKind)
	})

	t.Run("a kind nobody recognises is refused, naming the two there are", func(t *testing.T) {
		writeDto := aContractBotWrite()
		writeDto.MarketDataKind = "期貨"

		_, buildError := domains.NewStrategyBotDomain(writeDto)

		require.ErrorIs(t, buildError, domains.ErrStrategyBotValidation)
		assert.ErrorContains(t, buildError, "行情種類只能是 kCandle、contractKCandle 其中之一")
	})
}

func TestStrategyBotDomainReadsAContractBotsLeverage(t *testing.T) {
	testCases := []struct {
		name             string
		declared         string
		expectedLeverage string
		expectedRefusal  string
	}{
		{name: "left blank is one times", declared: "0", expectedLeverage: "1"},
		{name: "five times is five times", declared: "5", expectedLeverage: "5"},
		{name: "exactly one is allowed", declared: "1", expectedLeverage: "1"},
		{name: "under one is refused", declared: "0.5", expectedRefusal: "槓桿倍數不得小於 1 倍"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			writeDto := aContractBotWrite()
			writeDto.DeclaredLeverage = decimal.RequireFromString(testCase.declared)

			bot, buildError := domains.NewStrategyBotDomain(writeDto)

			if testCase.expectedRefusal != "" {
				require.ErrorIs(t, buildError, domains.ErrStrategyBotValidation)
				assert.ErrorContains(t, buildError, testCase.expectedRefusal)

				return
			}

			require.NoError(t, buildError)
			assert.Equal(t, testCase.expectedLeverage, bot.Leverage().String())
			assert.Equal(t, testCase.expectedLeverage, bot.ToEntity().PositionPlanLeverage.String())
		})
	}
}

// A spot bot still borrows nothing, in the words it has always been refused in.
func TestStrategyBotDomainStillRefusesASpotBotLeverage(t *testing.T) {
	writeDto := aContractBotWrite()
	writeDto.MarketDataKind = ""
	writeDto.DeclaredLeverage = decimal.NewFromInt(3)

	_, buildError := domains.NewStrategyBotDomain(writeDto)

	require.ErrorIs(t, buildError, domains.ErrStrategyBotValidation)
	assert.ErrorContains(t, buildError, "現貨是拿現金換東西，沒有人借錢給你")
}

func TestStrategyBotDomainFollowsOnlyRulesOfItsOwnKind(t *testing.T) {
	contractBot, contractBuildError := domains.NewStrategyBotDomain(aContractBotWrite())
	require.NoError(t, contractBuildError)

	spotWrite := aContractBotWrite()
	spotWrite.MarketDataKind = ""
	spotBot, spotBuildError := domains.NewStrategyBotDomain(spotWrite)
	require.NoError(t, spotBuildError)

	assert.NoError(t, contractBot.RequireFollowing(string(vo.MarketDataKindContractKCandle)))
	assert.NoError(t, spotBot.RequireFollowing(string(vo.MarketDataKindKCandle)))

	spotFollowingContract := spotBot.RequireFollowing(string(vo.MarketDataKindContractKCandle))
	require.ErrorIs(t, spotFollowingContract, domains.ErrStrategyBotValidation)
	assert.ErrorContains(t, spotFollowingContract, "這台機器人吃的是 K 線，那份交易策略吃的是合約行情")

	contractFollowingSpot := contractBot.RequireFollowing(string(vo.MarketDataKindKCandle))
	require.ErrorIs(t, contractFollowingSpot, domains.ErrStrategyBotValidation)
	assert.ErrorContains(t, contractFollowingSpot, "這台機器人吃的是合約行情，那份交易策略吃的是 K 線")
}

func TestMarketDataKindDomainKeepsABotsKindOnARewrite(t *testing.T) {
	spotKind, _ := domains.NewMarketDataKindDomain("kCandle")
	contractKind, _ := domains.NewMarketDataKindDomain("contractKCandle")

	t.Run("saying nothing keeps it", func(t *testing.T) {
		retained, retainError := contractKind.RetainingForStrategyBot("")

		require.NoError(t, retainError)
		assert.Equal(t, vo.MarketDataKindContractKCandle, retained.Value())
	})

	t.Run("restating it changes nothing", func(t *testing.T) {
		retained, retainError := spotKind.RetainingForStrategyBot("kCandle")

		require.NoError(t, retainError)
		assert.Equal(t, vo.MarketDataKindKCandle, retained.Value())
	})

	t.Run("naming the other kind is refused", func(t *testing.T) {
		_, retainError := spotKind.RetainingForStrategyBot("contractKCandle")

		require.ErrorIs(t, retainError, domains.ErrStrategyBotValidation)
		assert.ErrorContains(t, retainError, "行情種類建立後不得更換——這台機器人吃的是 K 線；要吃合約行情請另建一台")
	})

	t.Run("naming a kind nobody recognises is refused", func(t *testing.T) {
		_, retainError := spotKind.RetainingForStrategyBot("期貨")

		require.ErrorIs(t, retainError, domains.ErrStrategyBotValidation)
	})
}

// aLadderTopping at is a maintenance margin ladder whose smallest tier allows that
// much leverage and whose bigger one allows less.
func aLadderToppingAt(maximumLeverage int) []entities.ContractMaintenanceMarginTier {
	return []entities.ContractMaintenanceMarginTier{
		{Symbol: "BTCUSDT", Tier: 1, MaximumLeverage: maximumLeverage},
		{Symbol: "BTCUSDT", Tier: 2, MaximumLeverage: maximumLeverage / 2},
	}
}

func TestContractStrategyBotMarketDomainAdmitsOnlyAFollowedContract(t *testing.T) {
	testCases := []struct {
		name            string
		symbol          entities.ContractTradingSymbol
		isRegistered    bool
		tiers           []entities.ContractMaintenanceMarginTier
		leverage        string
		expectedRefusal string
	}{
		{name: "a watched contract is admitted", symbol: entities.ContractTradingSymbol{IsWatched: true},
			isRegistered: true, leverage: "1"},
		{name: "a registered contract nobody watches is refused", symbol: entities.ContractTradingSymbol{},
			isRegistered: true, leverage: "1", expectedRefusal: "BTCUSDT 不在合約追蹤名單上"},
		{name: "a contract the system has never heard of is refused", isRegistered: false,
			leverage: "1", expectedRefusal: "請先把它加進合約追蹤名單"},
		{name: "more leverage than the smallest tier allows is refused, naming the ceiling",
			symbol: entities.ContractTradingSymbol{IsWatched: true}, isRegistered: true,
			tiers: aLadderToppingAt(125), leverage: "150", expectedRefusal: "這個合約標的最高只能開 125 倍槓桿"},
		{name: "exactly the ceiling is admitted", symbol: entities.ContractTradingSymbol{IsWatched: true},
			isRegistered: true, tiers: aLadderToppingAt(125), leverage: "125"},
		{name: "with no ladder there is no ceiling to refuse by",
			symbol: entities.ContractTradingSymbol{IsWatched: true}, isRegistered: true, leverage: "150"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			admitError := domains.NewContractStrategyBotMarketDomain(
				"BTCUSDT", testCase.symbol, testCase.isRegistered, testCase.tiers,
			).Admit(decimal.RequireFromString(testCase.leverage))

			if testCase.expectedRefusal == "" {
				assert.NoError(t, admitError)

				return
			}

			require.ErrorIs(t, admitError, domains.ErrStrategyBotValidation)
			assert.ErrorContains(t, admitError, testCase.expectedRefusal)
		})
	}
}

// What a contract conclusion asks its reader to go and do, under each trading mode.
func TestStrategyBotMarketDomainNamesTheActOnAContractAccount(t *testing.T) {
	testCases := []struct {
		tradingMode    vo.ContractTradingModeVo
		verdict        vo.SignalVo
		expectedVerb   string
		expectedMark   string
		expectedTarget vo.TargetPositionVo
	}{
		{vo.ContractTradingModeLongShort, vo.SignalSell, "做空", "🔴", vo.TargetPositionShort},
		{vo.ContractTradingModeLongShort, vo.SignalBuy, "做多", "🟢", vo.TargetPositionLong},
		{vo.ContractTradingModeLongOnly, vo.SignalSell, "平多", "⚪", vo.TargetPositionFlat},
		{vo.ContractTradingModeLongOnly, vo.SignalBuy, "做多", "🟢", vo.TargetPositionLong},
		{vo.ContractTradingModeShortOnly, vo.SignalBuy, "平空", "⚪", vo.TargetPositionFlat},
		{vo.ContractTradingModeShortOnly, vo.SignalSell, "做空", "🔴", vo.TargetPositionShort},
	}

	for _, testCase := range testCases {
		t.Run(string(testCase.tradingMode)+" "+string(testCase.verdict), func(t *testing.T) {
			market := domains.NewStrategyBotMarketDomain("contractKCandle", string(testCase.tradingMode))
			signal := domains.NewSignalDomainOf(testCase.verdict)

			assert.Equal(t, testCase.expectedVerb, market.HeadlineVerb(signal))
			assert.Equal(t, testCase.expectedMark, market.HeadlineMark(signal))
			assert.Equal(t, testCase.expectedTarget, market.TargetFor(signal))
		})
	}
}

func TestStrategyBotMarketDomainLeavesASpotAccountAsItWas(t *testing.T) {
	market := domains.NewStrategyBotMarketDomain("kCandle", "")

	assert.Equal(t, "出場", market.HeadlineVerb(domains.NewSignalDomainOf(vo.SignalSell)))
	assert.Equal(t, "買入", market.HeadlineVerb(domains.NewSignalDomainOf(vo.SignalBuy)))
	assert.Equal(t, vo.TargetPositionFlat, market.TargetFor(domains.NewSignalDomainOf(vo.SignalSell)))
	assert.Equal(t, "BTCUSDT", market.SymbolLabel("BTCUSDT"))
	assert.Empty(t, market.TradingModeInWords())
}

func TestStrategyBotMarketDomainLabelsAContract(t *testing.T) {
	testCases := []struct {
		tradingMode   string
		expectedWords string
	}{
		{tradingMode: "longShort", expectedWords: "多空反手"},
		{tradingMode: "", expectedWords: "多空反手"},
		{tradingMode: "longOnly", expectedWords: "只做多"},
		{tradingMode: "shortOnly", expectedWords: "只做空"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.expectedWords, func(t *testing.T) {
			market := domains.NewStrategyBotMarketDomain("contractKCandle", testCase.tradingMode)

			assert.Equal(t, "BTCUSDT 永續合約", market.SymbolLabel("BTCUSDT"))
			assert.Equal(t, testCase.expectedWords, market.TradingModeInWords())
		})
	}
}

// A stored mode that cannot be read names no act and suggests nothing, rather than
// guessing which way somebody should trade.
func TestStrategyBotMarketDomainGuessesNothingForAModeItCannotRead(t *testing.T) {
	market := domains.NewStrategyBotMarketDomain("contractKCandle", "spot")
	signal := domains.NewSignalDomainOf(vo.SignalSell)

	assert.Equal(t, vo.TargetPositionUnchanged, market.TargetFor(signal))
	assert.Equal(t, "賣出", market.HeadlineVerb(signal))
	assert.Equal(t, "⚪", market.HeadlineMark(signal))
	assert.Empty(t, market.TradingModeInWords())
}

// aContractPositionPlanSettings is a thousand, staked whole, at five times, out at two
// percent against and four in favour.
func aContractPositionPlanSettings() dto.PositionPlanSettingsDto {
	return dto.PositionPlanSettingsDto{
		Capital:              decimal.NewFromInt(1000),
		StopLossPercentage:   decimal.NewFromInt(2),
		TakeProfitPercentage: decimal.NewFromInt(4),
		Leverage:             decimal.NewFromInt(5),
	}
}

func contractPlanAt100(
	t *testing.T, settings dto.PositionPlanSettingsDto, target vo.TargetPositionVo,
) (dto.PositionPlanDto, bool) {
	t.Helper()

	positionPlan, buildError := domains.NewPositionPlanDomain(settings)
	require.NoError(t, buildError)

	return positionPlan.PlanFor(target, decimal.NewFromInt(100), true)
}

func TestPositionPlanDomainSizesALongContractPosition(t *testing.T) {
	positionPlanDto, suggests := contractPlanAt100(t, aContractPositionPlanSettings(), vo.TargetPositionLong)

	require.True(t, suggests)
	assert.Equal(t, "1000", positionPlanDto.Stake.String())
	assert.Equal(t, "5", positionPlanDto.Leverage.String())
	assert.Equal(t, "5000", positionPlanDto.Notional.String())
	assert.Equal(t, "long", positionPlanDto.Direction)
	assert.Equal(t, "98", positionPlanDto.StopLossPrice.String())
	assert.Equal(t, "100", positionPlanDto.LossAtStop.String())
	assert.Equal(t, "104", positionPlanDto.TakeProfitPrice.String())
	assert.Equal(t, "200", positionPlanDto.GainAtTarget.String())
	assert.False(t, positionPlanDto.LiquidatesBeforeStop)
}

func TestPositionPlanDomainTurnsBothExitsRoundForAShortContractPosition(t *testing.T) {
	positionPlanDto, suggests := contractPlanAt100(t, aContractPositionPlanSettings(), vo.TargetPositionShort)

	require.True(t, suggests)
	assert.Equal(t, "short", positionPlanDto.Direction)
	assert.Equal(t, "5000", positionPlanDto.Notional.String())
	assert.Equal(t, "102", positionPlanDto.StopLossPrice.String())
	assert.Equal(t, "100", positionPlanDto.LossAtStop.String())
	assert.Equal(t, "96", positionPlanDto.TakeProfitPrice.String())
	assert.Equal(t, "200", positionPlanDto.GainAtTarget.String())
}

func TestPositionPlanDomainWarnsOfAStopTheMarginCannotReach(t *testing.T) {
	testCases := []struct {
		name          string
		leverage      int64
		stopLoss      int64
		expectedWarns bool
	}{
		{name: "ten times at ten percent eats the whole margin", leverage: 10, stopLoss: 10, expectedWarns: true},
		{name: "ten times at nine percent does not", leverage: 10, stopLoss: 9, expectedWarns: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			settings := aContractPositionPlanSettings()
			settings.Leverage = decimal.NewFromInt(testCase.leverage)
			settings.StopLossPercentage = decimal.NewFromInt(testCase.stopLoss)

			positionPlanDto, suggests := contractPlanAt100(t, settings, vo.TargetPositionLong)

			require.True(t, suggests)
			assert.Equal(t, testCase.expectedWarns, positionPlanDto.LiquidatesBeforeStop)
		})
	}
}

// A spot plan borrows nothing, so even a stop a whole price away is not a close-out.
func TestPositionPlanDomainNeverWarnsASpotPlanOfAClose(t *testing.T) {
	settings := aContractPositionPlanSettings()
	settings.Leverage = decimal.Zero
	settings.StopLossPercentage = decimal.NewFromInt(100)

	positionPlanDto, suggests := contractPlanAt100(t, settings, vo.TargetPositionLong)

	require.True(t, suggests)
	assert.Equal(t, "1000", positionPlanDto.Notional.String())
	assert.False(t, positionPlanDto.LiquidatesBeforeStop)
}

func TestPositionPlanDomainSuggestsNothingWhenAContractRoundCloses(t *testing.T) {
	_, suggests := contractPlanAt100(t, aContractPositionPlanSettings(), vo.TargetPositionFlat)

	assert.False(t, suggests)
}

func TestPositionPlanDomainSaysAContractMarginCannotBePutDown(t *testing.T) {
	settings := aContractPositionPlanSettings()
	settings.SizingMode = string(vo.PositionSizingModeFixedAmount)
	settings.SizingValue = decimal.NewFromInt(2000)

	positionPlanDto, suggests := contractPlanAt100(t, settings, vo.TargetPositionLong)

	require.True(t, suggests)
	assert.False(t, positionPlanDto.Affordable)
	assert.Equal(t, "2000", positionPlanDto.Stake.String())
}

// aContractRound is one contract bot's round that concluded sell under long and short,
// with a five-times plan behind it.
func aContractRound() dto.StrategyBotRoundDto {
	round := dto.StrategyBotRoundDto{
		BotName:             "合約突破",
		Symbol:              "BTCUSDT",
		MarketDataKind:      "contractKCandle",
		ContractTradingMode: "longShort",
		Verdict:             string(vo.SignalSell),
		ReferencePrice:      decimal.NewFromInt(100),
		ReferenceTime:       time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC),
		HasReference:        true,
		SourceSignals: []dto.StrategyBotSourceSignalDto{
			{Label: "A", AggregationInterval: "1h", Signal: string(vo.SignalSell)},
		},
	}

	positionPlan, _ := domains.NewPositionPlanDomain(aContractPositionPlanSettings())
	round.PositionPlan, round.HasPositionPlan = positionPlan.PlanFor(
		vo.TargetPositionShort, round.ReferencePrice, true)

	return round
}

func TestStrategyBotMessageSpeaksOfAContractAccount(t *testing.T) {
	message := domains.NewStrategyBotMessageDomain(aContractRound()).Text()
	lines := strings.Split(message, "\n")

	assert.Equal(t, "🔴【做空】合約突破 · BTCUSDT 永續合約", lines[0])
	assert.Contains(t, message, "⚙️ 交易模式 多空反手")
	assert.Contains(t, message, "💰 參考價 100")
	assert.Contains(t, message, "2026-09-24 08:00 UTC 那一根一分鐘合約 K 線的收盤價")
	assert.Contains(t, message, "　・保證金 1000（5 倍槓桿，名目 5000）")
	assert.Contains(t, message, "　・止損 102（往上，虧 100）")
	assert.Contains(t, message, "　・止盈 96（往下，賺 200）")
	assert.NotContains(t, message, "強制平倉")
	// The sources still speak in their own words.
	assert.Contains(t, message, "　・A（1h）：賣出")
}

func TestStrategyBotMessageWarnsOfAContractStopBeyondTheMargin(t *testing.T) {
	round := aContractRound()
	settings := aContractPositionPlanSettings()
	settings.Leverage = decimal.NewFromInt(10)
	settings.StopLossPercentage = decimal.NewFromInt(10)
	positionPlan, _ := domains.NewPositionPlanDomain(settings)
	round.PositionPlan, round.HasPositionPlan = positionPlan.PlanFor(vo.TargetPositionShort, round.ReferencePrice, true)

	message := domains.NewStrategyBotMessageDomain(round).Text()

	assert.Contains(t, message, "還沒到止損就會先被強制平倉")
}

func TestStrategyBotMessageSaysAContractPriceCouldNotBeRead(t *testing.T) {
	round := aContractRound()
	round.HasReference = false
	round.HasPositionPlan = false

	message := domains.NewStrategyBotMessageDomain(round).Text()

	assert.Contains(t, message, "💰 參考價 目前讀不到這個交易標的的最新合約 K 線")
}

func TestStrategyBotMessageNamesTheSideAContractCloseIsAbout(t *testing.T) {
	round := aContractRound()
	round.ContractTradingMode = "longOnly"
	round.HasPositionPlan = false

	message := domains.NewStrategyBotMessageDomain(round).Text()

	assert.Equal(t, "⚪【平多】合約突破 · BTCUSDT 永續合約", strings.Split(message, "\n")[0])
	assert.Contains(t, message, "⚙️ 交易模式 只做多")
	assert.NotContains(t, message, "建議部位")
}
