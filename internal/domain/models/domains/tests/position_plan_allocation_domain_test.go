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

func TestPositionPlanDomainSizesWithinASymbolsShare(t *testing.T) {
	testCases := []struct {
		name                 string
		settings             dto.PositionPlanSettingsDto
		allocationPercentage string
		expectedStake        string
		expectedAffordable   bool
		expectedPartial      bool
		expectedAllocated    string
	}{
		{name: "all in stakes the whole share",
			settings:             dto.PositionPlanSettingsDto{Capital: decimal.NewFromInt(150)},
			allocationPercentage: "50", expectedStake: "75", expectedAffordable: true, expectedPartial: true, expectedAllocated: "75"},
		{name: "a percentage is taken of the share",
			settings: dto.PositionPlanSettingsDto{Capital: decimal.NewFromInt(1000),
				SizingMode: string(vo.PositionSizingModePercentage), SizingValue: decimal.NewFromInt(10)},
			allocationPercentage: "40", expectedStake: "40", expectedAffordable: true, expectedPartial: true, expectedAllocated: "400"},
		{name: "a fixed amount past the share cannot be staked",
			settings: dto.PositionPlanSettingsDto{Capital: decimal.NewFromInt(150),
				SizingMode: string(vo.PositionSizingModeFixedAmount), SizingValue: decimal.NewFromInt(50)},
			allocationPercentage: "33.33", expectedStake: "50", expectedAffordable: false, expectedPartial: true, expectedAllocated: "49.995"},
		{name: "the whole capital reads as no share at all",
			settings: dto.PositionPlanSettingsDto{Capital: decimal.NewFromInt(50),
				SizingMode: string(vo.PositionSizingModeFixedAmount), SizingValue: decimal.NewFromInt(50)},
			allocationPercentage: "100", expectedStake: "50", expectedAffordable: true, expectedPartial: false, expectedAllocated: "0"},
		{name: "no share leaves the plan whole",
			settings:             dto.PositionPlanSettingsDto{Capital: decimal.NewFromInt(150)},
			allocationPercentage: "0", expectedStake: "150", expectedAffordable: true, expectedPartial: false, expectedAllocated: "0"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			positionPlan, buildError := domains.NewPositionPlanDomain(testCase.settings)
			require.NoError(t, buildError)

			positionPlanDto, suggests := positionPlan.
				AllocatedTo(decimal.RequireFromString(testCase.allocationPercentage)).
				PlanFor(vo.TargetPositionLong, aReferencePrice(), true)

			require.True(t, suggests)
			assert.Equal(t, testCase.expectedStake, positionPlanDto.Stake.String())
			assert.Equal(t, testCase.expectedAffordable, positionPlanDto.Affordable)
			assert.Equal(t, testCase.expectedPartial, positionPlanDto.PartiallyAllocated)
			assert.Equal(t, testCase.expectedAllocated, positionPlanDto.AllocatedCapital.String())
		})
	}
}

func TestPositionPlanDomainLeveragesTheShareAsMargin(t *testing.T) {
	positionPlan, buildError := domains.NewPositionPlanDomain(dto.PositionPlanSettingsDto{
		Capital: decimal.NewFromInt(100), Leverage: decimal.NewFromInt(5),
	})
	require.NoError(t, buildError)

	positionPlanDto, suggests := positionPlan.AllocatedTo(decimal.NewFromInt(50)).
		PlanFor(vo.TargetPositionLong, aReferencePrice(), true)

	require.True(t, suggests)
	assert.Equal(t, "50", positionPlanDto.Stake.String())
	assert.Equal(t, "250", positionPlanDto.Notional.String())
	assert.Equal(t, "100", positionPlanDto.TotalCapital.String())
	assert.Equal(t, "50", positionPlanDto.AllocationPercentage.String())
}

func TestStrategyBotMessageNamesTheShareOnlyWhenItIsNotTheWhole(t *testing.T) {
	shared := suggestingBotRound()
	shared.PositionPlan.PartiallyAllocated = true
	shared.PositionPlan.AllocationPercentage = decimal.RequireFromString("33.33")
	shared.PositionPlan.TotalCapital = decimal.NewFromInt(1000)
	shared.PositionPlan.AllocatedCapital = decimal.RequireFromString("333.3")

	assert.Contains(t, domains.NewStrategyBotMessageDomain(shared).Text(), "　・資金分配 33.33%：總金額 1000 中的 333.3")
	assert.NotContains(t, domains.NewStrategyBotMessageDomain(suggestingBotRound()).Text(), "資金分配")
}
