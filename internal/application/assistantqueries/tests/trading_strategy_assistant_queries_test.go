package assistantqueries_test

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/application/assistantqueries"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const assistantTradingStrategyID = uint(11)

type tradingStrategyAssistantQueriesUnderTest struct {
	createAssistantQuery      *assistantqueries.TradingStrategyCreateAssistantQuery
	updateAssistantQuery      *assistantqueries.TradingStrategyUpdateAssistantQuery
	getAssistantQuery         *assistantqueries.TradingStrategyGetAssistantQuery
	listAssistantQuery        *assistantqueries.TradingStrategyListAssistantQuery
	tradingStrategyRepository *mocks.MockITradingStrategyRepository
	strategyBotRepository     *mocks.MockIStrategyBotRepository
	revision                  assistantRevisionUnderTest
}

// newTradingStrategyAssistantQueriesUnderTest mocks only storage, so the assistant is bound by every rule a person is.
func newTradingStrategyAssistantQueriesUnderTest(t *testing.T) tradingStrategyAssistantQueriesUnderTest {
	controller := gomock.NewController(t)

	tradingStrategyRepository := mocks.NewMockITradingStrategyRepository(controller)
	strategyBotRepository := mocks.NewMockIStrategyBotRepository(controller)
	strategyBotRunRecordRepository := mocks.NewMockIStrategyBotRunRecordRepository(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)

	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
	strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.StrategyScript, error) {
			return entities.StrategyScript{
				ID: id, OwnerID: assistantViewerID, Name: "均線", Script: "the script",
				ResultType: string(vo.IndicatorResultTypeSignal),
				Parameters: []entities.StrategyScriptParameter{
					{Name: "lookback", Kind: "lookbackCount", DefaultValue: 20},
				},
			}, nil
		}).AnyTimes()
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	tradingStrategyApplication := application.NewTradingStrategyApplication(
		service.NewTradingStrategyService(tradingStrategyRepository),
		service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository),
		service.NewStrategyBotService(
			strategyBotRepository, strategyBotRunRecordRepository,
			mocks.NewMockIContractTradingSymbolRepository(controller),
			mocks.NewMockIContractMaintenanceMarginTierRepository(controller),
			mocks.NewMockIContractFundingRateSettlementRepository(controller),
			clockProxy),
	)

	revision := newAssistantRevisionUnderTest(controller, nil, tradingStrategyApplication)

	return tradingStrategyAssistantQueriesUnderTest{
		createAssistantQuery: assistantqueries.NewTradingStrategyCreateAssistantQuery(
			tradingStrategyApplication, revision.application),
		updateAssistantQuery:      assistantqueries.NewTradingStrategyUpdateAssistantQuery(revision.application),
		revision:                  revision,
		getAssistantQuery:         assistantqueries.NewTradingStrategyGetAssistantQuery(tradingStrategyApplication),
		listAssistantQuery:        assistantqueries.NewTradingStrategyListAssistantQuery(tradingStrategyApplication),
		tradingStrategyRepository: tradingStrategyRepository,
		strategyBotRepository:     strategyBotRepository,
	}
}

// aStoredTradingStrategy has a single hourly source A, buying on A's buy and selling on A's sell.
func aStoredTradingStrategy(id uint, name string, ownerID uint) entities.TradingStrategy {
	return entities.TradingStrategy{
		ID:        id,
		OwnerID:   ownerID,
		Name:      name,
		CreatedAt: time.Date(2026, 9, 17, 8, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 9, 17, 8, 0, 0, 0, time.UTC),
		SignalSources: []entities.TradingStrategySignalSource{
			{
				ID: 20, TradingStrategyID: id, Label: "A",
				StrategyScriptID: 9, AggregationInterval: "1h",
				ParameterValues: []entities.TradingStrategySignalSourceParameterValue{
					{Name: "lookback", Value: 20},
				},
			},
		},
		ConditionNodes: []entities.TradingStrategyConditionNode{
			{ID: 30, TradingStrategyID: id, Side: "buy", SourceLabel: "A", ExpectedSignal: "buy"},
			{ID: 31, TradingStrategyID: id, Side: "sell", SourceLabel: "A", ExpectedSignal: "sell"},
		},
	}
}

// aWellFormedTradingStrategyArgument builds the same strategy as aStoredTradingStrategy.
const aWellFormedTradingStrategyArgument = `{
  "name": "動能追蹤",
  "signalSources": [
    {"label": "A", "strategyScriptId": 9, "aggregationInterval": "1h",
     "parameterValues": [{"name": "lookback", "value": 20}]}
  ],
  "buyCondition": {"sourceLabel": "A", "signal": "buy"},
  "sellCondition": {"sourceLabel": "A", "signal": "sell"}
}`

func TestTradingStrategyCreateAssistantQueryStoresItForWhoeverAskedTheAssistant(t *testing.T) {
	// The assistant cannot name an owner, so what it builds belongs to the asker.
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, stored entities.TradingStrategy) (entities.TradingStrategy, error) {
			assert.Equal(t, assistantViewerID, stored.OwnerID)
			assert.Equal(t, "動能追蹤", stored.Name)
			stored.ID = assistantTradingStrategyID
			return stored, nil
		})

	outcome, runError := fixture.createAssistantQuery.Run(
		t.Context(), assistantOrigin, aWellFormedTradingStrategyArgument)

	require.NoError(t, runError)
	assert.Contains(t, outcome, "動能追蹤")
	assert.Contains(t, outcome, `"id":11`)
}

func TestTradingStrategyCreateAssistantQueryOffersNoWayToNameAnOwner(t *testing.T) {
	// The schema is the whole of what the assistant may send, so the missing owner field is checked there.
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)

	assert.NotContains(t, fixture.createAssistantQuery.ArgumentSchema(), "ownerId")
	assert.NotContains(t, fixture.updateAssistantQuery.ArgumentSchema(), "ownerId")
}

func TestTradingStrategyCreateAssistantQueryHandsBackTheRefusalWhenTheNameIsTaken(t *testing.T) {
	// The assistant reads the refusal and can rename and retry.
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		Return(entities.TradingStrategy{}, fmt.Errorf(
			"%w: 交易策略名稱「動能追蹤」已被使用", domains.ErrTradingStrategyNameConflict))

	_, runError := fixture.createAssistantQuery.Run(
		t.Context(), assistantOrigin, aWellFormedTradingStrategyArgument)

	require.ErrorIs(t, runError, domains.ErrTradingStrategyNameConflict)
	assert.Contains(t, runError.Error(), "動能追蹤")
}

func TestTradingStrategyCreateAssistantQueryHandsBackTheRefusalWhenAConditionNamesAnUndeclaredLabel(t *testing.T) {
	// An undeclared source is the most likely assistant mistake, and the error tells it what to declare.
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	_, runError := fixture.createAssistantQuery.Run(t.Context(), assistantOrigin, `{
      "name": "動能追蹤",
      "signalSources": [{"label": "A", "strategyScriptId": 9, "aggregationInterval": "1h"}],
      "buyCondition": {"sourceLabel": "C", "signal": "buy"},
      "sellCondition": {"sourceLabel": "A", "signal": "sell"}
    }`)

	require.Error(t, runError)
	assert.Contains(t, runError.Error(), "C")
}

func TestTradingStrategyCreateAssistantQueryRefusesArgumentsThatAreNotJson(t *testing.T) {
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)

	_, runError := fixture.createAssistantQuery.Run(t.Context(), assistantOrigin, "not json")

	require.ErrorIs(t, runError, domains.ErrAssistantQueryArgument)
}

func TestTradingStrategyUpdateAssistantQueryRewritesTheNamedOne(t *testing.T) {
	// Created by the assistant in this conversation and followed by no bot, so the rewrite needs no confirmation.
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	*fixture.revision.createdInConversation = true
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), assistantTradingStrategyID).
		Return(aStoredTradingStrategy(assistantTradingStrategyID, "動能追蹤", assistantViewerID), nil).
		AnyTimes()
	fixture.strategyBotRepository.EXPECT().
		FindAllByTradingStrategy(gomock.Any(), assistantTradingStrategyID).
		Return([]entities.StrategyBot{}, nil).AnyTimes()
	fixture.tradingStrategyRepository.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, stored entities.TradingStrategy) (entities.TradingStrategy, error) {
			assert.Equal(t, assistantTradingStrategyID, stored.ID)
			assert.Equal(t, assistantViewerID, stored.OwnerID)
			return stored, nil
		})

	outcome, runError := fixture.updateAssistantQuery.Run(t.Context(), assistantOrigin, `{
      "tradingStrategyId": 11,
      "name": "動能追蹤",
      "signalSources": [{"label": "A", "strategyScriptId": 9, "aggregationInterval": "1h",
        "parameterValues": [{"name": "lookback", "value": 60}]}],
      "buyCondition": {"sourceLabel": "A", "signal": "buy"},
      "sellCondition": {"sourceLabel": "A", "signal": "sell"}
    }`)

	require.NoError(t, runError)
	assert.Contains(t, outcome, "動能追蹤")
}

func TestTradingStrategyUpdateAssistantQueryLeavesTheRewriteForTheOwnerOnceABotFollowsIt(t *testing.T) {
	// Even one the assistant just created is not rewritten behind a bot's back; the refusal comes at confirmation.
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	*fixture.revision.createdInConversation = true
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), assistantTradingStrategyID).
		Return(aStoredTradingStrategy(assistantTradingStrategyID, "動能追蹤", assistantViewerID), nil).
		AnyTimes()
	fixture.strategyBotRepository.EXPECT().
		FindAllByTradingStrategy(gomock.Any(), assistantTradingStrategyID).
		Return([]entities.StrategyBot{
			{ID: 3, Name: "夜班", TradingStrategyID: assistantTradingStrategyID, RunState: "stopped"},
		}, nil)
	fixture.revision.pendingRevisionRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, proposed entities.AssistantPendingRevision) (entities.AssistantPendingRevision, error) {
			proposed.ID = 70

			return proposed, nil
		})

	outcome, runError := fixture.updateAssistantQuery.Run(t.Context(), assistantOrigin, `{
      "tradingStrategyId": 11,
      "name": "動能追蹤",
      "signalSources": [{"label": "A", "strategyScriptId": 9, "aggregationInterval": "1h"}],
      "buyCondition": {"sourceLabel": "A", "signal": "buy"},
      "sellCondition": {"sourceLabel": "A", "signal": "sell"}
    }`)

	require.NoError(t, runError)
	assert.Contains(t, outcome, `"pendingRevisionId":70`)
	assert.Contains(t, outcome, `"status":"pending"`)
}

func TestTradingStrategyGetAssistantQueryReadsItInFull(t *testing.T) {
	// A full read is needed because a rewrite replaces everything.
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), assistantTradingStrategyID).
		Return(aStoredTradingStrategy(assistantTradingStrategyID, "動能追蹤", assistantViewerID), nil)

	outcome, runError := fixture.getAssistantQuery.Run(
		t.Context(), assistantOrigin, `{"tradingStrategyId": 11}`)

	require.NoError(t, runError)
	assert.Contains(t, outcome, "動能追蹤")
	assert.Contains(t, outcome, `"sourceLabel":"A"`)
	assert.Contains(t, outcome, `"lookback"`)
}

func TestTradingStrategyGetAssistantQueryAnswersSomebodyElsesAsNotFound(t *testing.T) {
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), assistantTradingStrategyID).
		Return(aStoredTradingStrategy(assistantTradingStrategyID, "別人的", assistantViewerID+1), nil)

	_, runError := fixture.getAssistantQuery.Run(
		t.Context(), assistantOrigin, `{"tradingStrategyId": 11}`)

	require.ErrorIs(t, runError, domains.ErrTradingStrategyNotFound)
}

func TestTradingStrategyListAssistantQueryNamesEachOneWithItsCoarseness(t *testing.T) {
	// The coarseness is in the digest so the assistant can see which strategies it cannot replay without reading each in full.
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)

	fixture.tradingStrategyRepository.EXPECT().
		FindAllByOwner(gomock.Any(), assistantViewerID).
		Return([]entities.TradingStrategy{
			aStoredTradingStrategy(assistantTradingStrategyID, "動能追蹤", assistantViewerID),
		}, nil)

	outcome, runError := fixture.listAssistantQuery.Run(t.Context(), assistantOrigin, "")

	require.NoError(t, runError)

	digests := struct {
		TradingStrategies []struct {
			ID                   uint     `json:"id"`
			Name                 string   `json:"name"`
			SourceLabels         []string `json:"sourceLabels"`
			AggregationIntervals []string `json:"aggregationIntervals"`
		} `json:"tradingStrategies"`
	}{}
	require.NoError(t, json.Unmarshal([]byte(outcome), &digests))
	require.Len(t, digests.TradingStrategies, 1)
	assert.Equal(t, assistantTradingStrategyID, digests.TradingStrategies[0].ID)
	assert.Equal(t, "動能追蹤", digests.TradingStrategies[0].Name)
	assert.Equal(t, []string{"A"}, digests.TradingStrategies[0].SourceLabels)
	assert.Equal(t, []string{"1h"}, digests.TradingStrategies[0].AggregationIntervals)
}

func TestTradingStrategyListAssistantQueryAnswersHoldingNoneWithAnEmptyList(t *testing.T) {
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindAllByOwner(gomock.Any(), assistantViewerID).
		Return([]entities.TradingStrategy{}, nil)

	outcome, runError := fixture.listAssistantQuery.Run(t.Context(), assistantOrigin, "")

	require.NoError(t, runError)
	assert.JSONEq(t, `{"tradingStrategies":[]}`, outcome)
}

func TestTradingStrategyAssistantQueriesAreNamedSoTheAssistantCanTellThemApart(t *testing.T) {
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)

	assert.Equal(t, "create_trading_strategy", fixture.createAssistantQuery.Name())
	assert.Equal(t, "update_trading_strategy", fixture.updateAssistantQuery.Name())
	assert.Equal(t, "get_trading_strategy", fixture.getAssistantQuery.Name())
	assert.Equal(t, "list_trading_strategies", fixture.listAssistantQuery.Name())
}

func TestTradingStrategyCreateAssistantQueryHandsBackTheRefusalWhenASourceNamesAStrategyScriptItCannotSee(t *testing.T) {
	// Someone else's unpublished script reads as missing, so a strategy's sources can't probe other people's algorithms.
	controller := gomock.NewController(t)

	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
	strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(entities.StrategyScript{ID: 9, OwnerID: assistantViewerID + 1, Script: "別人的"}, nil)
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	tradingStrategyRepository := mocks.NewMockITradingStrategyRepository(controller)
	tradingStrategyRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	createAssistantQuery := assistantqueries.NewTradingStrategyCreateAssistantQuery(
		application.NewTradingStrategyApplication(
			service.NewTradingStrategyService(tradingStrategyRepository),
			service.NewStrategyScriptService(
				strategyScriptRepository, publishedStrategyScriptRepository),
			service.NewStrategyBotService(
				mocks.NewMockIStrategyBotRepository(controller),
				mocks.NewMockIStrategyBotRunRecordRepository(controller),
				mocks.NewMockIContractTradingSymbolRepository(controller),
				mocks.NewMockIContractMaintenanceMarginTierRepository(controller),
				mocks.NewMockIContractFundingRateSettlementRepository(controller),
				mocks.NewMockIClockProxy(controller)),
		),
		// Refused before anything is stored, so nothing is remembered as created.
		newAssistantRevisionUnderTest(controller, nil, nil).application)

	_, runError := createAssistantQuery.Run(
		t.Context(), assistantOrigin, aWellFormedTradingStrategyArgument)

	require.ErrorIs(t, runError, domains.ErrStrategyScriptNotFound)
}

func TestTradingStrategyCreateAssistantQueryHandsBackTheRefusalWhenASourceSetsAKnobNobodyDeclared(t *testing.T) {
	// An undeclared knob is refused at save time by name, rather than failing when the script runs.
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	_, runError := fixture.createAssistantQuery.Run(t.Context(), assistantOrigin, `{
      "name": "動能追蹤",
      "signalSources": [{"label": "A", "strategyScriptId": 9, "aggregationInterval": "1h",
        "parameterValues": [{"name": "這支腳本沒宣告過的參數", "value": 3}]}],
      "buyCondition": {"sourceLabel": "A", "signal": "buy"},
      "sellCondition": {"sourceLabel": "A", "signal": "sell"}
    }`)

	require.Error(t, runError)
	assert.Contains(t, runError.Error(), "這支腳本沒宣告過的參數")
}

// The schema offers no trading mode, since any choice would be refused.
func TestTradingStrategyWritingAssistantQueriesOfferNoTradingMode(t *testing.T) {
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)

	for _, schema := range []string{
		fixture.createAssistantQuery.ArgumentSchema(),
		fixture.updateAssistantQuery.ArgumentSchema(),
	} {
		assert.NotContains(t, schema, "tradingMode")
		assert.NotContains(t, schema, "longShort")
		assert.NotContains(t, schema, "shortOnly")
		assert.NotContains(t, schema, "leveragedLong")
	}
}
