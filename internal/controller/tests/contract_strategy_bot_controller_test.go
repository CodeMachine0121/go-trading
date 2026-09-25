package controller_test

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const aContractStrategyBotBody = `{
	"name": "合約突破",
	"symbol": "BTCUSDT",
	"marketDataKind": "contractKCandle",
	"tradingStrategyId": 9,
	"triggerIntervalMinutes": 5,
	"positionPlan": {
		"capital": "1000",
		"stopLossPercentage": "2",
		"leverage": "5"
	}
}`

func aStoredContractStrategyBotRow() entities.StrategyBot {
	bot := aStoredStrategyBotRow(vo.StrategyBotStopped)
	bot.Name = "合約突破"
	bot.MarketDataKind = string(vo.MarketDataKindContractKCandle)
	bot.PositionPlanCapital = decimal.NewFromInt(1000)
	bot.PositionPlanStopLossPercentage = decimal.NewFromInt(2)
	bot.PositionPlanLeverage = decimal.NewFromInt(5)

	return bot
}

func TestStrategyBotRouterCreatesAContractBotAndAnswersWithItsKind(t *testing.T) {
	fixture := newStrategyBotRouterUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(entities.TradingStrategy{
			ID: 9, OwnerID: signedInViewerID, Name: "合約黃金交叉",
			MarketDataKind: string(vo.MarketDataKindContractKCandle),
		}, nil)
	fixture.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}, true, nil)
	fixture.contractMaintenanceMarginTierRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(nil, nil)
	fixture.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) (entities.StrategyBot, error) {
			assert.Equal(t, string(vo.MarketDataKindContractKCandle), bot.MarketDataKind)
			assert.Equal(t, "5", bot.PositionPlanLeverage.String())

			return aStoredContractStrategyBotRow(), nil
		})

	response := fixture.send(http.MethodPost, "/strategy-bots", aContractStrategyBotBody)

	require.Equal(t, http.StatusCreated, response.Code)
	answer := struct {
		MarketDataKind string `json:"marketDataKind"`
		PositionPlan   map[string]string
	}{}
	require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
	assert.Equal(t, "contractKCandle", answer.MarketDataKind)
	// The plan is echoed back as sent, leverage included.
	assert.Equal(t, map[string]string{
		"capital": "1000", "sizingMode": "", "sizingValue": "0",
		"stopLossPercentage": "2", "takeProfitPercentage": "0", "leverage": "5",
	}, answer.PositionPlan)
}

func TestStrategyBotRouterRefusesAContractBotWatchingAnUnfollowedContract(t *testing.T) {
	fixture := newStrategyBotRouterUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(entities.TradingStrategy{
			ID: 9, OwnerID: signedInViewerID, MarketDataKind: string(vo.MarketDataKindContractKCandle),
		}, nil)
	fixture.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.ContractTradingSymbol{}, false, nil)
	fixture.contractMaintenanceMarginTierRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(nil, nil)

	response := fixture.send(http.MethodPost, "/strategy-bots", aContractStrategyBotBody)

	assert.Equal(t, http.StatusBadRequest, response.Code)
	assert.Contains(t, response.Body.String(), "請先把它加進合約追蹤名單")
}

func TestStrategyBotRouterListsOneKindOfBotWhenAsked(t *testing.T) {
	testCases := []struct {
		name          string
		target        string
		expectedCode  int
		expectedNames []string
	}{
		{name: "contract bots only", target: "/strategy-bots?marketDataKind=contractKCandle",
			expectedCode: http.StatusOK, expectedNames: []string{"合約突破"}},
		{name: "every bot", target: "/strategy-bots",
			expectedCode: http.StatusOK, expectedNames: []string{"早盤突破", "合約突破"}},
		{name: "a kind nobody recognises", target: "/strategy-bots?marketDataKind=futures",
			expectedCode: http.StatusBadRequest},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newStrategyBotRouterUnderTest(t)
			fixture.strategyBotRepository.EXPECT().FindAllByOwner(gomock.Any(), signedInViewerID).
				Return([]entities.StrategyBot{
					aStoredStrategyBotRow(vo.StrategyBotStopped), aStoredContractStrategyBotRow(),
				}, nil).AnyTimes()

			response := fixture.send(http.MethodGet, testCase.target, "")

			require.Equal(t, testCase.expectedCode, response.Code)
			if testCase.expectedCode != http.StatusOK {
				return
			}

			answer := []struct {
				Name string `json:"name"`
			}{}
			require.NoError(t, json.Unmarshal(response.Body.Bytes(), &answer))
			listedNames := make([]string, 0, len(answer))
			for _, bot := range answer {
				listedNames = append(listedNames, bot.Name)
			}
			assert.Equal(t, testCase.expectedNames, listedNames)
		})
	}
}

func TestStrategyBotRouterAnswersASpotBotsPlanWithoutLeverage(t *testing.T) {
	fixture := newStrategyBotRouterUnderTest(t)
	storedRow := aStoredStrategyBotRow(vo.StrategyBotStopped)
	storedRow.PositionPlanCapital = decimal.NewFromInt(50000)
	fixture.strategyBotRepository.EXPECT().FindOne(gomock.Any(), uint(3)).Return(storedRow, nil)

	response := fixture.send(http.MethodGet, "/strategy-bots/3", "")

	require.Equal(t, http.StatusOK, response.Code)
	assert.Contains(t, response.Body.String(), `"capital":"50000"`)
	assert.Contains(t, response.Body.String(), `"marketDataKind":"kCandle"`)
	assert.NotContains(t, response.Body.String(), "everage")
}
