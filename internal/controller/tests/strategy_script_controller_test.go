package controller_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/controller"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type strategyScriptRouterUnderTest struct {
	engine                   *gin.Engine
	strategyScriptRepository *mocks.MockIStrategyScriptRepository
	// botsUsingScript is what every bot lookup answers; empty unless a test says otherwise.
	botsUsingScript *[]entities.StrategyBot
}

func newStrategyScriptRouterUnderTest(t *testing.T) strategyScriptRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(mockController)
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(mockController)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	botsUsingScript := &[]entities.StrategyBot{}
	strategyScriptController := controller.NewStrategyScriptController(
		application.NewStrategyScriptApplication(
			service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository),
			service.NewStrategyBotService(
				botsAnswering(mockController, botsUsingScript),
				mocks.NewMockIStrategyBotRunRecordRepository(mockController),
				mocks.NewMockIContractTradingSymbolRepository(mockController),
				mocks.NewMockIContractMaintenanceMarginTierRepository(mockController),
				mocks.NewMockIContractFundingRateSettlementRepository(mockController),
				mocks.NewMockIClockProxy(mockController))))

	engine := gin.New()
	requiresSignIn := doorOpenFor(t, signedInViewerID)
	engine.POST("/strategy-scripts", requiresSignIn, strategyScriptController.CreateStrategyScript)
	engine.GET("/strategy-scripts", requiresSignIn, strategyScriptController.ListAvailableStrategyScripts)
	engine.GET("/strategy-scripts/:id", requiresSignIn, strategyScriptController.GetStrategyScript)
	engine.PUT("/strategy-scripts/:id", requiresSignIn, strategyScriptController.UpdateStrategyScript)
	engine.DELETE("/strategy-scripts/:id", requiresSignIn, strategyScriptController.DeleteStrategyScript)

	return strategyScriptRouterUnderTest{
		engine: engine, strategyScriptRepository: strategyScriptRepository, botsUsingScript: botsUsingScript,
	}
}

func (fixture strategyScriptRouterUnderTest) send(
	method string, target string, body string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", signedInProof)
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	return recorder
}

const aStrategyScriptBody = `{
	"name": "二十根均線",
	"script": "func Calculate(candles []vo.KCandleVo) map[string][]float64 { return nil }",
	"resultType": "floatList"
}`

func aStoredStrategyScriptRow(id uint, name string) entities.StrategyScript {
	return entities.StrategyScript{
		ID:         id,
		OwnerID:    signedInViewerID,
		Name:       name,
		Script:     "func Calculate(candles []vo.KCandleVo) map[string][]float64 { return nil }",
		ResultType: "floatList",
		CreatedAt:  time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC),
	}
}

// aPublishedStrategyScriptRow is a marketplace script owned by someone other than the viewer.
func aPublishedStrategyScriptRow(id uint, name string) entities.PublishedStrategyScript {
	strategyScript := aStoredStrategyScriptRow(id, name)
	strategyScript.OwnerID = signedInViewerID + 1
	strategyScript.Owner = entities.User{ID: strategyScript.OwnerID, Email: "someone@example.com"}

	return entities.PublishedStrategyScript{
		StrategyScriptID: id,
		PublishedAt:      time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC),
		StrategyScript:   strategyScript,
	}
}

func TestStrategyScriptRouterCreateStrategyScript(t *testing.T) {
	t.Run("answers created and hands back the strategy script", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
				// Every body field must arrive, and a new script has no identifier.
				assert.Equal(t, uint(0), strategyScript.ID)
				assert.Equal(t, "二十根均線", strategyScript.Name)
				assert.Equal(t, aStoredStrategyScriptRow(0, "").Script, strategyScript.Script)
				assert.Equal(t, "floatList", strategyScript.ResultType)

				return aStoredStrategyScriptRow(7, strategyScript.Name), nil
			})

		response := fixture.send(http.MethodPost, "/strategy-scripts", aStrategyScriptBody)

		require.Equal(t, http.StatusCreated, response.Code)
		strategyScriptDto := dto.StrategyScriptDto{}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &strategyScriptDto))
		assert.Equal(t, uint(7), strategyScriptDto.ID)
		assert.Equal(t, "二十根均線", strategyScriptDto.Name)
	})

	t.Run("a plan for feeding the algorithm is not part of a strategy script", func(t *testing.T) {
		// Legacy interval/count fields are not refused; they bind to nothing and are not echoed.
		fixture := newStrategyScriptRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
				assert.Equal(t, "二十根均線", strategyScript.Name)
				assert.Equal(t, "floatList", strategyScript.ResultType)

				return aStoredStrategyScriptRow(7, strategyScript.Name), nil
			})

		response := fixture.send(http.MethodPost, "/strategy-scripts",
			`{"name": "二十根均線", "script": "package main", "resultType": "floatList",`+
				` "aggregationInterval": "1h", "candleCount": 45}`)

		require.Equal(t, http.StatusCreated, response.Code)
		assert.NotContains(t, response.Body.String(), "aggregationInterval")
		assert.NotContains(t, response.Body.String(), "candleCount")
	})

	t.Run("answers bad request when the body cannot be read", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)

		response := fixture.send(http.MethodPost, "/strategy-scripts", "{ not json")

		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("says the body could not be read rather than blaming its content", func(t *testing.T) {
		// Only the name is mistyped as a number, so the caller must be told the body is unreadable, not that a name is missing.
		fixture := newStrategyScriptRouterUnderTest(t)

		response := fixture.send(http.MethodPost, "/strategy-scripts",
			`{"name": 20, "script": "x", "resultType": "float"}`)

		require.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "name")
		assert.NotContains(t, response.Body.String(), "必須給策略腳本取一個名稱")
	})

	t.Run("answers bad request when the content breaks a rule", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)

		response := fixture.send(http.MethodPost, "/strategy-scripts",
			`{"name": "無此種類", "script": "x", "resultType": "string"}`)

		require.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "指標值種類只能是")
	})

	t.Run("answers conflict when the name is already held", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).Return(entities.StrategyScript{}, domains.ErrStrategyScriptNameConflict)

		response := fixture.send(http.MethodPost, "/strategy-scripts", aStrategyScriptBody)

		assert.Equal(t, http.StatusConflict, response.Code)
	})

	t.Run("answers bad gateway when storage will not answer", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).Return(entities.StrategyScript{}, errors.New("connection refused"))

		response := fixture.send(http.MethodPost, "/strategy-scripts", aStrategyScriptBody)

		assert.Equal(t, http.StatusBadGateway, response.Code)
	})
}

func TestStrategyScriptRouterSavesTheStrategyScriptForWhoeverCameThroughTheDoor(t *testing.T) {
	// The owner comes from the proof, never the body; otherwise a middleware treating everyone as one person would pass every other test.
	fixture := newStrategyScriptRouterUnderTest(t)
	storedOwnerID := uint(0)
	fixture.strategyScriptRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
			storedOwnerID = strategyScript.OwnerID

			return aStoredStrategyScriptRow(7, strategyScript.Name), nil
		})

	response := fixture.send(http.MethodPost, "/strategy-scripts", aStrategyScriptBody)

	require.Equal(t, http.StatusCreated, response.Code)
	assert.Equal(t, signedInViewerID, storedOwnerID)
}

func TestStrategyScriptRouterSavesNothingForARequestCarryingNoProof(t *testing.T) {
	// Nothing is stubbed on the repository: nothing may reach storage.
	fixture := newStrategyScriptRouterUnderTest(t)

	request := httptest.NewRequest(http.MethodPost, "/strategy-scripts", strings.NewReader(aStrategyScriptBody))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	assert.Equal(t, http.StatusUnauthorized, recorder.Code)
}

func TestStrategyScriptRouterListAvailableStrategyScripts(t *testing.T) {
	t.Run("answers with the caller's own strategy scripts and the ones they adopted", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindAllOwnedBy(gomock.Any(), signedInViewerID).Return([]entities.StrategyScript{
			aStoredStrategyScriptRow(1, "二十根均線"),
			aStoredStrategyScriptRow(2, "六十根均線"),
		}, nil)
		fixture.strategyScriptRepository.EXPECT().
			FindAllAdoptedBy(gomock.Any(), signedInViewerID).
			Return([]entities.PublishedStrategyScript{aPublishedStrategyScriptRow(3, "別人的")}, nil)

		response := fixture.send(http.MethodGet, "/strategy-scripts", "")

		require.Equal(t, http.StatusOK, response.Code)
		availableStrategyScriptsDto := dto.AvailableStrategyScriptsDto{}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &availableStrategyScriptsDto))
		require.Len(t, availableStrategyScriptsDto.Mine, 2)
		require.Len(t, availableStrategyScriptsDto.Adopted, 1)
		assert.Equal(t, "二十根均線", availableStrategyScriptsDto.Mine[0].Name)
		assert.Equal(t, "別人的", availableStrategyScriptsDto.Adopted[0].Name)
	})

	t.Run("never puts an adopted strategy script's algorithm on the wire", func(t *testing.T) {
		// The adopted shape has no script field, so the handler cannot leak it.
		fixture := newStrategyScriptRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindAllOwnedBy(gomock.Any(), signedInViewerID).Return([]entities.StrategyScript{}, nil)
		fixture.strategyScriptRepository.EXPECT().
			FindAllAdoptedBy(gomock.Any(), signedInViewerID).
			Return([]entities.PublishedStrategyScript{aPublishedStrategyScriptRow(3, "別人的")}, nil)

		response := fixture.send(http.MethodGet, "/strategy-scripts", "")

		require.Equal(t, http.StatusOK, response.Code)
		assert.NotContains(t, response.Body.String(), "func Calculate")
	})

	t.Run("answers with empty collections rather than nothing at all", func(t *testing.T) {
		// Holding none still answers [] rather than null, so readers need no null guard.
		fixture := newStrategyScriptRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindAllOwnedBy(gomock.Any(), signedInViewerID).Return([]entities.StrategyScript{}, nil)
		fixture.strategyScriptRepository.EXPECT().
			FindAllAdoptedBy(gomock.Any(), signedInViewerID).Return([]entities.PublishedStrategyScript{}, nil)

		response := fixture.send(http.MethodGet, "/strategy-scripts", "")

		require.Equal(t, http.StatusOK, response.Code)
		assert.JSONEq(t, `{"mine":[],"adopted":[]}`, response.Body.String())
	})

	t.Run("answers bad gateway when storage will not answer", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindAllOwnedBy(gomock.Any(), signedInViewerID).Return(nil, errors.New("connection refused"))

		response := fixture.send(http.MethodGet, "/strategy-scripts", "")

		assert.Equal(t, http.StatusBadGateway, response.Code)
	})

	t.Run("turns away a request carrying no proof of identity", func(t *testing.T) {
		// Nothing is stubbed on the repository: nothing may reach storage.
		fixture := newStrategyScriptRouterUnderTest(t)

		request := httptest.NewRequest(http.MethodGet, "/strategy-scripts", nil)
		recorder := httptest.NewRecorder()
		fixture.engine.ServeHTTP(recorder, request)

		assert.Equal(t, http.StatusUnauthorized, recorder.Code)
	})
}

func TestStrategyScriptRouterGetStrategyScript(t *testing.T) {
	t.Run("answers with the named strategy script", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScriptRow(7, "二十根均線"), nil)

		response := fixture.send(http.MethodGet, "/strategy-scripts/7", "")

		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "二十根均線")
	})

	t.Run("answers not found when no strategy script carries that identifier", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(entities.StrategyScript{}, domains.ErrStrategyScriptNotFound)

		response := fixture.send(http.MethodGet, "/strategy-scripts/7", "")

		assert.Equal(t, http.StatusNotFound, response.Code)
	})
}

func TestStrategyScriptRouterRefusesAnIdentifierThatIsNotOne(t *testing.T) {
	// Nothing is stubbed on the repository; the last two exceed an identifier's range and must answer "not found" rather than wrap onto a real script or reach storage.
	for _, id := range []string{
		"abc", "0", "-1", "1.5", "%20", "18446744073709551616", "9223372036854775808",
	} {
		for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
			t.Run(method+" /strategy-scripts/"+id, func(t *testing.T) {
				fixture := newStrategyScriptRouterUnderTest(t)

				response := fixture.send(method, "/strategy-scripts/"+id, aStrategyScriptBody)

				require.Equal(t, http.StatusBadRequest, response.Code)
				assert.Contains(t, response.Body.String(), "策略腳本識別碼必須是正整數")
			})
		}
	}
}

func TestStrategyScriptRouterUpdateStrategyScript(t *testing.T) {
	t.Run("answers with the strategy script as it now stands", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScriptRow(7, "二十根均線"), nil).AnyTimes()
		fixture.strategyScriptRepository.EXPECT().
			Update(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
				// Which strategy script is meant comes from the path, never from the body.
				assert.Equal(t, uint(7), strategyScript.ID)

				return aStoredStrategyScriptRow(7, strategyScript.Name), nil
			})

		response := fixture.send(http.MethodPut, "/strategy-scripts/7", aStrategyScriptBody)

		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "二十根均線")
	})

	t.Run("answers bad request when the body cannot be read", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)

		response := fixture.send(http.MethodPut, "/strategy-scripts/7", "{ not json")

		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("says the body could not be read rather than blaming its content", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)

		response := fixture.send(http.MethodPut, "/strategy-scripts/7",
			`{"name": 20, "script": "x", "resultType": "float"}`)

		require.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "name")
		assert.NotContains(t, response.Body.String(), "必須給策略腳本取一個名稱")
	})

	t.Run("answers bad request when the content breaks a rule", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScriptRow(7, "二十根均線"), nil).AnyTimes()

		response := fixture.send(http.MethodPut, "/strategy-scripts/7",
			`{"name": "", "script": "x", "resultType": "float"}`)

		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("answers not found when no strategy script carries that identifier", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(entities.StrategyScript{}, domains.ErrStrategyScriptNotFound)

		response := fixture.send(http.MethodPut, "/strategy-scripts/7", aStrategyScriptBody)

		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("answers conflict when the new name is already held", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScriptRow(7, "二十根均線"), nil).AnyTimes()
		fixture.strategyScriptRepository.EXPECT().
			Update(gomock.Any(), gomock.Any()).Return(entities.StrategyScript{}, domains.ErrStrategyScriptNameConflict)

		response := fixture.send(http.MethodPut, "/strategy-scripts/7", aStrategyScriptBody)

		assert.Equal(t, http.StatusConflict, response.Code)
	})
}

func TestStrategyScriptRouterRefusesARewriteWhileTheOwnersRunningBotUsesIt(t *testing.T) {
	// Update is unstubbed, so any write fails the test.
	fixture := newStrategyScriptRouterUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().
		FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScriptRow(7, "二十根均線"), nil).AnyTimes()
	*fixture.botsUsingScript = []entities.StrategyBot{
		{OwnerID: signedInViewerID, Name: "早盤突破", RunState: string(vo.StrategyBotRunning)},
	}

	response := fixture.send(http.MethodPut, "/strategy-scripts/7", aStrategyScriptBody)

	assert.Equal(t, http.StatusConflict, response.Code)
	assert.Contains(t, response.Body.String(), "這幾台機器人正在用它跑：早盤突破，請先停止它們")
}

func TestStrategyScriptRouterDeleteStrategyScript(t *testing.T) {
	t.Run("answers no content and says nothing more", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScriptRow(7, "二十根均線"), nil)
		fixture.strategyScriptRepository.EXPECT().Delete(gomock.Any(), uint(7)).Return(nil)

		response := fixture.send(http.MethodDelete, "/strategy-scripts/7", "")

		require.Equal(t, http.StatusNoContent, response.Code)
		assert.Empty(t, response.Body.String())
	})

	t.Run("answers not found when no strategy script carries that identifier", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(entities.StrategyScript{}, domains.ErrStrategyScriptNotFound)

		response := fixture.send(http.MethodDelete, "/strategy-scripts/7", "")

		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("answers bad gateway when storage will not answer", func(t *testing.T) {
		fixture := newStrategyScriptRouterUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScriptRow(7, "二十根均線"), nil)
		fixture.strategyScriptRepository.EXPECT().
			Delete(gomock.Any(), uint(7)).Return(errors.New("connection refused"))

		response := fixture.send(http.MethodDelete, "/strategy-scripts/7", "")

		assert.Equal(t, http.StatusBadGateway, response.Code)
	})
}

// A NUL byte is valid JSON but the database rejects it as a broken encoding, which would surface as 502 for the caller's mistake.
func TestStrategyScriptRouterRefusesTextThatCannotBeStored(t *testing.T) {
	testCases := []struct {
		name string
		body string
	}{
		{
			name: "a name carrying a NUL",
			body: `{"name":"a\u0000b","script":"package main","resultType":"float"}`,
		},
		{
			name: "a script carrying a NUL",
			body: `{"name":"二十根均線","script":"package \u0000main","resultType":"float"}`,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// No expectation is set on the repository: nothing may reach storage.
			fixture := newStrategyScriptRouterUnderTest(t)

			recorder := fixture.send(http.MethodPost, "/strategy-scripts", testCase.body)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
			assert.Contains(t, recorder.Body.String(), "NUL")
		})
	}
}

func TestStrategyScriptRouterCarriesTheKindOfMarketTheScriptEats(t *testing.T) {
	fixture := newStrategyScriptRouterUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
			assert.Equal(t, "contractKCandle", strategyScript.MarketDataKind)

			stored := aStoredStrategyScriptRow(7, strategyScript.Name)
			stored.MarketDataKind = strategyScript.MarketDataKind

			return stored, nil
		})

	recorder := fixture.send(http.MethodPost, "/strategy-scripts", `{
		"name": "費率反轉",
		"script": "func Calculate(data []indicator.ContractKCandle) map[string]float64 { return nil }",
		"resultType": "float",
		"marketDataKind": "contractKCandle"
	}`)

	require.Equal(t, http.StatusCreated, recorder.Code)
	responseBody := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &responseBody))
	assert.Equal(t, "contractKCandle", responseBody["marketDataKind"])
}

// botsAnswering answers every bot lookup with whatever botsUsingScript holds at the time of the call.
func botsAnswering(
	mockController *gomock.Controller, botsUsingScript *[]entities.StrategyBot,
) *mocks.MockIStrategyBotRepository {
	strategyBotRepository := mocks.NewMockIStrategyBotRepository(mockController)
	strategyBotRepository.EXPECT().FindAllByStrategyScript(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, uint) ([]entities.StrategyBot, error) {
			return *botsUsingScript, nil
		}).AnyTimes()

	return strategyBotRepository
}
