package controller_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// expectTheContractBotCanBeSaved answers the contract trading strategy and the followed BTCUSDT a contract bot save reads.
func (fixture strategyBotRouterUnderTest) expectTheContractBotCanBeSaved() {
	fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(entities.TradingStrategy{
			ID: 9, OwnerID: signedInViewerID, Name: "合約黃金交叉",
			MarketDataKind: string(vo.MarketDataKindContractKCandle),
		}, nil).AnyTimes()
	fixture.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}, true, nil).AnyTimes()
	fixture.contractMaintenanceMarginTierRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(nil, nil).AnyTimes()
}

func TestStrategyBotRouterCarriesTheDirectionAndNotificationsBothWays(t *testing.T) {
	fixture := newStrategyBotRouterUnderTest(t)
	fixture.expectTheContractBotCanBeSaved()
	fixture.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) (entities.StrategyBot, error) {
			return bot, nil
		})
	body := strings.Replace(aContractStrategyBotBody, `"triggerIntervalMinutes": 5,`,
		`"triggerIntervalMinutes": 5, "tradingDirections": ["short"], "notifications": {"buy": false, "hold": true},`, 1)

	response := fixture.send(http.MethodPost, "/strategy-bots", body)

	require.Equal(t, http.StatusCreated, response.Code, response.Body.String())
	answer := struct {
		TradingDirections []string        `json:"tradingDirections"`
		Notifications     map[string]bool `json:"notifications"`
	}{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
	assert.Equal(t, []string{"short"}, answer.TradingDirections)
	assert.Equal(t, map[string]bool{"buy": false, "sell": true, "hold": true}, answer.Notifications)
}

func TestStrategyBotRouterAnswersASpotBotWithoutADirection(t *testing.T) {
	fixture := newStrategyBotRouterUnderTest(t)
	fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).
		Return(aStoredStrategyBotRow(vo.StrategyBotStopped), nil)

	response := fixture.send(http.MethodGet, "/strategy-bots/3", "")

	require.Equal(t, http.StatusOK, response.Code)
	answer := map[string]any{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
	assert.NotContains(t, answer, "tradingDirections")
	assert.Equal(t, map[string]any{"buy": true, "sell": true, "hold": false}, answer["notifications"])
}

func TestStrategyBotRouterNamesWhyABotCannotTurnAround(t *testing.T) {
	fixture := newStrategyBotRouterUnderTest(t)
	fixture.expectTheContractBotCanBeSaved()
	stored := aStoredContractStrategyBotRow()
	stored.ContractTradingMode = string(vo.ContractTradingModeLongOnly)
	stored.AutoOrderEnabled = true
	fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).Return(stored, nil).AnyTimes()
	fixture.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)
	body := strings.Replace(aContractStrategyBotBody, `"triggerIntervalMinutes": 5,`,
		`"triggerIntervalMinutes": 5, "tradingDirections": ["short"],`, 1)

	response := fixture.send(http.MethodPut, "/strategy-bots/3", body)

	require.Equal(t, http.StatusConflict, response.Code, response.Body.String())
	answer := map[string]any{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
	assert.Equal(t, "tradingDirectionAutoOrderOn", answer["reason"])
}

func TestStrategyBotRouterRefusesAnEmptyDirectionAsABadRequest(t *testing.T) {
	fixture := newStrategyBotRouterUnderTest(t)
	fixture.expectTheContractBotCanBeSaved()
	fixture.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)
	body := strings.Replace(aContractStrategyBotBody, `"triggerIntervalMinutes": 5,`,
		`"triggerIntervalMinutes": 5, "tradingDirections": [],`, 1)

	response := fixture.send(http.MethodPost, "/strategy-bots", body)

	require.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "至少要選一個交易方向")
}
