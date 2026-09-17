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

type strategyScriptAssistantQueriesUnderTest struct {
	listAssistantQuery       *assistantqueries.StrategyScriptListAssistantQuery
	getAssistantQuery        *assistantqueries.StrategyScriptGetAssistantQuery
	createAssistantQuery     *assistantqueries.StrategyScriptCreateAssistantQuery
	updateAssistantQuery     *assistantqueries.StrategyScriptUpdateAssistantQuery
	strategyScriptRepository *mocks.MockIStrategyScriptRepository
}

// newStrategyScriptAssistantQueriesUnderTest wires the real domain service and real domain
// models, mocking only storage — so every rule that governs a person saving a strategy script
// governs the assistant saving one.
func newStrategyScriptAssistantQueriesUnderTest(t *testing.T) strategyScriptAssistantQueriesUnderTest {
	controller := gomock.NewController(t)
	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()
	strategyScriptApplication := application.NewStrategyScriptApplication(
		service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository))

	return strategyScriptAssistantQueriesUnderTest{
		listAssistantQuery:       assistantqueries.NewStrategyScriptListAssistantQuery(strategyScriptApplication),
		getAssistantQuery:        assistantqueries.NewStrategyScriptGetAssistantQuery(strategyScriptApplication),
		createAssistantQuery:     assistantqueries.NewStrategyScriptCreateAssistantQuery(strategyScriptApplication),
		updateAssistantQuery:     assistantqueries.NewStrategyScriptUpdateAssistantQuery(strategyScriptApplication),
		strategyScriptRepository: strategyScriptRepository,
	}
}

// aStoredStrategyScriptWithKnobs is a strategy script as it comes back from storage, knobs and
// all. The knobs matter here because what a list may leave out and what a read must
// include is exactly what these tests are about.
func aStoredStrategyScriptWithKnobs(id uint, name string) entities.StrategyScript {
	return entities.StrategyScript{
		ID:         id,
		OwnerID:    assistantViewerID,
		Name:       name,
		Script:     "func Calculate(candles []vo.KCandleVo) map[string]float64 { return nil }",
		ResultType: "floatList",
		CreatedAt:  time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC),
		Parameters: []entities.StrategyScriptParameter{
			{Name: "lookback", Kind: "lookbackCount", DefaultValue: 20},
		},
	}
}

func TestStrategyScriptListAssistantQueryNamesEachStrategyScriptWithoutSendingItsAlgorithm(t *testing.T) {
	// A list is for choosing from, and a script is the longest thing a strategy script holds.
	// Sending every script every time would be the most expensive habit it could form.
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().
		FindAllOwnedBy(gomock.Any(), assistantViewerID).
		Return([]entities.StrategyScript{aStoredStrategyScriptWithKnobs(1, "二十根均線")}, nil)
	fixture.strategyScriptRepository.EXPECT().
		FindAllAdoptedBy(gomock.Any(), assistantViewerID).Return([]entities.PublishedStrategyScript{}, nil)

	outcome, runError := fixture.listAssistantQuery.Run(t.Context(), assistantViewerID, "{}")

	require.NoError(t, runError)
	assert.JSONEq(t,
		`{"strategyScripts":[{"id":1,"name":"二十根均線","resultType":"floatList",`+
			`"parameterNames":["lookback"],"mine":true}]}`,
		outcome)
	assert.NotContains(t, outcome, "func Calculate")
}

func TestStrategyScriptListAssistantQueryAnswersHoldingNoneWithAnEmptyList(t *testing.T) {
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().
		FindAllOwnedBy(gomock.Any(), assistantViewerID).Return([]entities.StrategyScript{}, nil)
	fixture.strategyScriptRepository.EXPECT().
		FindAllAdoptedBy(gomock.Any(), assistantViewerID).Return([]entities.PublishedStrategyScript{}, nil)

	outcome, runError := fixture.listAssistantQuery.Run(t.Context(), assistantViewerID, "{}")

	require.NoError(t, runError)
	assert.JSONEq(t, `{"strategyScripts":[]}`, outcome)
}

func TestStrategyScriptListAssistantQueryReportsAFailureToRead(t *testing.T) {
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	// The caller's own strategy scripts are read first, so a failure there ends the answer
	// before the shelf is ever asked about.
	fixture.strategyScriptRepository.EXPECT().
		FindAllOwnedBy(gomock.Any(), assistantViewerID).
		Return(nil, errors.New("storage unavailable"))

	_, runError := fixture.listAssistantQuery.Run(t.Context(), assistantViewerID, "{}")

	require.Error(t, runError)
}

func TestStrategyScriptGetAssistantQueryHandsOverTheAlgorithmToo(t *testing.T) {
	// Reading it in full is what makes changing it possible: a rewrite replaces
	// everything, so the assistant has to know the rest before it can send it back.
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(1)).
		Return(aStoredStrategyScriptWithKnobs(1, "二十根均線"), nil)

	outcome, runError := fixture.getAssistantQuery.Run(t.Context(), assistantViewerID, `{"strategyScriptId":1}`)

	require.NoError(t, runError)
	assert.Contains(t, outcome, "func Calculate")
	assert.Contains(t, outcome, "二十根均線")
}

func TestStrategyScriptGetAssistantQueryRelaysTheSystemsOwnWordsForAStrategyScriptThatIsNotThere(t *testing.T) {
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(99)).
		Return(entities.StrategyScript{}, domains.StrategyScriptNotFound(99))

	_, runError := fixture.getAssistantQuery.Run(t.Context(), assistantViewerID, `{"strategyScriptId":99}`)

	require.ErrorIs(t, runError, domains.ErrStrategyScriptNotFound)
}

func TestStrategyScriptCreateAssistantQuerySavesWhatTheAssistantDeclared(t *testing.T) {
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)

	savedStrategyScript := entities.StrategyScript{}
	fixture.strategyScriptRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
			savedStrategyScript = strategyScript
			strategyScript.ID = 5

			return strategyScript, nil
		})

	outcome, runError := fixture.createAssistantQuery.Run(t.Context(), assistantViewerID,
		`{"name":"五十根均線","script":"func Calculate() {}","resultType":"floatList",`+
			`"parameters":[{"name":"lookback","kind":"lookbackCount","defaultValue":50}]}`)

	require.NoError(t, runError)
	assert.Equal(t, "五十根均線", savedStrategyScript.Name)
	require.Len(t, savedStrategyScript.Parameters, 1)
	assert.Equal(t, "lookback", savedStrategyScript.Parameters[0].Name)
	assert.Contains(t, outcome, `"id":5`)
}

func TestStrategyScriptCreateAssistantQueryIsBoundByEveryRuleAPersonsSaveIsBoundBy(t *testing.T) {
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
			expectedMessage: "必須給策略腳本取一個名稱",
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
			// a second strategy script under a name it cannot have.
			name:            "a name another strategy script already holds",
			arguments:       `{"name":"二十根均線","script":"func Calculate() {}"}`,
			storageAnswer:   domains.ErrStrategyScriptNameConflict,
			expectsSave:     true,
			expectedMessage: "strategy script name already in use",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newStrategyScriptAssistantQueriesUnderTest(t)
			if testCase.expectsSave {
				fixture.strategyScriptRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
					Return(entities.StrategyScript{}, testCase.storageAnswer)
			}

			_, runError := fixture.createAssistantQuery.Run(t.Context(), assistantViewerID, testCase.arguments)

			require.Error(t, runError)
			assert.Contains(t, runError.Error(), testCase.expectedMessage)
		})
	}
}

func TestStrategyScriptUpdateAssistantQueryRewritesTheStrategyScriptItNames(t *testing.T) {
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(1)).
		Return(aStoredStrategyScriptWithKnobs(1, "二十根均線"), nil)

	updatedStrategyScript := entities.StrategyScript{}
	fixture.strategyScriptRepository.EXPECT().Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
			updatedStrategyScript = strategyScript

			return strategyScript, nil
		})

	outcome, runError := fixture.updateAssistantQuery.Run(t.Context(), assistantViewerID,
		`{"strategyScriptId":1,"name":"三十根均線","script":"func Calculate() {}","resultType":"floatList",`+
			`"parameters":[{"name":"lookback","kind":"lookbackCount","defaultValue":30}]}`)

	require.NoError(t, runError)
	assert.Equal(t, uint(1), updatedStrategyScript.ID)
	assert.Equal(t, "三十根均線", updatedStrategyScript.Name)
	require.Len(t, updatedStrategyScript.Parameters, 1)
	assert.InDelta(t, 30.0, updatedStrategyScript.Parameters[0].DefaultValue, 0)
	assert.Contains(t, outcome, "三十根均線")
}

func TestStrategyScriptUpdateAssistantQueryReportsAStrategyScriptThatIsNotThere(t *testing.T) {
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(99)).
		Return(entities.StrategyScript{}, domains.StrategyScriptNotFound(99))

	_, runError := fixture.updateAssistantQuery.Run(t.Context(), assistantViewerID,
		`{"strategyScriptId":99,"name":"三十根均線","script":"func Calculate() {}"}`)

	require.ErrorIs(t, runError, domains.ErrStrategyScriptNotFound)
}

func TestStrategyScriptAssistantQueriesRefuseArgumentsTheyCannotRead(t *testing.T) {
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)

	assistantQueries := map[string]func(string) (string, error){
		"get": func(arguments string) (string, error) {
			return fixture.getAssistantQuery.Run(t.Context(), assistantViewerID, arguments)
		},
		"create": func(arguments string) (string, error) {
			return fixture.createAssistantQuery.Run(t.Context(), assistantViewerID, arguments)
		},
		"update": func(arguments string) (string, error) {
			return fixture.updateAssistantQuery.Run(t.Context(), assistantViewerID, arguments)
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

func TestStrategyScriptAssistantQueriesRenderStrategyScriptsTheSameWayEveryTime(t *testing.T) {
	// Reading one, saving one and rewriting one hand back the same shape, so the
	// assistant never has to learn two ways of looking at the same thing.
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(1)).
		Return(aStoredStrategyScriptWithKnobs(1, "二十根均線"), nil).Times(2)
	fixture.strategyScriptRepository.EXPECT().Update(gomock.Any(), gomock.Any()).
		Return(aStoredStrategyScriptWithKnobs(1, "二十根均線"), nil)

	readOutcome, readError := fixture.getAssistantQuery.Run(t.Context(), assistantViewerID, `{"strategyScriptId":1}`)
	require.NoError(t, readError)

	rewrittenOutcome, rewriteError := fixture.updateAssistantQuery.Run(t.Context(), assistantViewerID,
		`{"strategyScriptId":1,"name":"二十根均線","script":"func Calculate() {}","resultType":"floatList"}`)
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

func TestStrategyScriptAssistantQueriesActAsWhoeverAskedThem(t *testing.T) {
	// The assistant has no standing of its own. What it saves belongs to the person
	// who asked, and what it can read is what they can read — otherwise "every
	// strategy script has an owner" gets its first exception the moment somebody asks the
	// assistant to save one.
	t.Run("a strategy script it saves belongs to the asker", func(t *testing.T) {
		fixture := newStrategyScriptAssistantQueriesUnderTest(t)
		storedOwnerID := uint(0)
		fixture.strategyScriptRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ any, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
				storedOwnerID = strategyScript.OwnerID

				return strategyScript, nil
			})

		_, runError := fixture.createAssistantQuery.Run(t.Context(), assistantViewerID,
			`{"name":"二十根均線","script":"func Calculate() {}"}`)

		require.NoError(t, runError)
		assert.Equal(t, assistantViewerID, storedOwnerID)
	})

	t.Run("it cannot read somebody else's strategy script", func(t *testing.T) {
		fixture := newStrategyScriptAssistantQueriesUnderTest(t)
		strangersStrategyScript := aStoredStrategyScriptWithKnobs(7, "別人的")
		strangersStrategyScript.OwnerID = assistantViewerID + 1
		fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(strangersStrategyScript, nil)

		_, runError := fixture.getAssistantQuery.Run(t.Context(), assistantViewerID, `{"strategyScriptId":7}`)

		require.ErrorIs(t, runError, domains.ErrStrategyScriptNotFound)
	})

	t.Run("it cannot rewrite somebody else's strategy script", func(t *testing.T) {
		// Nothing is stubbed on the writing side: the refusal must land before any
		// write goes out.
		fixture := newStrategyScriptAssistantQueriesUnderTest(t)
		strangersStrategyScript := aStoredStrategyScriptWithKnobs(7, "別人的")
		strangersStrategyScript.OwnerID = assistantViewerID + 1
		fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(strangersStrategyScript, nil)

		_, runError := fixture.updateAssistantQuery.Run(t.Context(), assistantViewerID,
			`{"strategyScriptId":7,"name":"改過的","script":"func Calculate() {}"}`)

		require.ErrorIs(t, runError, domains.ErrStrategyScriptNotFound)
	})

	t.Run("its list says which ones the asker may rewrite", func(t *testing.T) {
		fixture := newStrategyScriptAssistantQueriesUnderTest(t)
		adopted := aStoredStrategyScriptWithKnobs(2, "別人的")
		adopted.OwnerID = assistantViewerID + 1
		adopted.Owner = entities.User{ID: adopted.OwnerID, Email: "someone@example.com"}
		fixture.strategyScriptRepository.EXPECT().
			FindAllOwnedBy(gomock.Any(), assistantViewerID).
			Return([]entities.StrategyScript{aStoredStrategyScriptWithKnobs(1, "我的")}, nil)
		fixture.strategyScriptRepository.EXPECT().
			FindAllAdoptedBy(gomock.Any(), assistantViewerID).
			Return([]entities.PublishedStrategyScript{{StrategyScriptID: 2, StrategyScript: adopted}}, nil)

		outcome, runError := fixture.listAssistantQuery.Run(t.Context(), assistantViewerID, "{}")

		require.NoError(t, runError)
		assert.Contains(t, outcome, `"name":"我的","resultType":"floatList","parameterNames":["lookback"],"mine":true`)
		assert.Contains(t, outcome, `"name":"別人的","resultType":"floatList","parameterNames":["lookback"],"mine":false`)
		assert.NotContains(t, outcome, "func Calculate")
	})
}
