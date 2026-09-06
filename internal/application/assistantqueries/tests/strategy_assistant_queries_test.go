package assistantqueries_test

import (
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/application/assistantqueries"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

type strategyAssistantQueriesUnderTest struct {
	listAssistantQuery   *assistantqueries.StrategyListAssistantQuery
	getAssistantQuery    *assistantqueries.StrategyGetAssistantQuery
	createAssistantQuery *assistantqueries.StrategyCreateAssistantQuery
	updateAssistantQuery *assistantqueries.StrategyUpdateAssistantQuery
	strategyRepository   *mocks.MockIStrategyRepository
}

// newStrategyAssistantQueriesUnderTest wires the real domain service and real domain
// models, mocking only storage — so every rule that governs a person saving a strategy
// governs the assistant saving one.
func newStrategyAssistantQueriesUnderTest(t *testing.T) strategyAssistantQueriesUnderTest {
	controller := gomock.NewController(t)
	strategyRepository := mocks.NewMockIStrategyRepository(controller)
	strategyApplication := application.NewStrategyApplication(
		service.NewStrategyService(strategyRepository))

	return strategyAssistantQueriesUnderTest{
		listAssistantQuery:   assistantqueries.NewStrategyListAssistantQuery(strategyApplication),
		getAssistantQuery:    assistantqueries.NewStrategyGetAssistantQuery(strategyApplication),
		createAssistantQuery: assistantqueries.NewStrategyCreateAssistantQuery(strategyApplication),
		updateAssistantQuery: assistantqueries.NewStrategyUpdateAssistantQuery(strategyApplication),
		strategyRepository:   strategyRepository,
	}
}

// aStoredStrategyWithKnobs is a strategy as it comes back from storage, knobs and
// all. The knobs matter here because what a list may leave out and what a read must
// include is exactly what these tests are about.
func aStoredStrategyWithKnobs(id uint, name string) entities.Strategy {
	return entities.Strategy{
		ID:         id,
		Name:       name,
		Script:     "func Calculate(candles []vo.KCandleVo) map[string]float64 { return nil }",
		ResultType: "floatList",
		CreatedAt:  time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC),
		Parameters: []entities.StrategyParameter{
			{Name: "lookback", Kind: "lookbackCount", DefaultValue: 20},
		},
	}
}

func TestStrategyListAssistantQueryNamesEachStrategyWithoutSendingItsAlgorithm(t *testing.T) {
	// A list is for choosing from, and a script is the longest thing a strategy holds.
	// Sending every script every time would be the most expensive habit it could form.
	fixture := newStrategyAssistantQueriesUnderTest(t)
	fixture.strategyRepository.EXPECT().FindAll(gomock.Any()).
		Return([]entities.Strategy{aStoredStrategyWithKnobs(1, "二十根均線")}, nil)

	outcome, runError := fixture.listAssistantQuery.Run(t.Context(), "{}")

	require.NoError(t, runError)
	assert.JSONEq(t,
		`{"strategies":[{"id":1,"name":"二十根均線","resultType":"floatList","parameterNames":["lookback"]}]}`,
		outcome)
	assert.NotContains(t, outcome, "func Calculate")
}

func TestStrategyListAssistantQueryAnswersHoldingNoneWithAnEmptyList(t *testing.T) {
	fixture := newStrategyAssistantQueriesUnderTest(t)
	fixture.strategyRepository.EXPECT().FindAll(gomock.Any()).Return([]entities.Strategy{}, nil)

	outcome, runError := fixture.listAssistantQuery.Run(t.Context(), "{}")

	require.NoError(t, runError)
	assert.JSONEq(t, `{"strategies":[]}`, outcome)
}

func TestStrategyListAssistantQueryReportsAFailureToRead(t *testing.T) {
	fixture := newStrategyAssistantQueriesUnderTest(t)
	fixture.strategyRepository.EXPECT().FindAll(gomock.Any()).
		Return(nil, errors.New("storage unavailable"))

	_, runError := fixture.listAssistantQuery.Run(t.Context(), "{}")

	require.Error(t, runError)
}

func TestStrategyGetAssistantQueryHandsOverTheAlgorithmToo(t *testing.T) {
	// Reading it in full is what makes changing it possible: a rewrite replaces
	// everything, so the assistant has to know the rest before it can send it back.
	fixture := newStrategyAssistantQueriesUnderTest(t)
	fixture.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(1)).
		Return(aStoredStrategyWithKnobs(1, "二十根均線"), nil)

	outcome, runError := fixture.getAssistantQuery.Run(t.Context(), `{"strategyId":1}`)

	require.NoError(t, runError)
	assert.Contains(t, outcome, "func Calculate")
	assert.Contains(t, outcome, "二十根均線")
}

func TestStrategyGetAssistantQueryRelaysTheSystemsOwnWordsForAStrategyThatIsNotThere(t *testing.T) {
	fixture := newStrategyAssistantQueriesUnderTest(t)
	fixture.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(99)).
		Return(entities.Strategy{}, domains.StrategyNotFound(99))

	_, runError := fixture.getAssistantQuery.Run(t.Context(), `{"strategyId":99}`)

	require.ErrorIs(t, runError, domains.ErrStrategyNotFound)
}

func TestStrategyCreateAssistantQuerySavesWhatTheAssistantDeclared(t *testing.T) {
	fixture := newStrategyAssistantQueriesUnderTest(t)

	savedStrategy := entities.Strategy{}
	fixture.strategyRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, strategy entities.Strategy) (entities.Strategy, error) {
			savedStrategy = strategy
			strategy.ID = 5

			return strategy, nil
		})

	outcome, runError := fixture.createAssistantQuery.Run(t.Context(),
		`{"name":"五十根均線","script":"func Calculate() {}","resultType":"floatList",`+
			`"parameters":[{"name":"lookback","kind":"lookbackCount","defaultValue":50}]}`)

	require.NoError(t, runError)
	assert.Equal(t, "五十根均線", savedStrategy.Name)
	require.Len(t, savedStrategy.Parameters, 1)
	assert.Equal(t, "lookback", savedStrategy.Parameters[0].Name)
	assert.Contains(t, outcome, `"id":5`)
}

func TestStrategyCreateAssistantQueryIsBoundByEveryRuleAPersonsSaveIsBoundBy(t *testing.T) {
	testCases := []struct {
		name            string
		arguments       string
		storageAnswer   error
		expectsSave     bool
		expectedMessage string
	}{
		{
			name:            "a name nobody wrote",
			arguments:       `{"name":"  ","script":"func Calculate() {}"}`,
			expectedMessage: "必須給策略取一個名稱",
		},
		{
			name:            "no algorithm at all",
			arguments:       `{"name":"五十根均線","script":"   "}`,
			expectedMessage: "必須帶一段指標算式",
		},
		{
			name:            "a value kind the system does not recognise",
			arguments:       `{"name":"五十根均線","script":"func Calculate() {}","resultType":"decimal"}`,
			expectedMessage: "指標值種類",
		},
		{
			// The name is taken, and the assistant relays that rather than inventing
			// a second strategy under a name it cannot have.
			name:            "a name another strategy already holds",
			arguments:       `{"name":"二十根均線","script":"func Calculate() {}"}`,
			storageAnswer:   domains.ErrStrategyNameConflict,
			expectsSave:     true,
			expectedMessage: "strategy name already in use",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newStrategyAssistantQueriesUnderTest(t)
			if testCase.expectsSave {
				fixture.strategyRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
					Return(entities.Strategy{}, testCase.storageAnswer)
			}

			_, runError := fixture.createAssistantQuery.Run(t.Context(), testCase.arguments)

			require.Error(t, runError)
			assert.Contains(t, runError.Error(), testCase.expectedMessage)
		})
	}
}

func TestStrategyUpdateAssistantQueryRewritesTheStrategyItNames(t *testing.T) {
	fixture := newStrategyAssistantQueriesUnderTest(t)
	fixture.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(1)).
		Return(aStoredStrategyWithKnobs(1, "二十根均線"), nil)

	updatedStrategy := entities.Strategy{}
	fixture.strategyRepository.EXPECT().Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, strategy entities.Strategy) (entities.Strategy, error) {
			updatedStrategy = strategy

			return strategy, nil
		})

	outcome, runError := fixture.updateAssistantQuery.Run(t.Context(),
		`{"strategyId":1,"name":"三十根均線","script":"func Calculate() {}","resultType":"floatList",`+
			`"parameters":[{"name":"lookback","kind":"lookbackCount","defaultValue":30}]}`)

	require.NoError(t, runError)
	assert.Equal(t, uint(1), updatedStrategy.ID)
	assert.Equal(t, "三十根均線", updatedStrategy.Name)
	require.Len(t, updatedStrategy.Parameters, 1)
	assert.InDelta(t, 30.0, updatedStrategy.Parameters[0].DefaultValue, 0)
	assert.Contains(t, outcome, "三十根均線")
}

func TestStrategyUpdateAssistantQueryReportsAStrategyThatIsNotThere(t *testing.T) {
	fixture := newStrategyAssistantQueriesUnderTest(t)
	fixture.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(99)).
		Return(entities.Strategy{}, domains.StrategyNotFound(99))

	_, runError := fixture.updateAssistantQuery.Run(t.Context(),
		`{"strategyId":99,"name":"三十根均線","script":"func Calculate() {}"}`)

	require.ErrorIs(t, runError, domains.ErrStrategyNotFound)
}

func TestStrategyAssistantQueriesRefuseArgumentsTheyCannotRead(t *testing.T) {
	fixture := newStrategyAssistantQueriesUnderTest(t)

	assistantQueries := map[string]func(string) (string, error){
		"get": func(arguments string) (string, error) {
			return fixture.getAssistantQuery.Run(t.Context(), arguments)
		},
		"create": func(arguments string) (string, error) {
			return fixture.createAssistantQuery.Run(t.Context(), arguments)
		},
		"update": func(arguments string) (string, error) {
			return fixture.updateAssistantQuery.Run(t.Context(), arguments)
		},
	}

	for name, run := range assistantQueries {
		t.Run(name, func(t *testing.T) {
			_, runError := run(`not json at all`)

			require.ErrorIs(t, runError, domains.ErrAssistantQueryArgument)
			assert.Contains(t, runError.Error(), "不是合法的 JSON")
		})
	}
}

func TestStrategyAssistantQueriesRenderStrategiesTheSameWayEveryTime(t *testing.T) {
	// Reading one, saving one and rewriting one hand back the same shape, so the
	// assistant never has to learn two ways of looking at the same thing.
	fixture := newStrategyAssistantQueriesUnderTest(t)
	fixture.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(1)).
		Return(aStoredStrategyWithKnobs(1, "二十根均線"), nil).Times(2)
	fixture.strategyRepository.EXPECT().Update(gomock.Any(), gomock.Any()).
		Return(aStoredStrategyWithKnobs(1, "二十根均線"), nil)

	readOutcome, readError := fixture.getAssistantQuery.Run(t.Context(), `{"strategyId":1}`)
	require.NoError(t, readError)

	rewrittenOutcome, rewriteError := fixture.updateAssistantQuery.Run(t.Context(),
		`{"strategyId":1,"name":"二十根均線","script":"func Calculate() {}","resultType":"floatList"}`)
	require.NoError(t, rewriteError)

	readShape := map[string]json.RawMessage{}
	rewrittenShape := map[string]json.RawMessage{}
	require.NoError(t, json.Unmarshal([]byte(readOutcome), &readShape))
	require.NoError(t, json.Unmarshal([]byte(rewrittenOutcome), &rewrittenShape))
	assert.ElementsMatch(t, keysOf(readShape), keysOf(rewrittenShape))
}

func keysOf(shape map[string]json.RawMessage) []string {
	keys := make([]string, 0, len(shape))
	for key := range shape {
		keys = append(keys, key)
	}

	return keys
}
