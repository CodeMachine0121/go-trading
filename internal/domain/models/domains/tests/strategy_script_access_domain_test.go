package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	accessOwnerID    = uint(1)
	accessStrangerID = uint(2)
)

// anOwnedStrategyScript has one parameter so listing visibility can be checked.
func anOwnedStrategyScript() entities.StrategyScript {
	return entities.StrategyScript{
		ID:          7,
		OwnerID:     accessOwnerID,
		Name:        "二十根均線",
		Description: "抓短線轉折",
		Script:      "func Calculate(candles []vo.KCandleVo) map[string]float64 { return nil }",
		ResultType:  "floatList",
		Owner:       entities.User{ID: accessOwnerID, Email: "owner@example.com"},
		Parameters: []entities.StrategyScriptParameter{
			{Name: "lookback", Kind: "lookbackCount", DefaultValue: 20},
		},
	}
}

func TestStrategyScriptAccessDomainRecognisesItsOwner(t *testing.T) {
	testCases := []struct {
		name          string
		viewerID      uint
		ownerID       uint
		expectedOwned bool
	}{
		{name: "the owner is the owner", viewerID: accessOwnerID, ownerID: accessOwnerID, expectedOwned: true},
		{name: "somebody else is not", viewerID: accessStrangerID, ownerID: accessOwnerID, expectedOwned: false},
		// Zero never owns anything, so a request with no identity cannot match a row whose
		// owner is zero.
		{name: "nobody owns nothing", viewerID: 0, ownerID: 0, expectedOwned: false},
		{name: "nobody owns somebody's", viewerID: 0, ownerID: accessOwnerID, expectedOwned: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			strategyScript := anOwnedStrategyScript()
			strategyScript.OwnerID = testCase.ownerID

			accessDomain := domains.NewStrategyScriptAccessDomain(strategyScript, testCase.viewerID, false)

			assert.Equal(t, testCase.expectedOwned, accessDomain.IsOwnedByViewer())
		})
	}
}

func TestStrategyScriptAccessDomainSaysWhoMayRunIt(t *testing.T) {
	testCases := []struct {
		name             string
		viewerID         uint
		isPublished      bool
		expectedRunnable bool
	}{
		{name: "the owner may run their own", viewerID: accessOwnerID, isPublished: false, expectedRunnable: true},
		{
			name:     "a stranger may run a published one",
			viewerID: accessStrangerID, isPublished: true, expectedRunnable: true,
		},
		{
			name:     "a stranger may not run an unpublished one",
			viewerID: accessStrangerID, isPublished: false, expectedRunnable: false,
		},
		{
			name:     "the owner may run their own once published too",
			viewerID: accessOwnerID, isPublished: true, expectedRunnable: true,
		},
		{name: "nobody may run anything", viewerID: 0, isPublished: false, expectedRunnable: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			accessDomain := domains.NewStrategyScriptAccessDomain(
				anOwnedStrategyScript(), testCase.viewerID, testCase.isPublished)

			assert.Equal(t, testCase.expectedRunnable, accessDomain.IsRunnable())
		})
	}
}

func TestStrategyScriptAccessDomainHandsTheWholeStrategyScriptToItsOwner(t *testing.T) {
	accessDomain := domains.NewStrategyScriptAccessDomain(anOwnedStrategyScript(), accessOwnerID, false)

	strategyScriptDto, readError := accessDomain.ToOwnerDto()

	require.NoError(t, readError)
	assert.Equal(t, "二十根均線", strategyScriptDto.Name)
	assert.Equal(t, "抓短線轉折", strategyScriptDto.Description)
	assert.Contains(t, strategyScriptDto.Script, "func Calculate")
	require.Len(t, strategyScriptDto.Parameters, 1)
	assert.Equal(t, "lookback", strategyScriptDto.Parameters[0].Name)
}

func TestStrategyScriptAccessDomainRefusesEveryoneElseWithTheOneRefusal(t *testing.T) {
	// Reading, changing and running an unpublished script all give the same refusal so
	// identifiers cannot be probed.
	strangerReading := domains.NewStrategyScriptAccessDomain(anOwnedStrategyScript(), accessStrangerID, false)
	strangerReadingPublished := domains.NewStrategyScriptAccessDomain(anOwnedStrategyScript(), accessStrangerID, true)

	_, readError := strangerReading.ToOwnerDto()
	_, readPublishedError := strangerReadingPublished.ToOwnerDto()
	ownershipError := strangerReading.RequireOwnership()
	_, runError := strangerReading.ToRunnableDto()

	require.ErrorIs(t, readError, domains.ErrStrategyScriptNotFound)
	require.ErrorIs(t, readPublishedError, domains.ErrStrategyScriptNotFound,
		"publishing hands out the use of a strategy script, never the right to read it whole")
	require.ErrorIs(t, ownershipError, domains.ErrStrategyScriptNotFound)
	require.ErrorIs(t, runError, domains.ErrStrategyScriptNotFound)
	assert.Equal(t, readError.Error(), readPublishedError.Error())
	assert.Equal(t, readError.Error(), ownershipError.Error())
	assert.Equal(t, readError.Error(), runError.Error())
}

func TestStrategyScriptAccessDomainResolvesWhatARunNeeds(t *testing.T) {
	t.Run("for the owner", func(t *testing.T) {
		accessDomain := domains.NewStrategyScriptAccessDomain(anOwnedStrategyScript(), accessOwnerID, false)

		runnableStrategyScriptDto, resolveError := accessDomain.ToRunnableDto()

		require.NoError(t, resolveError)
		assert.Contains(t, runnableStrategyScriptDto.Script, "func Calculate")
		assert.Equal(t, "floatList", runnableStrategyScriptDto.ResultType)
		require.Len(t, runnableStrategyScriptDto.Parameters, 1)
		assert.Equal(t, "lookback", runnableStrategyScriptDto.Parameters[0].Name)
		assert.InDelta(t, 20.0, runnableStrategyScriptDto.Parameters[0].DefaultValue, 0)
	})

	t.Run("for a stranger, when it is published", func(t *testing.T) {
		// The runnable form must contain the script, or published scripts would be unusable
		// rather than just unreadable.
		accessDomain := domains.NewStrategyScriptAccessDomain(anOwnedStrategyScript(), accessStrangerID, true)

		runnableStrategyScriptDto, resolveError := accessDomain.ToRunnableDto()

		require.NoError(t, resolveError)
		assert.Contains(t, runnableStrategyScriptDto.Script, "func Calculate")
	})
}
