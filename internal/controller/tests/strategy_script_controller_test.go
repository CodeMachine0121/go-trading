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
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type strategyScriptRouterUnderTest struct {
	engine                   *gin.Engine
	strategyScriptRepository *mocks.MockIStrategyScriptRepository
}

func newStrategyScriptRouterUnderTest(t *testing.T) strategyScriptRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(mockController)
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(mockController)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	strategyScriptController := controller.NewStrategyScriptController(
		application.NewStrategyScriptApplication(
			service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository)))

	engine := gin.New()
	requiresSignIn := doorOpenFor(t, signedInViewerID)
	engine.POST("/strategy-scripts", requiresSignIn, strategyScriptController.CreateStrategyScript)
	engine.GET("/strategy-scripts", requiresSignIn, strategyScriptController.ListAvailableStrategyScripts)
	engine.GET("/strategy-scripts/:id", requiresSignIn, strategyScriptController.GetStrategyScript)
	engine.PUT("/strategy-scripts/:id", requiresSignIn, strategyScriptController.UpdateStrategyScript)
	engine.DELETE("/strategy-scripts/:id", requiresSignIn, strategyScriptController.DeleteStrategyScript)

	return strategyScriptRouterUnderTest{engine: engine, strategyScriptRepository: strategyScriptRepository}
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

// aPublishedStrategyScriptRow is one strategy script on the marketplace, owned by somebody who
// is not the signed-in viewer.
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
				// Every field the body carried has to arrive, and a strategy script being
				// created carries no identifier of its own — the path had none to give.
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
		// A caller that still sends how coarse the candles are and how many of them
		// is not refused — those fields simply bind to nothing, and nothing comes
		// back carrying them either.
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
		// Every field is present; only the name is written as a number. Reading the
		// body fails, and the caller has to be told that — being told "a strategy script
		// needs a name" would send them looking for a missing field rather than a
		// mistyped one.
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
	// Who a strategy script belongs to comes from the proof on the request, never from
	// what the request says about itself. Without this, a door that recognised
	// everybody as the same person would still pass every other test in this file.
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
		// The adopted half is a shape with no script field at all, so this is not a
		// promise the handler keeps — it is one it cannot break.
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
		// A reader that gets null has to guard against it; one that gets [] can just
		// read it, which is why holding none still answers with a collection.
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
	// Nothing is stubbed on the repository, so a request that got as far as storage
	// would fail the test rather than quietly answer.
	// The last one is larger than an identifier can hold. Read too wide and then
	// narrowed, it would wrap onto a real strategy script and answer for that one instead.
	// The last two are larger than an identifier can hold: one overflows the parse,
	// the other parses cleanly and would otherwise reach the database and come back
	// as a storage failure rather than as "no strategy script has that identifier".
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
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScriptRow(7, "二十根均線"), nil)
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
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScriptRow(7, "二十根均線"), nil)

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
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScriptRow(7, "二十根均線"), nil)
		fixture.strategyScriptRepository.EXPECT().
			Update(gomock.Any(), gomock.Any()).Return(entities.StrategyScript{}, domains.ErrStrategyScriptNameConflict)

		response := fixture.send(http.MethodPut, "/strategy-scripts/7", aStrategyScriptBody)

		assert.Equal(t, http.StatusConflict, response.Code)
	})
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

// A NUL byte is legal JSON and decodes into a real character, so it reaches the
// rules like any other text. Left to the database it comes back as a broken
// encoding, which the error mapping cannot recognise and reports as 502 — telling
// the caller the system failed when what failed was what they sent.
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
