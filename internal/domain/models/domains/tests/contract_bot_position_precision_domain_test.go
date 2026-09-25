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

var precisionReferenceTime = time.Date(2026, 9, 24, 8, 0, 0, 0, time.UTC)

// aSpecifiedContract steps quantities by 0.001, has minimums of 0.001 units and 5 notional, a 0.5% smallest tier and eight-hour funding.
func aSpecifiedContract(tickSize string) entities.ContractTradingSymbol {
	specifiedAt := precisionReferenceTime.Add(-time.Hour)
	fundingIntervalHours := 8

	return entities.ContractTradingSymbol{
		Symbol: "BTCUSDT", IsWatched: true,
		TickSize:               decimal.NewNullDecimal(decimal.RequireFromString(tickSize)),
		QuantityStep:           decimal.NewNullDecimal(decimal.RequireFromString("0.001")),
		MinimumQuantity:        decimal.NewNullDecimal(decimal.RequireFromString("0.001")),
		MinimumNotional:        decimal.NewNullDecimal(decimal.NewFromInt(5)),
		MaintenanceMarginRate:  decimal.NewNullDecimal(decimal.RequireFromString("0.005")),
		LiquidationFeeRate:     decimal.NewNullDecimal(decimal.RequireFromString("0.005")),
		FundingIntervalHours:   &fundingIntervalHours,
		SpecificationUpdatedAt: &specifiedAt,
	}
}

func aVenue(
	contract entities.ContractTradingSymbol, tiers []entities.ContractMaintenanceMarginTier, fundingRate string,
) domains.ContractStrategyBotVenueDomain {
	if fundingRate == "" {
		return domains.NewContractStrategyBotVenueDomain(
			contract, true, tiers, entities.ContractFundingRateSettlement{}, false)
	}

	return domains.NewContractStrategyBotVenueDomain(contract, true, tiers,
		entities.ContractFundingRateSettlement{FundingRate: decimal.RequireFromString(fundingRate)}, true)
}

// aPrecisionPlan is 1,000 staked whole at that leverage.
func aPrecisionPlan(leverage int64, stopLoss string, takeProfit string) dto.PositionPlanSettingsDto {
	return dto.PositionPlanSettingsDto{
		Capital:              decimal.NewFromInt(1000),
		StopLossPercentage:   decimal.RequireFromString(stopLoss),
		TakeProfitPercentage: decimal.RequireFromString(takeProfit),
		Leverage:             decimal.NewFromInt(leverage),
	}
}

func planOnVenue(
	t *testing.T, settings dto.PositionPlanSettingsDto, target vo.TargetPositionVo,
	referencePrice string, venue domains.ContractStrategyBotVenueDomain,
) dto.PositionPlanDto {
	t.Helper()

	positionPlan, buildError := domains.NewPositionPlanDomain(settings)
	require.NoError(t, buildError)

	positionPlanDto, suggests := positionPlan.PlanOnContractVenue(
		target, decimal.RequireFromString(referencePrice), precisionReferenceTime, venue)
	require.True(t, suggests)

	return positionPlanDto
}

func TestPositionPlanDomainStepsAContractSuggestionToTheVenue(t *testing.T) {
	testCases := []struct {
		name             string
		settings         dto.PositionPlanSettingsDto
		referencePrice   string
		tickSize         string
		expectedQuantity string
		expectedNotional string
		expectedMargin   string
		expectedStopLoss string
	}{
		{name: "five times at a hundred", settings: aPrecisionPlan(5, "2", "4"), referencePrice: "100",
			tickSize: "0.01", expectedQuantity: "50", expectedNotional: "5000", expectedMargin: "1000",
			expectedStopLoss: "98"},
		{name: "the quantity stepped down and the margin worked out again", settings: aPrecisionPlan(1, "0", "0"),
			referencePrice: "810", tickSize: "0.01", expectedQuantity: "1.234", expectedNotional: "999.54",
			expectedMargin: "999.54"},
		{name: "the stop on the nearest tick", settings: aPrecisionPlan(5, "2.03", "0"), referencePrice: "100",
			tickSize: "0.1", expectedQuantity: "50", expectedNotional: "5000", expectedMargin: "1000",
			expectedStopLoss: "98"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			positionPlanDto := planOnVenue(t, testCase.settings, vo.TargetPositionLong, testCase.referencePrice,
				aVenue(aSpecifiedContract(testCase.tickSize), nil, ""))

			require.True(t, positionPlanDto.HasQuantity)
			assert.Equal(t, testCase.expectedQuantity, positionPlanDto.Quantity.String())
			assert.Equal(t, testCase.expectedNotional, positionPlanDto.Notional.String())
			assert.Equal(t, testCase.expectedMargin, positionPlanDto.Stake.String())
			if testCase.expectedStopLoss != "" {
				assert.True(t, positionPlanDto.StopLossPrice.Equal(decimal.RequireFromString(testCase.expectedStopLoss)),
					"止損價 %s", positionPlanDto.StopLossPrice)
			}
		})
	}
}

func TestPositionPlanDomainSaysWhyTheVenueWouldRefuseASuggestion(t *testing.T) {
	fixedThree := aPrecisionPlan(1, "2", "4")
	fixedThree.SizingMode = string(vo.PositionSizingModeFixedAmount)
	fixedThree.SizingValue = decimal.NewFromInt(3)

	largeMinimum := aSpecifiedContract("0.01")
	largeMinimum.MinimumQuantity = decimal.NewNullDecimal(decimal.NewFromInt(100))

	testCases := []struct {
		name           string
		settings       dto.PositionPlanSettingsDto
		venue          domains.ContractStrategyBotVenueDomain
		expectedReason string
		expectedWords  string
	}{
		{name: "less notional than the smallest order", settings: fixedThree,
			venue: aVenue(aSpecifiedContract("0.01"), nil, ""), expectedReason: "belowMinimumNotional",
			expectedWords: "交易所不收這一筆：名目 3 低於最小名目 5"},
		{name: "fewer units than the smallest order", settings: aPrecisionPlan(5, "2", "4"),
			venue: aVenue(largeMinimum, nil, ""), expectedReason: "belowMinimumQuantity",
			expectedWords: "交易所不收這一筆：數量 50 低於最小下單量 100"},
		{name: "more leverage than the notional's tier allows", settings: aPrecisionPlan(20, "2", "4"),
			venue: aVenue(aSpecifiedContract("0.01"), []entities.ContractMaintenanceMarginTier{
				{Tier: 1, NotionalCap: decimal.NewFromInt(10000), MaintenanceMarginRate: decimal.RequireFromString("0.004"),
					MaximumLeverage: 125},
				{Tier: 2, NotionalFloor: decimal.NewFromInt(10000), NotionalCap: decimal.NewFromInt(50000),
					MaintenanceMarginRate: decimal.RequireFromString("0.01"), MaximumLeverage: 10},
			}, ""),
			expectedReason: "aboveTierLeverage", expectedWords: "交易所不收這一筆：名目 20000 那一級最高只能開 10 倍"},
		// Both minimums broken: quantity is reported because it is checked first.
		{name: "too few units and too little notional", settings: fixedThree,
			venue: aVenue(largeMinimum, nil, ""), expectedReason: "belowMinimumQuantity",
			expectedWords: "交易所不收這一筆：數量 0.03 低於最小下單量 100"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			positionPlanDto := planOnVenue(t, testCase.settings, vo.TargetPositionLong, "100", testCase.venue)

			require.True(t, positionPlanDto.HasVenueRefusal)
			assert.Equal(t, testCase.expectedReason, positionPlanDto.VenueRefusal.Reason)
			assert.False(t, positionPlanDto.HasStopLoss)
			assert.False(t, positionPlanDto.HasTakeProfit)
			assert.False(t, positionPlanDto.HasLiquidationPrice)

			message := domains.NewStrategyBotMessageDomain(aPrecisionRound(positionPlanDto)).Text()
			assert.Contains(t, message, testCase.expectedWords)
			assert.NotContains(t, message, "止損")
			assert.NotContains(t, message, "止盈")
			assert.NotContains(t, message, "預估強平價")
		})
	}
}

func TestPositionPlanDomainEstimatesWhereAContractSuggestionWouldBeClosedOut(t *testing.T) {
	testCases := []struct {
		name              string
		target            vo.TargetPositionVo
		expectedPrice     string
		expectedWordsSide string
	}{
		{name: "a long one below", target: vo.TargetPositionLong, expectedPrice: "80.4",
			expectedWordsSide: "　・預估強平價 80.4（往下，用最小那一級估算）"},
		{name: "a short one above", target: vo.TargetPositionShort, expectedPrice: "119.4",
			expectedWordsSide: "　・預估強平價 119.4（往上，用最小那一級估算）"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			positionPlanDto := planOnVenue(t, aPrecisionPlan(5, "0", "0"), testCase.target, "100",
				aVenue(aSpecifiedContract("0.01"), nil, ""))

			require.True(t, positionPlanDto.HasLiquidationPrice)
			assert.True(t, positionPlanDto.LiquidationPrice.Equal(decimal.RequireFromString(testCase.expectedPrice)),
				"預估強平價 %s", positionPlanDto.LiquidationPrice)
			assert.True(t, positionPlanDto.LiquidationFromSmallestTier)

			message := domains.NewStrategyBotMessageDomain(aPrecisionRound(positionPlanDto)).Text()
			assert.Contains(t, message, testCase.expectedWordsSide)
		})
	}
}

// With the full ladder the estimate uses the notional's own tier.
func TestPositionPlanDomainEstimatesFromTheNotionalsOwnTier(t *testing.T) {
	positionPlanDto := planOnVenue(t, aPrecisionPlan(5, "0", "0"), vo.TargetPositionLong, "100",
		aVenue(aSpecifiedContract("0.01"), []entities.ContractMaintenanceMarginTier{
			{Tier: 1, NotionalCap: decimal.NewFromInt(50000), MaintenanceMarginRate: decimal.RequireFromString("0.005"),
				MaximumLeverage: 125},
		}, ""))

	assert.False(t, positionPlanDto.LiquidationFromSmallestTier)
	message := domains.NewStrategyBotMessageDomain(aPrecisionRound(positionPlanDto)).Text()
	assert.Contains(t, message, "　・預估強平價 80.4（往下）")
}

func TestPositionPlanDomainWarnsOfAStopBeyondTheEstimatedLiquidation(t *testing.T) {
	testCases := []struct {
		name          string
		stopLoss      string
		expectedWarns bool
	}{
		{name: "fifteen percent is past a close-out at about 90.45", stopLoss: "15", expectedWarns: true},
		{name: "five percent is well inside it", stopLoss: "5", expectedWarns: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			positionPlanDto := planOnVenue(t, aPrecisionPlan(10, testCase.stopLoss, "0"), vo.TargetPositionLong, "100",
				aVenue(aSpecifiedContract("0.01"), nil, ""))

			assert.True(t, positionPlanDto.LiquidationPrice.Equal(decimal.RequireFromString("90.45")))
			assert.Equal(t, testCase.expectedWarns, positionPlanDto.LiquidatesBeforeStop)

			message := domains.NewStrategyBotMessageDomain(aPrecisionRound(positionPlanDto)).Text()
			if testCase.expectedWarns {
				assert.Contains(t, message, "止損比預估強平價還遠：還沒到止損就會先被強制平倉")
			} else {
				assert.NotContains(t, message, "強制平倉")
			}
		})
	}
}

func TestPositionPlanDomainWarnsOfAShortStopBeyondTheEstimatedLiquidation(t *testing.T) {
	positionPlanDto := planOnVenue(t, aPrecisionPlan(10, "15", "0"), vo.TargetPositionShort, "100",
		aVenue(aSpecifiedContract("0.01"), nil, ""))

	assert.True(t, positionPlanDto.LiquidatesBeforeStop)
}

func TestPositionPlanDomainSaysALowLeverageLongCannotBeClosedOut(t *testing.T) {
	positionPlanDto := planOnVenue(t, aPrecisionPlan(1, "50", "0"), vo.TargetPositionLong, "100",
		aVenue(aSpecifiedContract("0.01"), nil, ""))

	assert.True(t, positionPlanDto.CannotBeLiquidated)
	assert.False(t, positionPlanDto.HasLiquidationPrice)
	assert.False(t, positionPlanDto.LiquidatesBeforeStop)
	message := domains.NewStrategyBotMessageDomain(aPrecisionRound(positionPlanDto)).Text()
	assert.Contains(t, message, "　・這個槓桿下不會被強制平倉")
}

func TestPositionPlanDomainEstimatesWhatFundingCosts(t *testing.T) {
	testCases := []struct {
		name          string
		target        vo.TargetPositionVo
		fundingRate   string
		expectedWords string
	}{
		{name: "a positive rate is paid by a long", target: vo.TargetPositionLong, fundingRate: "0.0001",
			expectedWords: "　・資金費率 0.01%（最近一次結算）：每 8 小時約付 0.5（估算）"},
		{name: "a positive rate is received by a short", target: vo.TargetPositionShort, fundingRate: "0.0001",
			expectedWords: "　・資金費率 0.01%（最近一次結算）：每 8 小時約收 0.5（估算）"},
		{name: "a negative rate is received by a long", target: vo.TargetPositionLong, fundingRate: "-0.0001",
			expectedWords: "　・資金費率 -0.01%（最近一次結算）：每 8 小時約收 0.5（估算）"},
		{name: "a zero rate costs nothing", target: vo.TargetPositionLong, fundingRate: "0",
			expectedWords: "　・資金費率 0%（最近一次結算）：每 8 小時不付也不收（估算）"},
		{name: "no settlement yet", target: vo.TargetPositionLong, fundingRate: "",
			expectedWords: "　・資金費率：還沒有資金費率紀錄"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			positionPlanDto := planOnVenue(t, aPrecisionPlan(5, "0", "0"), testCase.target, "100",
				aVenue(aSpecifiedContract("0.01"), nil, testCase.fundingRate))

			message := domains.NewStrategyBotMessageDomain(aPrecisionRound(positionPlanDto)).Text()
			assert.Contains(t, message, testCase.expectedWords)
			if testCase.fundingRate == "" {
				assert.NotContains(t, message, "約付")
				assert.NotContains(t, message, "約收")
			}
		})
	}
}

// Without a recorded specification the plan is unrounded, has no liquidation price, uses the rough stop rule and estimates funding without an interval.
func TestPositionPlanDomainSuggestsOnAContractWithNoSpecificationYet(t *testing.T) {
	unspecified := domains.NewContractStrategyBotVenueDomain(
		entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}, true, nil,
		entities.ContractFundingRateSettlement{FundingRate: decimal.RequireFromString("0.0001")}, true)

	t.Run("figures, and the reason there are no more", func(t *testing.T) {
		positionPlanDto := planOnVenue(t, aPrecisionPlan(5, "2", "4"), vo.TargetPositionLong, "100", unspecified)

		assert.True(t, positionPlanDto.LacksTradingSpecification)
		assert.Equal(t, "1000", positionPlanDto.Stake.String())
		assert.Equal(t, "5000", positionPlanDto.Notional.String())
		assert.True(t, positionPlanDto.HasStopLoss)
		assert.True(t, positionPlanDto.HasTakeProfit)
		assert.False(t, positionPlanDto.HasQuantity)
		assert.False(t, positionPlanDto.HasLiquidationPrice)

		message := domains.NewStrategyBotMessageDomain(aPrecisionRound(positionPlanDto)).Text()
		assert.Contains(t, message, "這個合約標的還沒有交易規格：數字未照交易所規則取整，也估不出強平價")
		assert.Contains(t, message, "每次結算約付 0.5（估算）")
		assert.Contains(t, message, "止損 98（往下")
		assert.Contains(t, message, "止盈 104（往上")
		assert.NotContains(t, message, "數量")
	})

	t.Run("the rough rule warns of a close-out", func(t *testing.T) {
		positionPlanDto := planOnVenue(t, aPrecisionPlan(10, "10", "0"), vo.TargetPositionLong, "100", unspecified)

		message := domains.NewStrategyBotMessageDomain(aPrecisionRound(positionPlanDto)).Text()
		assert.Contains(t, message, "止損距離乘上槓桿已達 100%：還沒到止損就會先被強制平倉")
	})
}

// PlanOnContractVenue answers like PlanFor when there is nothing to suggest or the stake is unaffordable.
func TestPositionPlanDomainOnAContractVenueSuggestsNothingPlanForWouldNot(t *testing.T) {
	positionPlan, _ := domains.NewPositionPlanDomain(aPrecisionPlan(5, "2", "4"))
	venue := aVenue(aSpecifiedContract("0.01"), nil, "")

	_, suggests := positionPlan.PlanOnContractVenue(
		vo.TargetPositionFlat, decimal.NewFromInt(100), precisionReferenceTime, venue)
	assert.False(t, suggests)
	assert.False(t, positionPlan.NeedsVenue(vo.TargetPositionFlat, true))
	assert.False(t, positionPlan.NeedsVenue(vo.TargetPositionLong, false))
	assert.True(t, positionPlan.NeedsVenue(vo.TargetPositionShort, true))

	fixedTwoThousand := aPrecisionPlan(5, "2", "4")
	fixedTwoThousand.SizingMode = string(vo.PositionSizingModeFixedAmount)
	fixedTwoThousand.SizingValue = decimal.NewFromInt(2000)
	unaffordable := planOnVenue(t, fixedTwoThousand, vo.TargetPositionLong, "100", venue)
	assert.False(t, unaffordable.Affordable)
	unaffordablePlan, _ := domains.NewPositionPlanDomain(fixedTwoThousand)
	assert.False(t, unaffordablePlan.NeedsVenue(vo.TargetPositionLong, true))
}

// aPrecisionRound is a long-and-short contract bot round that concluded buy.
func aPrecisionRound(positionPlanDto dto.PositionPlanDto) dto.StrategyBotRoundDto {
	return dto.StrategyBotRoundDto{
		BotName: "合約突破", Symbol: "BTCUSDT",
		MarketDataKind: "contractKCandle", ContractTradingMode: "longShort",
		Verdict:        string(vo.SignalBuy),
		ReferencePrice: decimal.NewFromInt(100), ReferenceTime: precisionReferenceTime, HasReference: true,
		PositionPlan: positionPlanDto, HasPositionPlan: true,
	}
}

func TestStrategyBotMessageKeepsASpotSuggestionAsItWas(t *testing.T) {
	positionPlan, _ := domains.NewPositionPlanDomain(dto.PositionPlanSettingsDto{
		Capital: decimal.NewFromInt(1000), StopLossPercentage: decimal.NewFromInt(2),
	})
	positionPlanDto, _ := positionPlan.PlanFor(vo.TargetPositionLong, decimal.NewFromInt(100), true)
	round := aPrecisionRound(positionPlanDto)
	round.MarketDataKind = "kCandle"
	round.ContractTradingMode = ""

	message := domains.NewStrategyBotMessageDomain(round).Text()

	assert.Contains(t, message, "　・開倉金額 1000\n　・止損 98（往下，虧 20）\n　⚠️ 回測要算進止損止盈")
	for _, contractOnly := range []string{"數量", "預估強平價", "資金費率", "交易規格"} {
		assert.False(t, strings.Contains(message, contractOnly), "現貨訊息不該有 %s", contractOnly)
	}
	assert.False(t, positionPlanDto.ForContract)
}
