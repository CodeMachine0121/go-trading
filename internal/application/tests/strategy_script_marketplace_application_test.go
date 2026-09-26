package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// strategyScriptOwnerID owns things; marketplaceGuestID only looks at them.
const marketplaceGuestID = uint(2)

var marketplaceNow = time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)

type marketplaceUnderTest struct {
	marketplaceApplication            *application.StrategyScriptMarketplaceApplication
	strategyScriptRepository          *mocks.MockIStrategyScriptRepository
	publishedStrategyScriptRepository *mocks.MockIPublishedStrategyScriptRepository
}

// newMarketplaceUnderTest wires the real domain service and access model, mocking only storage and
// the clock.
func newMarketplaceUnderTest(t *testing.T) marketplaceUnderTest {
	controller := gomock.NewController(t)
	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(marketplaceNow).AnyTimes()

	return marketplaceUnderTest{
		marketplaceApplication: application.NewStrategyScriptMarketplaceApplication(
			service.NewStrategyScriptMarketplaceService(
				strategyScriptRepository, publishedStrategyScriptRepository, clockProxy)),
		strategyScriptRepository:          strategyScriptRepository,
		publishedStrategyScriptRepository: publishedStrategyScriptRepository,
	}
}

func TestStrategyScriptMarketplacePublishStrategyScript(t *testing.T) {
	t.Run("puts the owner's strategy script on the marketplace as of now", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScript(7, "二十根均線"), nil)
		fixture.publishedStrategyScriptRepository.EXPECT().
			Publish(gomock.Any(), uint(7), marketplaceNow).Return(nil)

		require.NoError(t, fixture.marketplaceApplication.PublishStrategyScript(
			t.Context(), strategyScriptOwnerID, 7))
	})

	t.Run("refuses somebody else's strategy script as one that is not there", func(t *testing.T) {
		// Nothing is stubbed on the publication store: nothing may be written.
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScript(7, "二十根均線"), nil)

		err := fixture.marketplaceApplication.PublishStrategyScript(t.Context(), marketplaceGuestID, 7)

		require.ErrorIs(t, err, domains.ErrStrategyScriptNotFound)
	})

	t.Run("refuses a strategy script that does not exist", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(entities.StrategyScript{}, domains.ErrStrategyScriptNotFound)

		err := fixture.marketplaceApplication.PublishStrategyScript(t.Context(), strategyScriptOwnerID, 7)

		require.ErrorIs(t, err, domains.ErrStrategyScriptNotFound)
	})
}

func TestStrategyScriptMarketplaceWithdrawStrategyScript(t *testing.T) {
	t.Run("takes the owner's strategy script off the marketplace", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScript(7, "二十根均線"), nil)
		fixture.publishedStrategyScriptRepository.EXPECT().Withdraw(gomock.Any(), uint(7)).Return(nil)

		require.NoError(t, fixture.marketplaceApplication.WithdrawStrategyScript(
			t.Context(), strategyScriptOwnerID, 7))
	})

	t.Run("refuses somebody else's strategy script as one that is not there", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScript(7, "二十根均線"), nil)

		err := fixture.marketplaceApplication.WithdrawStrategyScript(t.Context(), marketplaceGuestID, 7)

		require.ErrorIs(t, err, domains.ErrStrategyScriptNotFound)
	})
}

func TestStrategyScriptMarketplaceBrowseMarketplace(t *testing.T) {
	t.Run("hands back what is out there, without any algorithm", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().FindAllPublished(gomock.Any()).
			Return([]entities.PublishedStrategyScript{aPublication(2, "別人的", 8)}, nil)

		publishedStrategyScriptDtos, err := fixture.marketplaceApplication.BrowseMarketplace(t.Context())

		require.NoError(t, err)
		require.Len(t, publishedStrategyScriptDtos, 1)
		assert.Equal(t, "別人的", publishedStrategyScriptDtos[0].Name)
		assert.Equal(t, "someone@example.com", publishedStrategyScriptDtos[0].PublisherEmail)
		assert.False(t, publishedStrategyScriptDtos[0].PublishedAt.IsZero())
	})

	t.Run("an empty shelf is an answer, not a failure", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().FindAllPublished(gomock.Any()).
			Return([]entities.PublishedStrategyScript{}, nil)

		publishedStrategyScriptDtos, err := fixture.marketplaceApplication.BrowseMarketplace(t.Context())

		require.NoError(t, err)
		assert.NotNil(t, publishedStrategyScriptDtos)
		assert.Empty(t, publishedStrategyScriptDtos)
	})

	t.Run("reports a storage failure", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		storageFailure := errors.New("connection refused")
		fixture.strategyScriptRepository.EXPECT().FindAllPublished(gomock.Any()).Return(nil, storageFailure)

		_, err := fixture.marketplaceApplication.BrowseMarketplace(t.Context())

		require.ErrorIs(t, err, storageFailure)
	})
}

// aPublishedOriginal is somebody else's script on the marketplace, with a knob, as adopting finds it.
func aPublishedOriginal() entities.StrategyScript {
	original := aStoredStrategyScript(7, "二十根均線")
	original.OwnerID = strategyScriptOwnerID
	original.Description = "二十根收盤價的平均"
	original.Parameters = []entities.StrategyScriptParameter{
		{ID: 70, StrategyScriptID: 7, Name: "lookback", Kind: "lookbackCount", DefaultValue: 20},
	}
	original.Publication = &entities.PublishedStrategyScript{StrategyScriptID: 7, PublishedAt: marketplaceNow}

	return original
}

func TestStrategyScriptMarketplaceAdoptStrategyScript(t *testing.T) {
	t.Run("gives the adopter a copy of their own, marked as adopted", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aPublishedOriginal(), nil)
		fixture.strategyScriptRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, copied entities.StrategyScript) (entities.StrategyScript, error) {
				original := aPublishedOriginal()
				assert.Zero(t, copied.ID)
				assert.Equal(t, marketplaceGuestID, copied.OwnerID)
				assert.True(t, copied.IsAdoptedFromMarketplace)
				assert.Equal(t, original.Name, copied.Name)
				assert.Equal(t, original.Description, copied.Description)
				assert.Equal(t, original.Script, copied.Script)
				assert.Equal(t, original.ResultType, copied.ResultType)
				assert.Equal(t, original.MarketDataKind, copied.MarketDataKind)
				assert.Equal(t, []entities.StrategyScriptParameter{
					{Name: "lookback", Kind: "lookbackCount", DefaultValue: 20},
				}, copied.Parameters)
				assert.Nil(t, copied.Publication)
				assert.Equal(t, marketplaceNow, copied.CreatedAt)

				return copied, nil
			})

		require.NoError(t, fixture.marketplaceApplication.AdoptStrategyScript(t.Context(), marketplaceGuestID, 7))
	})

	t.Run("a name the adopter already holds is refused", func(t *testing.T) {
		// Adopting the same script twice lands here too.
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aPublishedOriginal(), nil)
		fixture.strategyScriptRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			Return(entities.StrategyScript{}, domains.ErrStrategyScriptNameConflict)

		err := fixture.marketplaceApplication.AdoptStrategyScript(t.Context(), marketplaceGuestID, 7)

		require.ErrorIs(t, err, domains.ErrStrategyScriptNameConflict)
	})

	t.Run("adopting one's own does nothing and is not a failure", func(t *testing.T) {
		// Save is unstubbed: nothing may be copied.
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aPublishedOriginal(), nil)

		require.NoError(t, fixture.marketplaceApplication.AdoptStrategyScript(t.Context(), strategyScriptOwnerID, 7))
	})

	t.Run("refuses a strategy script that does not exist", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(entities.StrategyScript{}, domains.ErrStrategyScriptNotFound)

		err := fixture.marketplaceApplication.AdoptStrategyScript(t.Context(), marketplaceGuestID, 7)

		require.ErrorIs(t, err, domains.ErrStrategyScriptNotFound)
	})

	t.Run("reports one that is not on the marketplace as one that is not there", func(t *testing.T) {
		// Save is unstubbed: an unpublished script is never copied.
		fixture := newMarketplaceUnderTest(t)
		unpublished := aPublishedOriginal()
		unpublished.Publication = nil
		fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(unpublished, nil)

		err := fixture.marketplaceApplication.AdoptStrategyScript(t.Context(), marketplaceGuestID, 7)

		require.ErrorIs(t, err, domains.ErrStrategyScriptNotFound)
	})
}

func TestStrategyScriptMarketplacePublishRefusesAMarketplaceCopy(t *testing.T) {
	// Publish is unstubbed: a copy never reaches the marketplace again.
	fixture := newMarketplaceUnderTest(t)
	copied := aStoredStrategyScript(8, "二十根均線")
	copied.IsAdoptedFromMarketplace = true
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(8)).Return(copied, nil)

	err := fixture.marketplaceApplication.PublishStrategyScript(t.Context(), strategyScriptOwnerID, 8)

	require.ErrorIs(t, err, domains.ErrStrategyScriptFromMarketplace)
	assert.Contains(t, err.Error(), "從市集加入的策略腳本不能再發佈")
}
