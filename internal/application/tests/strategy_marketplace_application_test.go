package application_test

import (
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

// The two people on the marketplace. strategyOwnerID owns things; marketplaceGuestID
// only ever looks at them.
const marketplaceGuestID = uint(2)

// marketplaceNow is when every publication and adoption below happens.
var marketplaceNow = time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)

type marketplaceUnderTest struct {
	marketplaceApplication      *application.StrategyMarketplaceApplication
	strategyRepository          *mocks.MockIStrategyRepository
	publishedStrategyRepository *mocks.MockIPublishedStrategyRepository
	strategyAdoptionRepository  *mocks.MockIStrategyAdoptionRepository
}

// newMarketplaceUnderTest wires the real domain service and the real access model,
// mocking only the outermost boundaries: storage and the clock.
func newMarketplaceUnderTest(t *testing.T) marketplaceUnderTest {
	controller := gomock.NewController(t)
	strategyRepository := mocks.NewMockIStrategyRepository(controller)
	publishedStrategyRepository := mocks.NewMockIPublishedStrategyRepository(controller)
	strategyAdoptionRepository := mocks.NewMockIStrategyAdoptionRepository(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(marketplaceNow).AnyTimes()

	return marketplaceUnderTest{
		marketplaceApplication: application.NewStrategyMarketplaceApplication(
			service.NewStrategyMarketplaceService(
				strategyRepository, publishedStrategyRepository, strategyAdoptionRepository, clockProxy)),
		strategyRepository:          strategyRepository,
		publishedStrategyRepository: publishedStrategyRepository,
		strategyAdoptionRepository:  strategyAdoptionRepository,
	}
}

func TestStrategyMarketplacePublishStrategy(t *testing.T) {
	t.Run("puts the owner's strategy on the marketplace as of now", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategy(7, "二十根均線"), nil)
		fixture.publishedStrategyRepository.EXPECT().
			Publish(gomock.Any(), uint(7), marketplaceNow).Return(nil)

		require.NoError(t, fixture.marketplaceApplication.PublishStrategy(
			t.Context(), strategyOwnerID, 7))
	})

	t.Run("refuses somebody else's strategy as one that is not there", func(t *testing.T) {
		// Nothing is stubbed on the publication store: nothing may be written.
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategy(7, "二十根均線"), nil)

		err := fixture.marketplaceApplication.PublishStrategy(t.Context(), marketplaceGuestID, 7)

		require.ErrorIs(t, err, domains.ErrStrategyNotFound)
	})

	t.Run("refuses a strategy that does not exist", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(entities.Strategy{}, domains.ErrStrategyNotFound)

		err := fixture.marketplaceApplication.PublishStrategy(t.Context(), strategyOwnerID, 7)

		require.ErrorIs(t, err, domains.ErrStrategyNotFound)
	})
}

func TestStrategyMarketplaceWithdrawStrategy(t *testing.T) {
	t.Run("takes the owner's strategy off the marketplace", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategy(7, "二十根均線"), nil)
		fixture.publishedStrategyRepository.EXPECT().Withdraw(gomock.Any(), uint(7)).Return(nil)

		require.NoError(t, fixture.marketplaceApplication.WithdrawStrategy(
			t.Context(), strategyOwnerID, 7))
	})

	t.Run("refuses somebody else's strategy as one that is not there", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategy(7, "二十根均線"), nil)

		err := fixture.marketplaceApplication.WithdrawStrategy(t.Context(), marketplaceGuestID, 7)

		require.ErrorIs(t, err, domains.ErrStrategyNotFound)
	})
}

func TestStrategyMarketplaceBrowseMarketplace(t *testing.T) {
	t.Run("hands back what is out there, without any algorithm", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyRepository.EXPECT().FindAllPublished(gomock.Any()).
			Return([]entities.PublishedStrategy{aPublication(2, "別人的", 8)}, nil)

		publishedStrategyDtos, err := fixture.marketplaceApplication.BrowseMarketplace(t.Context())

		require.NoError(t, err)
		require.Len(t, publishedStrategyDtos, 1)
		assert.Equal(t, "別人的", publishedStrategyDtos[0].Name)
		assert.Equal(t, "someone@example.com", publishedStrategyDtos[0].PublisherEmail)
		assert.False(t, publishedStrategyDtos[0].PublishedAt.IsZero())
	})

	t.Run("an empty shelf is an answer, not a failure", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyRepository.EXPECT().FindAllPublished(gomock.Any()).
			Return([]entities.PublishedStrategy{}, nil)

		publishedStrategyDtos, err := fixture.marketplaceApplication.BrowseMarketplace(t.Context())

		require.NoError(t, err)
		assert.NotNil(t, publishedStrategyDtos)
		assert.Empty(t, publishedStrategyDtos)
	})

	t.Run("reports a storage failure", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		storageFailure := errors.New("connection refused")
		fixture.strategyRepository.EXPECT().FindAllPublished(gomock.Any()).Return(nil, storageFailure)

		_, err := fixture.marketplaceApplication.BrowseMarketplace(t.Context())

		require.ErrorIs(t, err, storageFailure)
	})
}

func TestStrategyMarketplaceAdoptStrategy(t *testing.T) {
	t.Run("puts somebody else's strategy on this person's shelf", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategy(7, "二十根均線"), nil)
		fixture.strategyAdoptionRepository.EXPECT().
			Adopt(gomock.Any(), marketplaceGuestID, uint(7), marketplaceNow).Return(nil)

		require.NoError(t, fixture.marketplaceApplication.AdoptStrategy(
			t.Context(), marketplaceGuestID, 7))
	})

	t.Run("adopting one's own does nothing and is not a failure", func(t *testing.T) {
		// Nothing is stubbed on the adoption store: their own strategy is already on
		// their shelf, so there is nothing here to write.
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategy(7, "二十根均線"), nil)

		require.NoError(t, fixture.marketplaceApplication.AdoptStrategy(
			t.Context(), strategyOwnerID, 7))
	})

	t.Run("refuses a strategy that does not exist", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(entities.Strategy{}, domains.ErrStrategyNotFound)

		err := fixture.marketplaceApplication.AdoptStrategy(t.Context(), marketplaceGuestID, 7)

		require.ErrorIs(t, err, domains.ErrStrategyNotFound)
	})

	t.Run("reports one that is not on the marketplace as one that is not there", func(t *testing.T) {
		// The store is what finds out — there is no publication for the shelf entry
		// to hang from — and it says so in the same words a missing strategy gets.
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategy(7, "二十根均線"), nil)
		fixture.strategyAdoptionRepository.EXPECT().
			Adopt(gomock.Any(), marketplaceGuestID, uint(7), marketplaceNow).
			Return(domains.StrategyNotFound(7))

		err := fixture.marketplaceApplication.AdoptStrategy(t.Context(), marketplaceGuestID, 7)

		require.ErrorIs(t, err, domains.ErrStrategyNotFound)
	})
}

func TestStrategyMarketplaceAbandonStrategy(t *testing.T) {
	t.Run("takes it off this person's shelf without asking anybody", func(t *testing.T) {
		// The strategy store is not touched at all: a shelf entry belongs to the
		// person whose shelf it is, so removing one needs no permission — and a
		// strategy since deleted outright would make a lookup fail on a request
		// that is only tidying up.
		fixture := newMarketplaceUnderTest(t)
		fixture.strategyAdoptionRepository.EXPECT().
			Abandon(gomock.Any(), marketplaceGuestID, uint(7)).Return(nil)

		require.NoError(t, fixture.marketplaceApplication.AbandonStrategy(
			t.Context(), marketplaceGuestID, 7))
	})

	t.Run("reports a storage failure", func(t *testing.T) {
		fixture := newMarketplaceUnderTest(t)
		storageFailure := errors.New("connection refused")
		fixture.strategyAdoptionRepository.EXPECT().
			Abandon(gomock.Any(), marketplaceGuestID, uint(7)).Return(storageFailure)

		err := fixture.marketplaceApplication.AbandonStrategy(t.Context(), marketplaceGuestID, 7)

		require.ErrorIs(t, err, storageFailure)
	})
}
