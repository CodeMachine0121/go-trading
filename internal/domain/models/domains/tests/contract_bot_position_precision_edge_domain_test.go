package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// A reference price of nothing is a price nothing can be opened at: the suggestion reads
// as it does without the venue, with no quantity and no liquidation price — and the round
// goes on rather than dividing by nothing.
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

// The close-out warning follows the replay's rule: judged against the unrounded
// liquidation price, and a stop exactly there fires first.
//
// A five-times short from 100 is closed out at 119.40298…, printed on the tick as 119.4.
// A stop at 119.4 sits below that raw figure, so the replay stops out first — and the bot
// must not warn of a close-out, even though the stop equals the printed estimate.
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

// A liquidation price the venue cannot quote above nothing — here a hair over one times,
// on a tick of one — is said to be no close-out at all, never printed as a price of zero.
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
