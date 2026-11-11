package controller_test

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// aStoredScriptOfTheViewer is the signed-in viewer's script with the given usage.
func aStoredScriptOfTheViewer(usage string) entities.StrategyScript {
	return entities.StrategyScript{
		ID: 4, OwnerID: signedInViewerID, Name: "均線", Script: "package main", ResultType: "floatList", Usage: usage,
	}
}

func TestStrategyScriptRouterSwitchesUsage(t *testing.T) {
	testCases := []struct {
		name                 string
		tradingStrategyNames []string
		requestBody          string
		expectedStatus       int
		expectedReason       string
		expectedUsage        string
	}{
		{name: "switched when nothing computes with it", tradingStrategyNames: []string{},
			requestBody: `{"usage":"chart"}`, expectedStatus: http.StatusOK, expectedUsage: "chart"},
		{name: "refused with a reason while a trading strategy computes with it",
			tradingStrategyNames: []string{"4H 趨勢"},
			requestBody:          `{"usage":"chart"}`, expectedStatus: http.StatusConflict,
			expectedReason: "strategyScriptUsageInUse"},
		{name: "a usage outside the two is a bad request", tradingStrategyNames: []string{},
			requestBody: `{"usage":"backtest"}`, expectedStatus: http.StatusBadRequest},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newStrategyScriptRouterUnderTest(t)
			fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(4)).
				Return(aStoredScriptOfTheViewer("calculation"), nil)
			fixture.tradingStrategyRepository.EXPECT().FindNamesByStrategyScript(gomock.Any(), uint(4)).
				Return(testCase.tradingStrategyNames, nil)
			fixture.screeningProfileRepository.EXPECT().FindNamesByStrategyScript(gomock.Any(), uint(4)).
				Return([]string{}, nil)
			fixture.strategyScriptRepository.EXPECT().UpdateUsage(gomock.Any(), uint(4), "chart").
				Return(aStoredScriptOfTheViewer("chart"), nil).MaxTimes(1)

			recorder := fixture.send(http.MethodPut, "/strategy-scripts/4/usage", testCase.requestBody)

			require.Equal(t, testCase.expectedStatus, recorder.Code, recorder.Body.String())
			body := map[string]any{}
			require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
			if testCase.expectedReason != "" {
				assert.Equal(t, testCase.expectedReason, body["reason"])
			}
			if testCase.expectedUsage != "" {
				assert.Equal(t, testCase.expectedUsage, body["usage"])
			}
		})
	}
}

func TestStrategyScriptRouterRefusesAUsageInARewriteWithAReason(t *testing.T) {
	fixture := newStrategyScriptRouterUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(4)).
		Return(aStoredScriptOfTheViewer("calculation"), nil).AnyTimes()

	recorder := fixture.send(http.MethodPut, "/strategy-scripts/4",
		`{"name":"均線","script":"package main","resultType":"floatList","usage":"chart"}`)

	require.Equal(t, http.StatusBadRequest, recorder.Code, recorder.Body.String())
	body := map[string]any{}
	require.NoError(t, json.Unmarshal(recorder.Body.Bytes(), &body))
	assert.Equal(t, "strategyScriptUsageNotRewritable", body["reason"])
}

func TestStrategyScriptRouterRefusesAUsageSwitchItCannotRead(t *testing.T) {
	testCases := []struct {
		name   string
		target string
		body   string
	}{
		{name: "an identifier that is not one", target: "/strategy-scripts/abc/usage", body: `{"usage":"chart"}`},
		{name: "a body that is not JSON", target: "/strategy-scripts/4/usage", body: "not json"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// Nothing is stubbed: the refusal lands before anything is read.
			fixture := newStrategyScriptRouterUnderTest(t)

			recorder := fixture.send(http.MethodPut, testCase.target, testCase.body)

			assert.Equal(t, http.StatusBadRequest, recorder.Code)
		})
	}
}

func TestStrategyScriptRouterAnswersWithTheUsage(t *testing.T) {
	fixture := newStrategyScriptRouterUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(4)).
		Return(aStoredScriptOfTheViewer("chart"), nil)
	fixture.strategyScriptRepository.EXPECT().FindAllOwnedBy(gomock.Any(), signedInViewerID).
		Return([]entities.StrategyScript{aStoredScriptOfTheViewer("chart")}, nil)

	readResponse := fixture.send(http.MethodGet, "/strategy-scripts/4", "")
	listResponse := fixture.send(http.MethodGet, "/strategy-scripts", "")

	require.Equal(t, http.StatusOK, readResponse.Code, readResponse.Body.String())
	read := map[string]any{}
	require.NoError(t, json.Unmarshal(readResponse.Body.Bytes(), &read))
	assert.Equal(t, "chart", read["usage"])
	require.Equal(t, http.StatusOK, listResponse.Code, listResponse.Body.String())
	assert.Contains(t, listResponse.Body.String(), `"usage":"chart"`)
}
