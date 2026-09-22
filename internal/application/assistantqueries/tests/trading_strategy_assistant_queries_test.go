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

// assistantTradingStrategyID is the one trading strategy every case below reads or
// rewrites.
const assistantTradingStrategyID = uint(11)

type tradingStrategyAssistantQueriesUnderTest struct {
	createAssistantQuery      *assistantqueries.TradingStrategyCreateAssistantQuery
	updateAssistantQuery      *assistantqueries.TradingStrategyUpdateAssistantQuery
	getAssistantQuery         *assistantqueries.TradingStrategyGetAssistantQuery
	listAssistantQuery        *assistantqueries.TradingStrategyListAssistantQuery
	tradingStrategyRepository *mocks.MockITradingStrategyRepository
	strategyBotRepository     *mocks.MockIStrategyBotRepository
}

// newTradingStrategyAssistantQueriesUnderTest wires the real application, the real
// domain services and the real models, mocking only storage — so every rule that
// governs a person building a trading strategy governs the assistant building one.
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
			strategyBotRepository, strategyBotRunRecordRepository, clockProxy),
	)

	return tradingStrategyAssistantQueriesUnderTest{
		createAssistantQuery:      assistantqueries.NewTradingStrategyCreateAssistantQuery(tradingStrategyApplication),
		updateAssistantQuery:      assistantqueries.NewTradingStrategyUpdateAssistantQuery(tradingStrategyApplication),
		getAssistantQuery:         assistantqueries.NewTradingStrategyGetAssistantQuery(tradingStrategyApplication),
		listAssistantQuery:        assistantqueries.NewTradingStrategyListAssistantQuery(tradingStrategyApplication),
		tradingStrategyRepository: tradingStrategyRepository,
		strategyBotRepository:     strategyBotRepository,
	}
}

// aStoredTradingStrategy is one trading strategy as it comes back from storage: a
// single source A reading hourly candles, buying on A's buy and selling on A's sell.
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

// aWellFormedTradingStrategyArgument is what the assistant sends to build the same
// trading strategy aStoredTradingStrategy describes.
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
	// The assistant acts for the person who asked it. It never names an owner —
	// there is no field for one — so a trading strategy it builds can only ever
	// belong to them.
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
		t.Context(), assistantViewerID, aWellFormedTradingStrategyArgument)

	require.NoError(t, runError)
	assert.Contains(t, outcome, "動能追蹤")
	assert.Contains(t, outcome, `"id":11`)
}

func TestTradingStrategyCreateAssistantQueryOffersNoWayToNameAnOwner(t *testing.T) {
	// An owner the assistant could name is an owner it could get wrong. The schema
	// it is handed is the whole of what it may send, so the absence is checked there.
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)

	assert.NotContains(t, fixture.createAssistantQuery.ArgumentSchema(), "ownerId")
	assert.NotContains(t, fixture.updateAssistantQuery.ArgumentSchema(), "ownerId")
}

func TestTradingStrategyCreateAssistantQueryHandsBackTheRefusalWhenTheNameIsTaken(t *testing.T) {
	// A refusal is the assistant's to read and act on — it renames and sends again.
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		Return(entities.TradingStrategy{}, fmt.Errorf(
			"%w: 交易策略名稱「動能追蹤」已被使用", domains.ErrTradingStrategyNameConflict))

	_, runError := fixture.createAssistantQuery.Run(
		t.Context(), assistantViewerID, aWellFormedTradingStrategyArgument)

	require.ErrorIs(t, runError, domains.ErrTradingStrategyNameConflict)
	assert.Contains(t, runError.Error(), "動能追蹤")
}

func TestTradingStrategyCreateAssistantQueryHandsBackTheRefusalWhenAConditionNamesAnUndeclaredLabel(t *testing.T) {
	// The single most likely thing for the assistant to get wrong, and the one it
	// is best placed to fix: it reads "C was never declared" and declares C.
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	_, runError := fixture.createAssistantQuery.Run(t.Context(), assistantViewerID, `{
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

	_, runError := fixture.createAssistantQuery.Run(t.Context(), assistantViewerID, "not json")

	require.ErrorIs(t, runError, domains.ErrAssistantQueryArgument)
}

func TestTradingStrategyUpdateAssistantQueryRewritesTheNamedOne(t *testing.T) {
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), assistantTradingStrategyID).
		Return(aStoredTradingStrategy(assistantTradingStrategyID, "動能追蹤", assistantViewerID), nil).
		AnyTimes()
	fixture.strategyBotRepository.EXPECT().
		FindAllByTradingStrategy(gomock.Any(), assistantTradingStrategyID).
		Return([]entities.StrategyBot{}, nil)
	fixture.tradingStrategyRepository.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, stored entities.TradingStrategy) (entities.TradingStrategy, error) {
			assert.Equal(t, assistantTradingStrategyID, stored.ID)
			assert.Equal(t, assistantViewerID, stored.OwnerID)
			return stored, nil
		})

	outcome, runError := fixture.updateAssistantQuery.Run(t.Context(), assistantViewerID, `{
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

func TestTradingStrategyUpdateAssistantQueryHandsBackTheRefusalWhenABotIsRunning(t *testing.T) {
	// The assistant relays this rather than working around it: a round that began
	// under one version of the rules and ended under another leaves nobody able to
	// say which version it used.
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), assistantTradingStrategyID).
		Return(aStoredTradingStrategy(assistantTradingStrategyID, "動能追蹤", assistantViewerID), nil).
		AnyTimes()
	fixture.strategyBotRepository.EXPECT().
		FindAllByTradingStrategy(gomock.Any(), assistantTradingStrategyID).
		Return([]entities.StrategyBot{
			{ID: 3, Name: "夜班", TradingStrategyID: assistantTradingStrategyID, RunState: "running"},
		}, nil)

	_, runError := fixture.updateAssistantQuery.Run(t.Context(), assistantViewerID, `{
      "tradingStrategyId": 11,
      "name": "動能追蹤",
      "signalSources": [{"label": "A", "strategyScriptId": 9, "aggregationInterval": "1h"}],
      "buyCondition": {"sourceLabel": "A", "signal": "buy"},
      "sellCondition": {"sourceLabel": "A", "signal": "sell"}
    }`)

	require.Error(t, runError)
	assert.Contains(t, runError.Error(), "夜班")
}

func TestTradingStrategyGetAssistantQueryReadsItInFull(t *testing.T) {
	// Reading in full is what makes rewriting possible: a rewrite replaces
	// everything, so the assistant has to know the rest before it changes one knob.
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), assistantTradingStrategyID).
		Return(aStoredTradingStrategy(assistantTradingStrategyID, "動能追蹤", assistantViewerID), nil)

	outcome, runError := fixture.getAssistantQuery.Run(
		t.Context(), assistantViewerID, `{"tradingStrategyId": 11}`)

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
		t.Context(), assistantViewerID, `{"tradingStrategyId": 11}`)

	require.ErrorIs(t, runError, domains.ErrTradingStrategyNotFound)
}

func TestTradingStrategyListAssistantQueryNamesEachOneWithItsCoarseness(t *testing.T) {
	// The coarseness travels with the digest so the assistant can tell at a glance
	// which of these it cannot replay, without reading each one in full first. The
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)

	fixture.tradingStrategyRepository.EXPECT().
		FindAllByOwner(gomock.Any(), assistantViewerID).
		Return([]entities.TradingStrategy{
			aStoredTradingStrategy(assistantTradingStrategyID, "動能追蹤", assistantViewerID),
		}, nil)

	outcome, runError := fixture.listAssistantQuery.Run(t.Context(), assistantViewerID, "")

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

	outcome, runError := fixture.listAssistantQuery.Run(t.Context(), assistantViewerID, "")

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
	// The assistant reaches strategy scripts through the same three gates a person
	// does. A script that is somebody else's and not on the marketplace comes back as
	// one that is not there — which is what stops a trading strategy's sources
	// becoming a way to probe for other people's algorithms.
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
				mocks.NewMockIClockProxy(controller)),
		))

	_, runError := createAssistantQuery.Run(
		t.Context(), assistantViewerID, aWellFormedTradingStrategyArgument)

	require.ErrorIs(t, runError, domains.ErrStrategyScriptNotFound)
}

func TestTradingStrategyCreateAssistantQueryHandsBackTheRefusalWhenASourceSetsAKnobNobodyDeclared(t *testing.T) {
	// The other mistake the assistant is well placed to fix itself: it reads which
	// knob was never declared and drops it. Caught here rather than at three in the
	// morning when the script it feeds finally runs.
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	_, runError := fixture.createAssistantQuery.Run(t.Context(), assistantViewerID, `{
      "name": "動能追蹤",
      "signalSources": [{"label": "A", "strategyScriptId": 9, "aggregationInterval": "1h",
        "parameterValues": [{"name": "這支腳本沒宣告過的參數", "value": 3}]}],
      "buyCondition": {"sourceLabel": "A", "signal": "buy"},
      "sellCondition": {"sourceLabel": "A", "signal": "sell"}
    }`)

	require.Error(t, runError)
	assert.Contains(t, runError.Error(), "這支腳本沒宣告過的參數")
}

// The assistant has no set of rules to pick, and the schema says so — otherwise it
// would keep offering a choice that is refused on arrival.
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
