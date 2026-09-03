package controller_test

import (
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

const strategyRouterMaxCandleCount = 1000

type strategyRouterUnderTest struct {
	engine             *gin.Engine
	strategyRepository *mocks.MockIStrategyRepository
}

func newStrategyRouterUnderTest(t *testing.T) strategyRouterUnderTest {
	gin.SetMode(gin.TestMode)
	mockController := gomock.NewController(t)
	strategyRepository := mocks.NewMockIStrategyRepository(mockController)

	strategyController := controller.NewStrategyController(
		application.NewStrategyApplication(
			service.NewStrategyService(strategyRepository, strategyRouterMaxCandleCount)))

	engine := gin.New()
	engine.POST("/strategies", strategyController.CreateStrategy)
	engine.GET("/strategies", strategyController.ListStrategies)
	engine.GET("/strategies/:id", strategyController.GetStrategy)
	engine.PUT("/strategies/:id", strategyController.UpdateStrategy)
	engine.DELETE("/strategies/:id", strategyController.DeleteStrategy)

	return strategyRouterUnderTest{engine: engine, strategyRepository: strategyRepository}
}

func (fixture strategyRouterUnderTest) send(
	method string, target string, body string,
) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, target, strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	recorder := httptest.NewRecorder()
	fixture.engine.ServeHTTP(recorder, request)

	return recorder
}

const aStrategyBody = `{
	"name": "二十根均線",
	"script": "func Calculate(candles []vo.KCandleVo) map[string][]float64 { return nil }",
	"resultType": "floatList",
	"aggregationInterval": "1h",
	"candleCount": 45
}`

func aStoredStrategyRow(id uint, name string) entities.Strategy {
	return entities.Strategy{
		ID:                  id,
		Name:                name,
		Script:              "func Calculate(candles []vo.KCandleVo) map[string][]float64 { return nil }",
		ResultType:          "floatList",
		AggregationInterval: "1h",
		CandleCount:         20,
		CreatedAt:           time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC),
		UpdatedAt:           time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC),
	}
}

func TestStrategyRouterCreateStrategy(t *testing.T) {
	t.Run("answers created and hands back the strategy", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)
		fixture.strategyRepository.EXPECT().
			Save(gomock.Any()).
			DoAndReturn(func(strategy entities.Strategy) (entities.Strategy, error) {
				// Every field the body carried has to arrive, and a strategy being
				// created carries no identifier of its own — the path had none to give.
				assert.Equal(t, uint(0), strategy.ID)
				assert.Equal(t, "二十根均線", strategy.Name)
				assert.Equal(t, aStoredStrategyRow(0, "").Script, strategy.Script)
				assert.Equal(t, "floatList", strategy.ResultType)
				assert.Equal(t, "1h", strategy.AggregationInterval)
				assert.Equal(t, 45, strategy.CandleCount)

				return aStoredStrategyRow(7, strategy.Name), nil
			})

		response := fixture.send(http.MethodPost, "/strategies", aStrategyBody)

		require.Equal(t, http.StatusCreated, response.Code)
		strategyDto := dto.StrategyDto{}
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &strategyDto))
		assert.Equal(t, uint(7), strategyDto.ID)
		assert.Equal(t, "二十根均線", strategyDto.Name)
		assert.Equal(t, "1h", strategyDto.AggregationInterval)
	})

	t.Run("answers bad request when the body cannot be read", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)

		response := fixture.send(http.MethodPost, "/strategies", "{ not json")

		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("says the body could not be read rather than blaming its content", func(t *testing.T) {
		// Every field is filled in and legal; only the candle count is written as
		// text. Reading the body fails, and the caller has to be told that — being
		// told "a strategy needs a name" would send them looking at the wrong field.
		fixture := newStrategyRouterUnderTest(t)

		response := fixture.send(http.MethodPost, "/strategies",
			`{"name": "二十根均線", "script": "x", "resultType": "float",`+
				` "aggregationInterval": "1h", "candleCount": "twenty"}`)

		require.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "candleCount")
		assert.NotContains(t, response.Body.String(), "必須給策略取一個名稱")
	})

	t.Run("answers bad request when the content breaks a rule", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)

		response := fixture.send(http.MethodPost, "/strategies",
			`{"name": "七分鐘", "script": "x", "aggregationInterval": "7m", "candleCount": 20}`)

		require.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "彙總刻度只能是")
	})

	t.Run("answers conflict when the name is already held", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)
		fixture.strategyRepository.EXPECT().
			Save(gomock.Any()).Return(entities.Strategy{}, domains.ErrStrategyNameConflict)

		response := fixture.send(http.MethodPost, "/strategies", aStrategyBody)

		assert.Equal(t, http.StatusConflict, response.Code)
	})

	t.Run("answers bad gateway when storage will not answer", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)
		fixture.strategyRepository.EXPECT().
			Save(gomock.Any()).Return(entities.Strategy{}, errors.New("connection refused"))

		response := fixture.send(http.MethodPost, "/strategies", aStrategyBody)

		assert.Equal(t, http.StatusBadGateway, response.Code)
	})
}

func TestStrategyRouterListStrategies(t *testing.T) {
	t.Run("answers with every strategy", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)
		fixture.strategyRepository.EXPECT().FindAll().Return([]entities.Strategy{
			aStoredStrategyRow(1, "二十根均線"),
			aStoredStrategyRow(2, "六十根均線"),
		}, nil)

		response := fixture.send(http.MethodGet, "/strategies", "")

		require.Equal(t, http.StatusOK, response.Code)
		strategyDtos := make([]dto.StrategyDto, 0)
		require.NoError(t, json.Unmarshal(response.Body.Bytes(), &strategyDtos))
		require.Len(t, strategyDtos, 2)
		assert.Equal(t, "二十根均線", strategyDtos[0].Name)
	})

	t.Run("answers with an empty collection rather than nothing at all", func(t *testing.T) {
		// A reader that gets null has to guard against it; one that gets [] can just
		// read it, which is why holding none still answers with a collection.
		fixture := newStrategyRouterUnderTest(t)
		fixture.strategyRepository.EXPECT().FindAll().Return([]entities.Strategy{}, nil)

		response := fixture.send(http.MethodGet, "/strategies", "")

		require.Equal(t, http.StatusOK, response.Code)
		assert.JSONEq(t, "[]", response.Body.String())
	})

	t.Run("answers bad gateway when storage will not answer", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)
		fixture.strategyRepository.EXPECT().FindAll().Return(nil, errors.New("connection refused"))

		response := fixture.send(http.MethodGet, "/strategies", "")

		assert.Equal(t, http.StatusBadGateway, response.Code)
	})
}

func TestStrategyRouterGetStrategy(t *testing.T) {
	t.Run("answers with the named strategy", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)
		fixture.strategyRepository.EXPECT().
			FindOne(uint(7)).Return(aStoredStrategyRow(7, "二十根均線"), nil)

		response := fixture.send(http.MethodGet, "/strategies/7", "")

		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "二十根均線")
	})

	t.Run("answers not found when no strategy carries that identifier", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)
		fixture.strategyRepository.EXPECT().
			FindOne(uint(7)).Return(entities.Strategy{}, domains.ErrStrategyNotFound)

		response := fixture.send(http.MethodGet, "/strategies/7", "")

		assert.Equal(t, http.StatusNotFound, response.Code)
	})
}

func TestStrategyRouterRefusesAnIdentifierThatIsNotOne(t *testing.T) {
	// Nothing is stubbed on the repository, so a request that got as far as storage
	// would fail the test rather than quietly answer.
	for _, id := range []string{"abc", "0", "-1", "1.5", "%20"} {
		for _, method := range []string{http.MethodGet, http.MethodPut, http.MethodDelete} {
			t.Run(method+" /strategies/"+id, func(t *testing.T) {
				fixture := newStrategyRouterUnderTest(t)

				response := fixture.send(method, "/strategies/"+id, aStrategyBody)

				require.Equal(t, http.StatusBadRequest, response.Code)
				assert.Contains(t, response.Body.String(), "策略識別碼必須是正整數")
			})
		}
	}
}

func TestStrategyRouterUpdateStrategy(t *testing.T) {
	t.Run("answers with the strategy as it now stands", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)
		fixture.strategyRepository.EXPECT().
			Update(gomock.Any()).
			DoAndReturn(func(strategy entities.Strategy) (entities.Strategy, error) {
				// Which strategy is meant comes from the path, never from the body.
				assert.Equal(t, uint(7), strategy.ID)

				return aStoredStrategyRow(7, strategy.Name), nil
			})

		response := fixture.send(http.MethodPut, "/strategies/7", aStrategyBody)

		require.Equal(t, http.StatusOK, response.Code)
		assert.Contains(t, response.Body.String(), "二十根均線")
	})

	t.Run("answers bad request when the body cannot be read", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)

		response := fixture.send(http.MethodPut, "/strategies/7", "{ not json")

		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("says the body could not be read rather than blaming its content", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)

		response := fixture.send(http.MethodPut, "/strategies/7",
			`{"name": "二十根均線", "script": "x", "resultType": "float",`+
				` "aggregationInterval": "1h", "candleCount": "twenty"}`)

		require.Equal(t, http.StatusBadRequest, response.Code)
		assert.Contains(t, response.Body.String(), "candleCount")
		assert.NotContains(t, response.Body.String(), "必須給策略取一個名稱")
	})

	t.Run("answers bad request when the content breaks a rule", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)

		response := fixture.send(http.MethodPut, "/strategies/7",
			`{"name": "", "script": "x", "aggregationInterval": "1h", "candleCount": 20}`)

		assert.Equal(t, http.StatusBadRequest, response.Code)
	})

	t.Run("answers not found when no strategy carries that identifier", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)
		fixture.strategyRepository.EXPECT().
			Update(gomock.Any()).Return(entities.Strategy{}, domains.ErrStrategyNotFound)

		response := fixture.send(http.MethodPut, "/strategies/7", aStrategyBody)

		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("answers conflict when the new name is already held", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)
		fixture.strategyRepository.EXPECT().
			Update(gomock.Any()).Return(entities.Strategy{}, domains.ErrStrategyNameConflict)

		response := fixture.send(http.MethodPut, "/strategies/7", aStrategyBody)

		assert.Equal(t, http.StatusConflict, response.Code)
	})
}

func TestStrategyRouterDeleteStrategy(t *testing.T) {
	t.Run("answers no content and says nothing more", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)
		fixture.strategyRepository.EXPECT().Delete(uint(7)).Return(nil)

		response := fixture.send(http.MethodDelete, "/strategies/7", "")

		require.Equal(t, http.StatusNoContent, response.Code)
		assert.Empty(t, response.Body.String())
	})

	t.Run("answers not found when no strategy carries that identifier", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)
		fixture.strategyRepository.EXPECT().
			Delete(uint(7)).Return(domains.ErrStrategyNotFound)

		response := fixture.send(http.MethodDelete, "/strategies/7", "")

		assert.Equal(t, http.StatusNotFound, response.Code)
	})

	t.Run("answers bad gateway when storage will not answer", func(t *testing.T) {
		fixture := newStrategyRouterUnderTest(t)
		fixture.strategyRepository.EXPECT().
			Delete(uint(7)).Return(errors.New("connection refused"))

		response := fixture.send(http.MethodDelete, "/strategies/7", "")

		assert.Equal(t, http.StatusBadGateway, response.Code)
	})
}
