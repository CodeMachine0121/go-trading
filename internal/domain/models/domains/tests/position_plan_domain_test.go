package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// aPositionPlanSettings is a fully filled plan: fifty thousand, staking a tenth of it,
// out at three percent against and five percent in favour.
func aPositionPlanSettings() dto.PositionPlanSettingsDto {
	return dto.PositionPlanSettingsDto{
		Capital:              decimal.NewFromInt(50000),
		SizingMode:           string(vo.PositionSizingModePercentage),
		SizingValue:          decimal.NewFromInt(10),
		StopLossPercentage:   decimal.NewFromInt(3),
		TakeProfitPercentage: decimal.NewFromInt(5),
	}
}

// aReferencePrice is the price every figure below is measured from.
func aReferencePrice() decimal.Decimal {
	return decimal.RequireFromString("64180.5")
}

// planUnderTest builds the plan and asks it about one round, failing the test if the
// settings themselves were refused — every case below that expects a refusal asks for
// one on its own.
func planUnderTest(
	t *testing.T, settings dto.PositionPlanSettingsDto, target vo.TargetPositionVo,
) (dto.PositionPlanDto, bool) {
	t.Helper()

	positionPlan, buildError := domains.NewPositionPlanDomain(settings)
	require.NoError(t, buildError)

	return positionPlan.PlanFor(target, aReferencePrice(), true)
}

// The multiplications, written out. They are here with real figures rather than
// described in prose because the arithmetic is the whole of what this model does, and
// a formula restated in a test is a formula that agrees with itself.
func TestPositionPlanDomainSizesAPosition(t *testing.T) {
	positionPlanDto, suggests := planUnderTest(
		t, aPositionPlanSettings(), vo.TargetPositionLong)

	require.True(t, suggests)
	assert.True(t, positionPlanDto.Affordable)
	// A tenth of fifty thousand.
	assert.Equal(t, "5000", positionPlanDto.Stake.String())
	// The price falling is the loss — so the stop sits below, and the target above.
	assert.Equal(t, "62255.085", positionPlanDto.StopLossPrice.String())
	assert.Equal(t, "67389.525", positionPlanDto.TakeProfitPrice.String())
	assert.True(t, positionPlanDto.StopLossPrice.LessThan(aReferencePrice()))
	assert.True(t, positionPlanDto.TakeProfitPrice.GreaterThan(aReferencePrice()))
	// Measured against the stake, which is the whole of what is in the market.
	assert.Equal(t, "150", positionPlanDto.LossAtStop.String())
	assert.Equal(t, "250", positionPlanDto.GainAtTarget.String())
}

func TestPositionPlanDomainReadsEachSettingItWasGiven(t *testing.T) {
	testCases := []struct {
		name             string
		adjust           func(settings *dto.PositionPlanSettingsDto)
		expectedStake    string
		expectStopLoss   bool
		expectTakeProfit bool
	}{
		{
			name: "staking everything needs no figure",
			adjust: func(settings *dto.PositionPlanSettingsDto) {
				settings.SizingMode = ""
				settings.SizingValue = decimal.Zero
			},
			expectedStake:    "50000",
			expectStopLoss:   true,
			expectTakeProfit: true,
		},
		{
			name: "a fixed amount is that amount, whatever the capital is",
			adjust: func(settings *dto.PositionPlanSettingsDto) {
				settings.SizingMode = string(vo.PositionSizingModeFixedAmount)
				settings.SizingValue = decimal.NewFromInt(8000)
			},
			expectedStake:    "8000",
			expectStopLoss:   true,
			expectTakeProfit: true,
		},
		{
			name: "a stop on its own",
			adjust: func(settings *dto.PositionPlanSettingsDto) {
				settings.TakeProfitPercentage = decimal.Zero
			},
			expectedStake:    "5000",
			expectStopLoss:   true,
			expectTakeProfit: false,
		},
		{
			name: "a target on its own",
			adjust: func(settings *dto.PositionPlanSettingsDto) {
				settings.StopLossPercentage = decimal.Zero
			},
			expectedStake:    "5000",
			expectStopLoss:   false,
			expectTakeProfit: true,
		},
		{
			name: "neither exit at all still suggests a size",
			adjust: func(settings *dto.PositionPlanSettingsDto) {
				settings.StopLossPercentage = decimal.Zero
				settings.TakeProfitPercentage = decimal.Zero
			},
			expectedStake:    "5000",
			expectStopLoss:   false,
			expectTakeProfit: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			settings := aPositionPlanSettings()
			testCase.adjust(&settings)

			positionPlanDto, suggests := planUnderTest(t, settings, vo.TargetPositionLong)

			require.True(t, suggests)
			assert.Equal(t, testCase.expectedStake, positionPlanDto.Stake.String())
			assert.Equal(t, testCase.expectStopLoss, positionPlanDto.HasStopLoss)
			assert.Equal(t, testCase.expectTakeProfit, positionPlanDto.HasTakeProfit)
		})
	}
}

// A fixed amount the capital cannot cover is the same ordinary situation a replay
// skips an opening for. Printing a stake nobody can put down would be worse than
// printing none, so the plan says so instead.
func TestPositionPlanDomainSaysSoWhenTheCapitalCannotCoverTheStake(t *testing.T) {
	settings := aPositionPlanSettings()
	settings.Capital = decimal.NewFromInt(5000)
	settings.SizingMode = string(vo.PositionSizingModeFixedAmount)
	settings.SizingValue = decimal.NewFromInt(8000)

	positionPlanDto, suggests := planUnderTest(t, settings, vo.TargetPositionLong)

	require.True(t, suggests, "有設定就有建議，只是那個建議是「押不下去」")
	assert.False(t, positionPlanDto.Affordable)
	// Nothing computed on top of a stake that cannot be put down.
	assert.False(t, positionPlanDto.HasStopLoss)
	assert.False(t, positionPlanDto.HasTakeProfit)
}

// Four ways to have nothing to suggest, deliberately one answer. To whoever reads the
// message, "this round has nothing to put down" is a single fact — four sentences
// about it would grow four ways of writing the same paragraph.
func TestPositionPlanDomainSuggestsNothingWhenThereIsNothingToSuggest(t *testing.T) {
	testCases := []struct {
		name         string
		settings     dto.PositionPlanSettingsDto
		target       vo.TargetPositionVo
		hasReference bool
	}{
		{
			name:         "no capital was ever set",
			settings:     dto.PositionPlanSettingsDto{},
			target:       vo.TargetPositionLong,
			hasReference: true,
		},
		{
			// A sell clears out. There is nothing to size, and a suggestion here would
			// have somebody putting money down to close a position.
			name:         "the round is asking to stand aside",
			settings:     aPositionPlanSettings(),
			target:       vo.TargetPositionFlat,
			hasReference: true,
		},
		{
			name:         "the round is asking for no change",
			settings:     aPositionPlanSettings(),
			target:       vo.TargetPositionUnchanged,
			hasReference: true,
		},
		{
			// Every exit is measured from the price. Without one there is nothing to
			// measure from, and a figure off some other price would look just as real.
			name:         "there is no price to measure from",
			settings:     aPositionPlanSettings(),
			target:       vo.TargetPositionLong,
			hasReference: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			positionPlan, buildError := domains.NewPositionPlanDomain(testCase.settings)
			require.NoError(t, buildError)

			positionPlanDto, suggests := positionPlan.PlanFor(
				testCase.target, aReferencePrice(), testCase.hasReference)

			assert.False(t, suggests)
			assert.Equal(t, dto.PositionPlanDto{}, positionPlanDto)
		})
	}
}

// A plan that never went through the constructor suggests nothing: a model that
// cannot read itself is the last place to size somebody's position.
func TestPositionPlanDomainZeroValueSuggestsNothing(t *testing.T) {
	_, suggests := domains.PositionPlanDomain{}.PlanFor(
		vo.TargetPositionLong, aReferencePrice(), true)

	assert.False(t, suggests)
}

func TestNewPositionPlanDomainRefusesSettingsItCannotUse(t *testing.T) {
	testCases := []struct {
		name          string
		adjust        func(settings *dto.PositionPlanSettingsDto)
		expectedWords string
	}{
		{
			name: "a percentage above a hundred",
			adjust: func(settings *dto.PositionPlanSettingsDto) {
				settings.SizingValue = decimal.NewFromInt(150)
			},
			// The replay's own sentence, carried through rather than reworded.
			expectedWords: "百分比必須大於零且不超過一百",
		},
		{
			name: "a fixed amount of nothing",
			adjust: func(settings *dto.PositionPlanSettingsDto) {
				settings.SizingMode = string(vo.PositionSizingModeFixedAmount)
				settings.SizingValue = decimal.Zero
			},
			expectedWords: "固定金額必須大於零",
		},
		{
			name: "a sizing mode nobody offers",
			adjust: func(settings *dto.PositionPlanSettingsDto) {
				settings.SizingMode = "dayTrade"
			},
			expectedWords: "每次開倉押多少只能是",
		},
		{
			// Somebody who typed half a times meant something by it. Reading that as a
			// whole one would double what they asked for without telling them.
			name: "leverage below one times",
			adjust: func(settings *dto.PositionPlanSettingsDto) {
				settings.Leverage = decimal.RequireFromString("0.5")
			},
			expectedWords: "槓桿倍數不得小於 1 倍",
		},
		{
			// Nothing here lends, so a bot may only ever suggest what a replay could
			// have modelled. The refusal is the replay's own sentence.
			name: "leverage above one times",
			adjust: func(settings *dto.PositionPlanSettingsDto) {
				settings.Leverage = decimal.NewFromInt(20)
			},
			expectedWords: "沒有人借錢給你",
		},
		{
			name: "a negative stop distance",
			adjust: func(settings *dto.PositionPlanSettingsDto) {
				settings.StopLossPercentage = decimal.NewFromInt(-3)
			},
			expectedWords: "停損距離不得為負",
		},
		{
			name: "a stop distance past the whole price",
			adjust: func(settings *dto.PositionPlanSettingsDto) {
				settings.StopLossPercentage = decimal.NewFromInt(120)
			},
			expectedWords: "停損距離不得超過 100%",
		},
		{
			name: "a negative target distance",
			adjust: func(settings *dto.PositionPlanSettingsDto) {
				settings.TakeProfitPercentage = decimal.NewFromInt(-5)
			},
			expectedWords: "停利距離不得為負",
		},
		{
			name: "a target distance past the whole price",
			adjust: func(settings *dto.PositionPlanSettingsDto) {
				settings.TakeProfitPercentage = decimal.NewFromInt(120)
			},
			expectedWords: "停利距離不得超過 100%",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			settings := aPositionPlanSettings()
			testCase.adjust(&settings)

			_, buildError := domains.NewPositionPlanDomain(settings)

			require.Error(t, buildError)
			assert.Contains(t, buildError.Error(), testCase.expectedWords)
		})
	}
}

// Not filling something in is not the same as filling it in wrongly. A bot with no
// capital has no plan, and the four settings beside it are not judged at all — they
// have nothing to apply to.
func TestNewPositionPlanDomainAcceptsNoCapitalAsNoPlan(t *testing.T) {
	settings := aPositionPlanSettings()
	settings.Capital = decimal.Zero
	settings.SizingValue = decimal.NewFromInt(150)

	positionPlan, buildError := domains.NewPositionPlanDomain(settings)

	require.NoError(t, buildError, "沒填不是填錯")

	_, suggests := positionPlan.PlanFor(vo.TargetPositionLong, aReferencePrice(), true)
	assert.False(t, suggests)
}

// A distance of the entire price puts a long's stop at exactly zero. Absurd, but it is
// arithmetic — and drawing a line between ninety-nine and a hundred would need a
// reason nobody has given.
func TestPositionPlanDomainAllowsADistanceOfTheWholePrice(t *testing.T) {
	settings := aPositionPlanSettings()
	settings.StopLossPercentage = decimal.NewFromInt(100)

	positionPlanDto, suggests := planUnderTest(t, settings, vo.TargetPositionLong)

	require.True(t, suggests)
	assert.True(t, positionPlanDto.StopLossPrice.IsZero())
}

// The settings go back out the way they came in, because something has to store them.
func TestPositionPlanDomainHandsItsSettingsBack(t *testing.T) {
	positionPlan, buildError := domains.NewPositionPlanDomain(aPositionPlanSettings())
	require.NoError(t, buildError)

	settingsDto := positionPlan.ToSettingsDto()

	assert.Equal(t, "50000", settingsDto.Capital.String())
	assert.Equal(t, string(vo.PositionSizingModePercentage), settingsDto.SizingMode)
	assert.Equal(t, "10", settingsDto.SizingValue.String())
	assert.Equal(t, "3", settingsDto.StopLossPercentage.String())
	assert.Equal(t, "5", settingsDto.TakeProfitPercentage.String())
}

// A bot with no position plan stores nothing at all.
func TestPositionPlanDomainStoresNothingForABotWithNoPlan(t *testing.T) {
	plan, buildError := domains.NewPositionPlanDomain(dto.PositionPlanSettingsDto{})
	require.NoError(t, buildError)

	settings := plan.ToSettingsDto()

	assert.Equal(t, "0", settings.Capital.String())
	assert.Equal(t, "", settings.SizingMode)
	assert.Equal(t, "0", settings.SizingValue.String())
	assert.Equal(t, "0", settings.StopLossPercentage.String())
	assert.Equal(t, "0", settings.TakeProfitPercentage.String())
}
