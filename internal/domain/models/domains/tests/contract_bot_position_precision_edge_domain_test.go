package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A zero reference price yields a plan without quantity or liquidation price rather than dividing by zero.
func TestPositionPlanDomainSuggestsWithoutTheVenueAtAReferencePriceOfNothing(t *testing.T) {
	require.NotPanics(t, func() {
		positionPlanDto := planOnVenue(t, aPrecisionPlan(5, "2", "4"), vo.TargetPositionLong, "0",
			aVenue(aSpecifiedContract("0.01"), nil, "0.0001"))

		assert.True(t, positionPlanDto.ForContract)
		assert.False(t, positionPlanDto.HasQuantity)
		assert.False(t, positionPlanDto.HasLiquidationPrice)
		assert.False(t, positionPlanDto.HasVenueRefusal)
		assert.True(t, positionPlanDto.HasFundingRate)
	})
}

// The close-out warning uses the unrounded liquidation price like the replay: a 5x short from 100 liquidates at 119.40298…, so a stop at the printed 119.4 fires first and must not warn.
func TestPositionPlanDomainWarnsOfACloseOutOnlyWhereTheReplayWouldCloseOut(t *testing.T) {
	testCases := []struct {
		name          string
		target        vo.TargetPositionVo
		stopLoss      string
		expectedStop  string
		expectedWarns bool
	}{
		{name: "a short stop on the printed estimate but inside the raw one", target: vo.TargetPositionShort,
			stopLoss: "19.4", expectedStop: "119.4", expectedWarns: false},
		{name: "a short stop past the raw estimate", target: vo.TargetPositionShort,
			stopLoss: "19.41", expectedStop: "119.41", expectedWarns: true},
		{name: "a long stop just below the raw estimate", target: vo.TargetPositionLong,
			stopLoss: "19.6", expectedStop: "80.4", expectedWarns: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			positionPlanDto := planOnVenue(t, aPrecisionPlan(5, testCase.stopLoss, "0"), testCase.target, "100",
				aVenue(aSpecifiedContract("0.01"), nil, ""))

			require.True(t, positionPlanDto.HasLiquidationPrice)
			assert.True(t, positionPlanDto.StopLossPrice.Equal(decimal.RequireFromString(testCase.expectedStop)),
				"止損價 %s", positionPlanDto.StopLossPrice)
			assert.Equal(t, testCase.expectedWarns, positionPlanDto.LiquidatesBeforeStop)
		})
	}
}

// A liquidation price that rounds to zero on the tick is reported as no close-out, never as a price of zero.
func TestPositionPlanDomainNeverPrintsALiquidationPriceOfNothing(t *testing.T) {
	settings := aPrecisionPlan(1, "50", "0")
	settings.Leverage = decimal.RequireFromString("1.001")

	positionPlanDto := planOnVenue(t, settings, vo.TargetPositionLong, "100",
		aVenue(aSpecifiedContract("1"), nil, ""))

	assert.True(t, positionPlanDto.CannotBeLiquidated)
	assert.False(t, positionPlanDto.HasLiquidationPrice)
	assert.False(t, positionPlanDto.LiquidatesBeforeStop)
	message := domains.NewStrategyBotMessageDomain(aPrecisionRound(positionPlanDto)).Text()
	assert.Contains(t, message, "這個槓桿下不會被強制平倉")
	assert.NotContains(t, message, "預估強平價 0")
}
