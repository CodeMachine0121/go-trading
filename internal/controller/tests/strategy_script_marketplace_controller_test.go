package controller_test

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var marketplaceRouterNow = time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC)

type marketplaceRouterUnderTest struct {
	engine                            *gin.Engine
	strategyScriptRepository          *mocks.MockIStrategyScriptRepository
	publishedStrategyScriptRepository *mocks.MockIPublishedStrategyScriptRepository
	strategyScriptAdoptionRepository  *mocks.MockIStrategyScriptAdoptionRepository
}

func newMarketplaceRouterUnderTest(t *testing.T) marketplaceRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(mockController)
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(mockController)
	strategyScriptAdoptionRepository := mocks.NewMockIStrategyScriptAdoptionRepository(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(marketplaceRouterNow).AnyTimes()

	marketplaceController := controller.NewStrategyScriptMarketplaceController(
		application.NewStrategyScriptMarketplaceApplication(
			service.NewStrategyScriptMarketplaceService(
				strategyScriptRepository, publishedStrategyScriptRepository, strategyScriptAdoptionRepository, clockProxy)))

	engine := gin.New()
	requiresSignIn := doorOpenFor(t, signedInViewerID)
	engine.POST("/strategy-scripts/:id/publication", requiresSignIn, marketplaceController.PublishStrategyScript)
	engine.DELETE("/strategy-scripts/:id/publication", requiresSignIn, marketplaceController.WithdrawStrategyScript)
	engine.GET("/marketplace/strategy-scripts", requiresSignIn, marketplaceController.BrowseMarketplace)
	engine.POST("/marketplace/strategy-scripts/:id/adoption", requiresSignIn, marketplaceController.AdoptStrategyScript)
	engine.DELETE("/marketplace/strategy-scripts/:id/adoption", requiresSignIn, marketplaceController.AbandonStrategyScript)

	return marketplaceRouterUnderTest{
		engine:                            engine,
		strategyScriptRepository:          strategyScriptRepository,
		publishedStrategyScriptRepository: publishedStrategyScriptRepository,
		strategyScriptAdoptionRepository:  strategyScriptAdoptionRepository,
	}
}

func (fixture marketplaceRouterUnderTest) send(method string, target string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, nil)
	request.Header.Set("Authorization", signedInProof)
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	return recorder
}

// sendUnproven is the same request with no proof of identity on it.
func (fixture marketplaceRouterUnderTest) sendUnproven(method string, target string) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, nil)
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	return recorder
}

func TestMarketplaceRouterPublishStrategyScript(t *testing.T) {
	t.Run("answers no content and says nothing more", func(t *testing.T) {
		fixture := newMarketplaceRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScriptRow(7, "二十根均線"), nil)
		fixture.publishedStrategyScriptRepository.EXPECT().
			Publish(gomock.Any(), uint(7), marketplaceRouterNow).Return(nil)

		response := fixture.send(http.MethodPost, "/strategy-scripts/7/publication")

		assert.Equal(t, http.StatusNoContent, response.Code)
		assert.Empty(t, response.Body.String())
	})

	t.Run("answers not found for somebody else's strategy script", func(t *testing.T) {
		fixture := newMarketplaceRouterUnderTest(t)
		strangersStrategyScript := aStoredStrategyScriptRow(7, "別人的")
		strangersStrategyScript.OwnerID = signedInViewerID + 1
		fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(strangersStrategyScript, nil)

		response := fixture.send(http.MethodPost, "/strategy-scripts/7/publication")

		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("answers bad request for an identifier that is not one", func(t *testing.T) {
		fixture := newMarketplaceRouterUnderTest(t)

		response := fixture.send(http.MethodPost, "/strategy-scripts/0/publication")

		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("turns away a request carrying no proof of identity", func(t *testing.T) {
		fixture := newMarketplaceRouterUnderTest(t)

		response := fixture.sendUnproven(http.MethodPost, "/strategy-scripts/7/publication")

		assert.Equal(t, http.StatusUnauthorized, response.Code)
	})
}

func TestMarketplaceRouterWithdrawStrategyScript(t *testing.T) {
	t.Run("answers no content", func(t *testing.T) {
		fixture := newMarketplaceRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScriptRow(7, "二十根均線"), nil)
		fixture.publishedStrategyScriptRepository.EXPECT().Withdraw(gomock.Any(), uint(7)).Return(nil)

		response := fixture.send(http.MethodDelete, "/strategy-scripts/7/publication")

		assert.Equal(t, http.StatusNoContent, response.Code)
	})

	t.Run("answers not found for somebody else's strategy script", func(t *testing.T) {
		fixture := newMarketplaceRouterUnderTest(t)
		strangersStrategyScript := aStoredStrategyScriptRow(7, "別人的")
		strangersStrategyScript.OwnerID = signedInViewerID + 1
		fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(strangersStrategyScript, nil)

		response := fixture.send(http.MethodDelete, "/strategy-scripts/7/publication")

		assert.Equal(t, http.StatusNotFound, response.Code)
	})
}

func TestMarketplaceRouterBrowseMarketplace(t *testing.T) {
	t.Run("answers with what is out there and never an algorithm", func(t *testing.T) {
		fixture := newMarketplaceRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().FindAllPublished(gomock.Any()).
			Return([]entities.PublishedStrategyScript{aPublishedStrategyScriptRow(3, "別人的")}, nil)

		response := fixture.send(http.MethodGet, "/marketplace/strategy-scripts")

		require.Equal(t, http.StatusOK, response.Code)
		publishedStrategyScriptDtos := make([]dto.PublishedStrategyScriptDto, 0)
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &publishedStrategyScriptDtos))
		require.Len(t, publishedStrategyScriptDtos, 1)
		assert.Equal(t, "別人的", publishedStrategyScriptDtos[0].Name)
		assert.Equal(t, "someone@example.com", publishedStrategyScriptDtos[0].PublisherEmail)
		assert.NotContains(t, response.Body.String(), "func Calculate")
		assert.NotContains(t, response.Body.String(), `"script"`)
	})

	t.Run("answers with an empty collection rather than nothing at all", func(t *testing.T) {
		fixture := newMarketplaceRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().FindAllPublished(gomock.Any()).
			Return([]entities.PublishedStrategyScript{}, nil)

		response := fixture.send(http.MethodGet, "/marketplace/strategy-scripts")

		require.Equal(t, http.StatusOK, response.Code)
		assert.JSONEq(t, "[]", response.Body.String())
	})

	t.Run("reports a storage failure as this system's own", func(t *testing.T) {
		fixture := newMarketplaceRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().FindAllPublished(gomock.Any()).
			Return(nil, errors.New("connection refused"))

		response := fixture.send(http.MethodGet, "/marketplace/strategy-scripts")

		assert.Equal(t, http.StatusInternalServerError, response.Code)
	})
}

func TestMarketplaceRouterAdoption(t *testing.T) {
	t.Run("adopting answers no content", func(t *testing.T) {
		fixture := newMarketplaceRouterUnderTest(t)
		strangersStrategyScript := aStoredStrategyScriptRow(7, "別人的")
		strangersStrategyScript.OwnerID = signedInViewerID + 1
		fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(strangersStrategyScript, nil)
		fixture.strategyScriptAdoptionRepository.EXPECT().
			Adopt(gomock.Any(), signedInViewerID, uint(7), marketplaceRouterNow).Return(nil)

		response := fixture.send(http.MethodPost, "/marketplace/strategy-scripts/7/adoption")

		assert.Equal(t, http.StatusNoContent, response.Code)
	})

	t.Run("adopting something not on the marketplace answers not found", func(t *testing.T) {
		fixture := newMarketplaceRouterUnderTest(t)
		strangersStrategyScript := aStoredStrategyScriptRow(7, "別人的")
		strangersStrategyScript.OwnerID = signedInViewerID + 1
		fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(strangersStrategyScript, nil)
		fixture.strategyScriptAdoptionRepository.EXPECT().
			Adopt(gomock.Any(), signedInViewerID, uint(7), marketplaceRouterNow).
			Return(domains.StrategyScriptNotFound(7))

		response := fixture.send(http.MethodPost, "/marketplace/strategy-scripts/7/adoption")

		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("abandoning answers no content", func(t *testing.T) {
		fixture := newMarketplaceRouterUnderTest(t)
		fixture.strategyScriptAdoptionRepository.EXPECT().
			Abandon(gomock.Any(), signedInViewerID, uint(7)).Return(nil)

		response := fixture.send(http.MethodDelete, "/marketplace/strategy-scripts/7/adoption")

		assert.Equal(t, http.StatusNoContent, response.Code)
	})

	t.Run("turns away a request carrying no proof of identity", func(t *testing.T) {
		fixture := newMarketplaceRouterUnderTest(t)

		response := fixture.sendUnproven(http.MethodPost, "/marketplace/strategy-scripts/7/adoption")

		assert.Equal(t, http.StatusUnauthorized, response.Code)
	})
}

func TestMarketplaceRouterRefusesAnIdentifierThatIsNotOne(t *testing.T) {
	// Every path that names a strategy script reads it the same way, so every one of them
	// turns the same nonsense away before anything is read or written.
	testCases := []struct {
		name   string
		method string
		target string
	}{
		{name: "withdrawing", method: http.MethodDelete, target: "/strategy-scripts/nought/publication"},
		{name: "adopting", method: http.MethodPost, target: "/marketplace/strategy-scripts/0/adoption"},
		{name: "abandoning", method: http.MethodDelete, target: "/marketplace/strategy-scripts/nought/adoption"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Nothing is stubbed: nothing may reach storage.
			fixture := newMarketplaceRouterUnderTest(t)

			response := fixture.send(testCase.method, testCase.target)

			assert.Equal(t, http.StatusBadRequest, response.Code)
		})
	}
}

func TestMarketplaceRouterReportsAFailureToTidyAShelf(t *testing.T) {
	fixture := newMarketplaceRouterUnderTest(t)
	fixture.strategyScriptAdoptionRepository.EXPECT().
		Abandon(gomock.Any(), signedInViewerID, uint(7)).Return(errors.New("connection refused"))

	response := fixture.send(http.MethodDelete, "/marketplace/strategy-scripts/7/adoption")

	assert.Equal(t, http.StatusInternalServerError, response.Code)
}
