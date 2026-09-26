package assistantqueries_test

import (
	"context"
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
	// botsUsingScript is what every bot lookup answers; empty unless a test says otherwise.
	botsUsingScript *[]entities.StrategyBot
	revision        assistantRevisionUnderTest
}

// newStrategyScriptAssistantQueriesUnderTest mocks only storage, so the assistant is bound by every rule a person is.
func newStrategyScriptAssistantQueriesUnderTest(t *testing.T) strategyScriptAssistantQueriesUnderTest {
	controller := gomock.NewController(t)
	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()
	strategyBotRepository := mocks.NewMockIStrategyBotRepository(controller)
	botsUsingScript := &[]entities.StrategyBot{}
	strategyBotRepository.EXPECT().FindAllByStrategyScript(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, uint) ([]entities.StrategyBot, error) {
			return *botsUsingScript, nil
		}).AnyTimes()
	strategyScriptApplication := application.NewStrategyScriptApplication(
		service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository),
		service.NewStrategyBotService(
			strategyBotRepository,
			mocks.NewMockIStrategyBotRunRecordRepository(controller),
			mocks.NewMockIContractTradingSymbolRepository(controller),
			mocks.NewMockIContractMaintenanceMarginTierRepository(controller),
			mocks.NewMockIContractFundingRateSettlementRepository(controller),
			mocks.NewMockIClockProxy(controller)))

	revision := newAssistantRevisionUnderTest(controller, strategyScriptApplication, nil)

	return strategyScriptAssistantQueriesUnderTest{
		listAssistantQuery: assistantqueries.NewStrategyScriptListAssistantQuery(strategyScriptApplication),
		getAssistantQuery:  assistantqueries.NewStrategyScriptGetAssistantQuery(strategyScriptApplication),
		createAssistantQuery: assistantqueries.NewStrategyScriptCreateAssistantQuery(
			strategyScriptApplication, revision.application),
		updateAssistantQuery:     assistantqueries.NewStrategyScriptUpdateAssistantQuery(revision.application),
		strategyScriptRepository: strategyScriptRepository,
		botsUsingScript:          botsUsingScript,
		revision:                 revision,
	}
}

// aStoredStrategyScriptWithKnobs includes knobs because these tests check what a list omits and a read includes.
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
	// A list omits each script's source, the longest and costliest field.
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().
		FindAllOwnedBy(gomock.Any(), assistantViewerID).
		Return([]entities.StrategyScript{aStoredStrategyScriptWithKnobs(1, "二十根均線")}, nil)

	outcome, runError := fixture.listAssistantQuery.Run(t.Context(), assistantOrigin, "{}")

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

	outcome, runError := fixture.listAssistantQuery.Run(t.Context(), assistantOrigin, "{}")

	require.NoError(t, runError)
	assert.JSONEq(t, `{"strategyScripts":[]}`, outcome)
}

func TestStrategyScriptListAssistantQueryReportsAFailureToRead(t *testing.T) {
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	// The caller's own scripts are read first, so a failure there ends before the marketplace is queried.
	fixture.strategyScriptRepository.EXPECT().
		FindAllOwnedBy(gomock.Any(), assistantViewerID).
		Return(nil, errors.New("storage unavailable"))

	_, runError := fixture.listAssistantQuery.Run(t.Context(), assistantOrigin, "{}")

	require.Error(t, runError)
}

func TestStrategyScriptGetAssistantQueryHandsOverTheAlgorithmToo(t *testing.T) {
	// A full read is needed because a rewrite replaces everything.
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(1)).
		Return(aStoredStrategyScriptWithKnobs(1, "二十根均線"), nil)

	outcome, runError := fixture.getAssistantQuery.Run(t.Context(), assistantOrigin, `{"strategyScriptId":1}`)

	require.NoError(t, runError)
	assert.Contains(t, outcome, "func Calculate")
	assert.Contains(t, outcome, "二十根均線")
}

func TestStrategyScriptGetAssistantQueryRelaysTheSystemsOwnWordsForAStrategyScriptThatIsNotThere(t *testing.T) {
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(99)).
		Return(entities.StrategyScript{}, domains.StrategyScriptNotFound(99))

	_, runError := fixture.getAssistantQuery.Run(t.Context(), assistantOrigin, `{"strategyScriptId":99}`)

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

	outcome, runError := fixture.createAssistantQuery.Run(t.Context(), assistantOrigin,
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

			_, runError := fixture.createAssistantQuery.Run(t.Context(), assistantOrigin, testCase.arguments)

			require.Error(t, runError)
			assert.Contains(t, runError.Error(), testCase.expectedMessage)
		})
	}
}

func TestStrategyScriptUpdateAssistantQueryRewritesTheStrategyScriptItNames(t *testing.T) {
	// Created by the assistant in this conversation and used by no bot, so the rewrite needs no confirmation.
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	*fixture.revision.createdInConversation = true
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(1)).
		Return(aStoredStrategyScriptWithKnobs(1, "二十根均線"), nil).AnyTimes()

	updatedStrategyScript := entities.StrategyScript{}
	fixture.strategyScriptRepository.EXPECT().Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
			updatedStrategyScript = strategyScript

			return strategyScript, nil
		})

	outcome, runError := fixture.updateAssistantQuery.Run(t.Context(), assistantOrigin,
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

	_, runError := fixture.updateAssistantQuery.Run(t.Context(), assistantOrigin,
		`{"strategyScriptId":99,"name":"三十根均線","script":"func Calculate() {}"}`)

	require.ErrorIs(t, runError, domains.ErrStrategyScriptNotFound)
}

func TestStrategyScriptAssistantQueriesRefuseArgumentsTheyCannotRead(t *testing.T) {
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)

	assistantQueries := map[string]func(string) (string, error){
		"get": func(arguments string) (string, error) {
			return fixture.getAssistantQuery.Run(t.Context(), assistantOrigin, arguments)
		},
		"create": func(arguments string) (string, error) {
			return fixture.createAssistantQuery.Run(t.Context(), assistantOrigin, arguments)
		},
		"update": func(arguments string) (string, error) {
			return fixture.updateAssistantQuery.Run(t.Context(), assistantOrigin, arguments)
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
	// Read, save and rewrite return the same shape.
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	*fixture.revision.createdInConversation = true
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(1)).
		Return(aStoredStrategyScriptWithKnobs(1, "二十根均線"), nil).AnyTimes()
	fixture.strategyScriptRepository.EXPECT().Update(gomock.Any(), gomock.Any()).
		Return(aStoredStrategyScriptWithKnobs(1, "二十根均線"), nil)

	readOutcome, readError := fixture.getAssistantQuery.Run(t.Context(), assistantOrigin, `{"strategyScriptId":1}`)
	require.NoError(t, readError)

	rewrittenOutcome, rewriteError := fixture.updateAssistantQuery.Run(t.Context(), assistantOrigin,
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
	// The assistant has no standing of its own: it saves and reads as the person who asked.
	t.Run("a strategy script it saves belongs to the asker", func(t *testing.T) {
		fixture := newStrategyScriptAssistantQueriesUnderTest(t)
		storedOwnerID := uint(0)
		fixture.strategyScriptRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ any, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
				storedOwnerID = strategyScript.OwnerID

				return strategyScript, nil
			})

		_, runError := fixture.createAssistantQuery.Run(t.Context(), assistantOrigin,
			`{"name":"二十根均線","script":"func Calculate() {}"}`)

		require.NoError(t, runError)
		assert.Equal(t, assistantViewerID, storedOwnerID)
	})

	t.Run("it cannot read somebody else's strategy script", func(t *testing.T) {
		fixture := newStrategyScriptAssistantQueriesUnderTest(t)
		strangersStrategyScript := aStoredStrategyScriptWithKnobs(7, "別人的")
		strangersStrategyScript.OwnerID = assistantViewerID + 1
		fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(strangersStrategyScript, nil)

		_, runError := fixture.getAssistantQuery.Run(t.Context(), assistantOrigin, `{"strategyScriptId":7}`)

		require.ErrorIs(t, runError, domains.ErrStrategyScriptNotFound)
	})

	t.Run("it cannot rewrite somebody else's strategy script", func(t *testing.T) {
		// Nothing is stubbed on the write side: the refusal must land before any write.
		fixture := newStrategyScriptAssistantQueriesUnderTest(t)
		strangersStrategyScript := aStoredStrategyScriptWithKnobs(7, "別人的")
		strangersStrategyScript.OwnerID = assistantViewerID + 1
		fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(strangersStrategyScript, nil)

		_, runError := fixture.updateAssistantQuery.Run(t.Context(), assistantOrigin,
			`{"strategyScriptId":7,"name":"改過的","script":"func Calculate() {}"}`)

		require.ErrorIs(t, runError, domains.ErrStrategyScriptNotFound)
	})

	t.Run("its list says which ones the asker may rewrite", func(t *testing.T) {
		fixture := newStrategyScriptAssistantQueriesUnderTest(t)
		adopted := aStoredStrategyScriptWithKnobs(2, "別人的")
		adopted.IsAdoptedFromMarketplace = true
		fixture.strategyScriptRepository.EXPECT().
			FindAllOwnedBy(gomock.Any(), assistantViewerID).
			Return([]entities.StrategyScript{aStoredStrategyScriptWithKnobs(1, "我的"), adopted}, nil)

		outcome, runError := fixture.listAssistantQuery.Run(t.Context(), assistantOrigin, "{}")

		require.NoError(t, runError)
		assert.Contains(t, outcome, `"name":"我的","resultType":"floatList","parameterNames":["lookback"],"mine":true`)
		assert.Contains(t, outcome, `"name":"別人的","resultType":"floatList","parameterNames":["lookback"],"mine":false`)
		assert.NotContains(t, outcome, "func Calculate")
	})
}
