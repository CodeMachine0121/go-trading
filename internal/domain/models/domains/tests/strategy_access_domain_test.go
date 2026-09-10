package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// The two people in every case below. Neither number means anything beyond "not the
// other one".
const (
	accessOwnerID    = uint(1)
	accessStrangerID = uint(2)
)

// anOwnedStrategy is a strategy belonging to accessOwnerID, with one knob so that
// what a marketplace listing may show can be told from what it may not.
func anOwnedStrategy() entities.Strategy {
	return entities.Strategy{
		ID:          7,
		OwnerID:     accessOwnerID,
		Name:        "二十根均線",
		Description: "抓短線轉折",
		Script:      "func Calculate(candles []vo.KCandleVo) map[string]float64 { return nil }",
		ResultType:  "floatList",
		Owner:       entities.User{ID: accessOwnerID, Email: "owner@example.com"},
		Parameters: []entities.StrategyParameter{
			{Name: "lookback", Kind: "lookbackCount", DefaultValue: 20},
		},
	}
}

func TestStrategyAccessDomainRecognisesItsOwner(t *testing.T) {
	testCases := []struct {
		name          string
		viewerID      uint
		ownerID       uint
		expectedOwned bool
	}{
		{name: "the owner is the owner", viewerID: accessOwnerID, ownerID: accessOwnerID, expectedOwned: true},
		{name: "somebody else is not", viewerID: accessStrangerID, ownerID: accessOwnerID, expectedOwned: false},
		// Nobody owns nothing. Saying so here rather than trusting every caller to
		// have checked is what keeps a request with no identity from matching a row
		// whose owner column somehow holds zero.
		{name: "nobody owns nothing", viewerID: 0, ownerID: 0, expectedOwned: false},
		{name: "nobody owns somebody's", viewerID: 0, ownerID: accessOwnerID, expectedOwned: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			strategy := anOwnedStrategy()
			strategy.OwnerID = testCase.ownerID

			accessDomain := domains.NewStrategyAccessDomain(strategy, testCase.viewerID, false)

			assert.Equal(t, testCase.expectedOwned, accessDomain.IsOwnedByViewer())
		})
	}
}

func TestStrategyAccessDomainSaysWhoMayRunIt(t *testing.T) {
	testCases := []struct {
		name             string
		viewerID         uint
		isPublished      bool
		expectedRunnable bool
	}{
		{name: "the owner may run their own", viewerID: accessOwnerID, isPublished: false, expectedRunnable: true},
		{
			name: "a stranger may run a published one",
			viewerID: accessStrangerID, isPublished: true, expectedRunnable: true,
		},
		{
			name: "a stranger may not run an unpublished one",
			viewerID: accessStrangerID, isPublished: false, expectedRunnable: false,
		},
		{
			name: "the owner may run their own once published too",
			viewerID: accessOwnerID, isPublished: true, expectedRunnable: true,
		},
		{name: "nobody may run anything", viewerID: 0, isPublished: false, expectedRunnable: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			accessDomain := domains.NewStrategyAccessDomain(
				anOwnedStrategy(), testCase.viewerID, testCase.isPublished)

			assert.Equal(t, testCase.expectedRunnable, accessDomain.IsRunnable())
		})
	}
}

func TestStrategyAccessDomainHandsTheWholeStrategyToItsOwner(t *testing.T) {
	accessDomain := domains.NewStrategyAccessDomain(anOwnedStrategy(), accessOwnerID, false)

	strategyDto, readError := accessDomain.ToOwnerDto()

	require.NoError(t, readError)
	assert.Equal(t, "二十根均線", strategyDto.Name)
	assert.Equal(t, "抓短線轉折", strategyDto.Description)
	assert.Contains(t, strategyDto.Script, "func Calculate")
	require.Len(t, strategyDto.Parameters, 1)
	assert.Equal(t, "lookback", strategyDto.Parameters[0].Name)
}

func TestStrategyAccessDomainRefusesEveryoneElseWithTheOneRefusal(t *testing.T) {
	// Reading somebody else's, changing it, and running an unpublished one are three
	// different attempts, and all three owe the same sentence — otherwise a caller
	// holding a list of identifiers could tell which of them exist by comparing how
	// the refusals differ.
	strangerReading := domains.NewStrategyAccessDomain(anOwnedStrategy(), accessStrangerID, false)
	strangerReadingPublished := domains.NewStrategyAccessDomain(anOwnedStrategy(), accessStrangerID, true)

	_, readError := strangerReading.ToOwnerDto()
	_, readPublishedError := strangerReadingPublished.ToOwnerDto()
	ownershipError := strangerReading.RequireOwnership()
	_, runError := strangerReading.ToRunnableDto()

	require.ErrorIs(t, readError, domains.ErrStrategyNotFound)
	require.ErrorIs(t, readPublishedError, domains.ErrStrategyNotFound,
		"publishing hands out the use of a strategy, never the right to read it whole")
	require.ErrorIs(t, ownershipError, domains.ErrStrategyNotFound)
	require.ErrorIs(t, runError, domains.ErrStrategyNotFound)
	assert.Equal(t, readError.Error(), readPublishedError.Error())
	assert.Equal(t, readError.Error(), ownershipError.Error())
	assert.Equal(t, readError.Error(), runError.Error())
}

func TestStrategyAccessDomainResolvesWhatARunNeeds(t *testing.T) {
	t.Run("for the owner", func(t *testing.T) {
		accessDomain := domains.NewStrategyAccessDomain(anOwnedStrategy(), accessOwnerID, false)

		runnableStrategyDto, resolveError := accessDomain.ToRunnableDto()

		require.NoError(t, resolveError)
		assert.Contains(t, runnableStrategyDto.Script, "func Calculate")
		assert.Equal(t, "floatList", runnableStrategyDto.ResultType)
		require.Len(t, runnableStrategyDto.Parameters, 1)
		assert.Equal(t, "lookback", runnableStrategyDto.Parameters[0].Name)
		assert.InDelta(t, 20.0, runnableStrategyDto.Parameters[0].DefaultValue, 0)
	})

	t.Run("for a stranger, when it is published", func(t *testing.T) {
		// The stranger never sees this — it goes to the layer that runs it — but the
		// algorithm has to be in it, or a published strategy would be unusable
		// rather than merely unreadable.
		accessDomain := domains.NewStrategyAccessDomain(anOwnedStrategy(), accessStrangerID, true)

		runnableStrategyDto, resolveError := accessDomain.ToRunnableDto()

		require.NoError(t, resolveError)
		assert.Contains(t, runnableStrategyDto.Script, "func Calculate")
	})
}

func TestStrategyAccessDomainShowsAMarketplaceListingWithoutTheAlgorithm(t *testing.T) {
	publishedAt := time.Date(2026, 9, 10, 8, 0, 0, 0, time.FixedZone("Asia/Taipei", 8*60*60))
	accessDomain := domains.NewStrategyAccessDomain(anOwnedStrategy(), accessStrangerID, true)

	publishedStrategyDto := accessDomain.ToPublishedDto(publishedAt)

	assert.Equal(t, uint(7), publishedStrategyDto.ID)
	assert.Equal(t, "二十根均線", publishedStrategyDto.Name)
	assert.Equal(t, "抓短線轉折", publishedStrategyDto.Description)
	assert.Equal(t, "floatList", publishedStrategyDto.ResultType)
	assert.Equal(t, "owner@example.com", publishedStrategyDto.PublisherEmail)
	assert.Equal(t, publishedAt.UTC(), publishedStrategyDto.PublishedAt,
		"a moment is handed out in universal time whatever zone it was read back in")
	require.Len(t, publishedStrategyDto.Parameters, 1,
		"a knob is a name and a default, not a step — declaring it gives nothing away")
	assert.Equal(t, "lookback", publishedStrategyDto.Parameters[0].Name)
}
