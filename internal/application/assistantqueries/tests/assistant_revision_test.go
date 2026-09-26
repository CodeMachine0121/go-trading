package assistantqueries_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application/assistantqueries"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const proposedPendingRevisionID = uint(70)

// scriptLastChangedAt is when the stored script in these tests was last rewritten.
var scriptLastChangedAt = time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC)

const aStrategyScriptRewrite = `{"strategyScriptId":1,"name":"六十根均線","script":"func Calculate() {}","resultType":"floatList"}`

// aProposedStrategyScriptRewrite is the stored form of aStrategyScriptRewrite, proposed against the script as last changed.
func aProposedStrategyScriptRewrite(status vo.AssistantPendingRevisionStatusVo) entities.AssistantPendingRevision {
	return entities.AssistantPendingRevision{
		ID:               proposedPendingRevisionID,
		OwnerID:          assistantViewerID,
		ConversationID:   assistantOrigin.ConversationID,
		AssistantTurnID:  assistantOrigin.TurnID,
		SubjectKind:      string(vo.AssistantRevisionSubjectStrategyScript),
		SubjectID:        1,
		SubjectName:      "二十根均線",
		Content:          aStrategyScriptRewrite,
		SubjectUpdatedAt: scriptLastChangedAt,
		Status:           string(status),
		ProposedAt:       revisionProposedAt,
	}
}

func (fixture strategyScriptAssistantQueriesUnderTest) expectTheStoredScript() {
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(1)).
		Return(aStoredStrategyScriptWithKnobs(1, "二十根均線"), nil).AnyTimes()
}

func (fixture strategyScriptAssistantQueriesUnderTest) expectTheProposal(
	pendingRevision entities.AssistantPendingRevision,
) {
	fixture.revision.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), pendingRevision.ID).
		Return(pendingRevision, nil).AnyTimes()
}

func (fixture strategyScriptAssistantQueriesUnderTest) expectStatusMove(from string, to string, moved bool) {
	fixture.revision.pendingRevisionRepository.EXPECT().
		TransitionStatus(gomock.Any(), proposedPendingRevisionID, from, to).Return(moved, nil)
}

func TestStrategyScriptUpdateAssistantQueryOnlyProposesARewriteOfAScriptThePersonAlreadyHad(t *testing.T) {
	// Update is unstubbed, so writing the script would fail the test.
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.expectTheStoredScript()
	fixture.revision.pendingRevisionRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, proposed entities.AssistantPendingRevision) (entities.AssistantPendingRevision, error) {
			expected := aProposedStrategyScriptRewrite(vo.AssistantPendingRevisionPending)
			expected.ID = 0
			assert.Equal(t, expected, proposed)
			proposed.ID = proposedPendingRevisionID

			return proposed, nil
		})

	outcome, runError := fixture.updateAssistantQuery.Run(t.Context(), assistantOrigin, aStrategyScriptRewrite)

	require.NoError(t, runError)
	report := map[string]any{}
	require.NoError(t, json.Unmarshal([]byte(outcome), &report))
	assert.Equal(t, float64(proposedPendingRevisionID), report["pendingRevisionId"])
	assert.Equal(t, "pending", report["status"])
	assert.Equal(t, domains.AssistantRevisionProposedNotice, report["notice"])
	// Whether it was created here is asked of this very conversation.
	assert.Equal(t, []uint{assistantOrigin.ConversationID}, *fixture.revision.askedAboutConversations)
}

func TestStrategyScriptUpdateAssistantQueryRefusesContentThatCouldNeverBeWrittenWithoutProposingIt(t *testing.T) {
	// Neither Save nor Update is stubbed: nothing may be proposed or written.
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.expectTheStoredScript()

	_, runError := fixture.updateAssistantQuery.Run(t.Context(), assistantOrigin,
		`{"strategyScriptId":1,"name":"","script":"func Calculate() {}","resultType":"floatList"}`)

	require.ErrorIs(t, runError, domains.ErrStrategyScriptValidation)
	assert.Contains(t, runError.Error(), "必須給策略腳本取一個名稱")
}

func TestStrategyScriptUpdateAssistantQueryProposesARewriteOfItsOwnScriptOnceABotUsesIt(t *testing.T) {
	// Created in this conversation, but a stopped bot's trading strategy names it.
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	*fixture.revision.createdInConversation = true
	*fixture.botsUsingScript = []entities.StrategyBot{
		{OwnerID: assistantViewerID, Name: "尾盤回補", RunState: string(vo.StrategyBotStopped)},
	}
	fixture.expectTheStoredScript()
	fixture.revision.pendingRevisionRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(aProposedStrategyScriptRewrite(vo.AssistantPendingRevisionPending), nil)

	outcome, runError := fixture.updateAssistantQuery.Run(t.Context(), assistantOrigin, aStrategyScriptRewrite)

	require.NoError(t, runError)
	assert.Contains(t, outcome, `"status":"pending"`)
}

func TestStrategyScriptCreateAssistantQueryRemembersWhatItCreatedInThisConversation(t *testing.T) {
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(aStoredStrategyScriptWithKnobs(8, "二十根均線"), nil)

	_, runError := fixture.createAssistantQuery.Run(t.Context(), assistantOrigin,
		`{"name":"二十根均線","script":"func Calculate() {}","resultType":"floatList"}`)

	require.NoError(t, runError)
	require.Len(t, *fixture.revision.createdSubjects, 1)
	createdSubject := (*fixture.revision.createdSubjects)[0]
	assert.Equal(t, assistantOrigin.ConversationID, createdSubject.ConversationID)
	assert.Equal(t, string(vo.AssistantRevisionSubjectStrategyScript), createdSubject.SubjectKind)
	assert.Equal(t, uint(8), createdSubject.SubjectID)
}

func TestAssistantRevisionApplicationConfirmWritesTheProposedRewrite(t *testing.T) {
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.expectTheStoredScript()
	fixture.expectTheProposal(aProposedStrategyScriptRewrite(vo.AssistantPendingRevisionPending))
	fixture.expectStatusMove("pending", "confirmed", true)
	fixture.strategyScriptRepository.EXPECT().Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
			assert.Equal(t, uint(1), strategyScript.ID)
			assert.Equal(t, "六十根均線", strategyScript.Name)

			return strategyScript, nil
		})

	confirmed, confirmError := fixture.revision.application.ConfirmPendingRevision(
		t.Context(), assistantViewerID, proposedPendingRevisionID)

	require.NoError(t, confirmError)
	assert.Equal(t, "confirmed", confirmed.Status)
	assert.Equal(t, proposedPendingRevisionID, confirmed.ID)
}

func TestAssistantRevisionApplicationConfirmRefusesWithoutWriting(t *testing.T) {
	// Update and every status move are unstubbed unless a case says so: none may happen.
	someoneElsesProposal := aProposedStrategyScriptRewrite(vo.AssistantPendingRevisionPending)
	someoneElsesProposal.OwnerID = assistantViewerID + 1
	proposedBeforeTheLastChange := aProposedStrategyScriptRewrite(vo.AssistantPendingRevisionPending)
	proposedBeforeTheLastChange.SubjectUpdatedAt = scriptLastChangedAt.Add(-time.Hour)

	testCases := []struct {
		name            string
		proposal        entities.AssistantPendingRevision
		expectedError   error
		expectedMessage string
	}{
		{
			name:            "one already confirmed",
			proposal:        aProposedStrategyScriptRewrite(vo.AssistantPendingRevisionConfirmed),
			expectedError:   domains.ErrAssistantPendingRevisionResolved,
			expectedMessage: "這筆修改已經處理過了",
		},
		{
			name:            "one already rejected",
			proposal:        aProposedStrategyScriptRewrite(vo.AssistantPendingRevisionRejected),
			expectedError:   domains.ErrAssistantPendingRevisionResolved,
			expectedMessage: "這筆修改已經處理過了",
		},
		{
			name:            "one whose script was changed after it was proposed",
			proposal:        proposedBeforeTheLastChange,
			expectedError:   domains.ErrAssistantPendingRevisionStale,
			expectedMessage: "提出這筆修改之後，它已經被改過了，請請助手重新提出",
		},
		{
			name:            "someone else's",
			proposal:        someoneElsesProposal,
			expectedError:   domains.ErrAssistantPendingRevisionNotFound,
			expectedMessage: "找不到識別碼為 70 的待確認修改",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newStrategyScriptAssistantQueriesUnderTest(t)
			fixture.expectTheStoredScript()
			fixture.expectTheProposal(testCase.proposal)

			_, confirmError := fixture.revision.application.ConfirmPendingRevision(
				t.Context(), assistantViewerID, proposedPendingRevisionID)

			require.ErrorIs(t, confirmError, testCase.expectedError)
			assert.Contains(t, confirmError.Error(), testCase.expectedMessage)
		})
	}
}

func TestAssistantRevisionApplicationConfirmLeavesTheProposalPendingWhenTheRewriteIsRefused(t *testing.T) {
	testCases := []struct {
		name            string
		botsUsingScript []entities.StrategyBot
		storageAnswer   error
		expectedError   error
		expectedMessage string
	}{
		{
			name: "a running bot uses the script",
			botsUsingScript: []entities.StrategyBot{
				{OwnerID: assistantViewerID, Name: "早盤突破", RunState: string(vo.StrategyBotRunning)},
			},
			expectedError:   domains.ErrStrategyScriptBotRunning,
			expectedMessage: "這幾台機器人正在用它跑：早盤突破，請先停止它們",
		},
		{
			name:          "the new name is already held",
			storageAnswer: domains.ErrStrategyScriptNameConflict,
			expectedError: domains.ErrStrategyScriptNameConflict,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newStrategyScriptAssistantQueriesUnderTest(t)
			*fixture.botsUsingScript = testCase.botsUsingScript
			fixture.expectTheStoredScript()
			fixture.expectTheProposal(aProposedStrategyScriptRewrite(vo.AssistantPendingRevisionPending))
			gomock.InOrder(
				fixture.revision.pendingRevisionRepository.EXPECT().
					TransitionStatus(gomock.Any(), proposedPendingRevisionID, "pending", "confirmed").Return(true, nil),
				fixture.revision.pendingRevisionRepository.EXPECT().
					TransitionStatus(gomock.Any(), proposedPendingRevisionID, "confirmed", "pending").Return(true, nil),
			)
			if testCase.storageAnswer != nil {
				fixture.strategyScriptRepository.EXPECT().Update(gomock.Any(), gomock.Any()).
					Return(entities.StrategyScript{}, testCase.storageAnswer)
			}

			_, confirmError := fixture.revision.application.ConfirmPendingRevision(
				t.Context(), assistantViewerID, proposedPendingRevisionID)

			require.ErrorIs(t, confirmError, testCase.expectedError)
			assert.Contains(t, confirmError.Error(), testCase.expectedMessage)
		})
	}
}

func TestAssistantRevisionApplicationConfirmLosingARaceReadsAsAlreadyHandled(t *testing.T) {
	// The other press moved it first; Update is unstubbed, so it must not be written twice.
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.expectTheStoredScript()
	fixture.expectTheProposal(aProposedStrategyScriptRewrite(vo.AssistantPendingRevisionPending))
	fixture.expectStatusMove("pending", "confirmed", false)

	_, confirmError := fixture.revision.application.ConfirmPendingRevision(
		t.Context(), assistantViewerID, proposedPendingRevisionID)

	require.ErrorIs(t, confirmError, domains.ErrAssistantPendingRevisionResolved)
}

func TestAssistantRevisionApplicationRejectWritesNothing(t *testing.T) {
	// Update is unstubbed: a rejection never touches the script.
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.expectTheProposal(aProposedStrategyScriptRewrite(vo.AssistantPendingRevisionPending))
	fixture.expectStatusMove("pending", "rejected", true)

	rejected, rejectError := fixture.revision.application.RejectPendingRevision(
		t.Context(), assistantViewerID, proposedPendingRevisionID)

	require.NoError(t, rejectError)
	assert.Equal(t, "rejected", rejected.Status)
}

func TestAssistantRevisionApplicationRejectRefusesWhatItMayNotReject(t *testing.T) {
	someoneElsesProposal := aProposedStrategyScriptRewrite(vo.AssistantPendingRevisionPending)
	someoneElsesProposal.OwnerID = assistantViewerID + 1

	testCases := []struct {
		name          string
		proposal      entities.AssistantPendingRevision
		expectedError error
	}{
		{
			name:          "one already confirmed",
			proposal:      aProposedStrategyScriptRewrite(vo.AssistantPendingRevisionConfirmed),
			expectedError: domains.ErrAssistantPendingRevisionResolved,
		},
		{
			name:          "someone else's",
			proposal:      someoneElsesProposal,
			expectedError: domains.ErrAssistantPendingRevisionNotFound,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			fixture := newStrategyScriptAssistantQueriesUnderTest(t)
			fixture.expectTheProposal(testCase.proposal)

			_, rejectError := fixture.revision.application.RejectPendingRevision(
				t.Context(), assistantViewerID, proposedPendingRevisionID)

			require.ErrorIs(t, rejectError, testCase.expectedError)
		})
	}
}

func TestAssistantRevisionApplicationConfirmReportsAProposalThatIsNotThere(t *testing.T) {
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.revision.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), uint(404)).
		Return(entities.AssistantPendingRevision{}, domains.AssistantPendingRevisionNotFound(404))

	_, confirmError := fixture.revision.application.ConfirmPendingRevision(t.Context(), assistantViewerID, 404)

	require.ErrorIs(t, confirmError, domains.ErrAssistantPendingRevisionNotFound)
}

func TestTradingStrategyUpdateAssistantQueryOnlyProposesARewriteOfAStrategyThePersonAlreadyHad(t *testing.T) {
	// Save on the trading strategy is unstubbed, so writing it would fail the test.
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), assistantTradingStrategyID).
		Return(aStoredTradingStrategy(assistantTradingStrategyID, "動能追蹤", assistantViewerID), nil).
		AnyTimes()
	fixture.strategyBotRepository.EXPECT().
		FindAllByTradingStrategy(gomock.Any(), assistantTradingStrategyID).Return([]entities.StrategyBot{}, nil)
	fixture.revision.pendingRevisionRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, proposed entities.AssistantPendingRevision) (entities.AssistantPendingRevision, error) {
			assert.Equal(t, string(vo.AssistantRevisionSubjectTradingStrategy), proposed.SubjectKind)
			assert.Equal(t, assistantTradingStrategyID, proposed.SubjectID)
			assert.JSONEq(t, aTradingStrategyRewrite, proposed.Content)
			proposed.ID = proposedPendingRevisionID

			return proposed, nil
		})

	outcome, runError := fixture.updateAssistantQuery.Run(t.Context(), assistantOrigin, aTradingStrategyRewrite)

	require.NoError(t, runError)
	assert.Contains(t, outcome, `"status":"pending"`)
}

const aTradingStrategyRewrite = `{
  "tradingStrategyId": 11,
  "name": "動能追蹤",
  "signalSources": [{"label": "A", "strategyScriptId": 9, "aggregationInterval": "1h"}],
  "buyCondition": {"sourceLabel": "A", "signal": "buy"},
  "sellCondition": {"sourceLabel": "A", "signal": "sell"}
}`

func TestTradingStrategyCreateAssistantQueryRemembersWhatItCreatedInThisConversation(t *testing.T) {
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(aStoredTradingStrategy(assistantTradingStrategyID, "動能追蹤", assistantViewerID), nil)

	_, runError := fixture.createAssistantQuery.Run(t.Context(), assistantOrigin, aWellFormedTradingStrategyArgument)

	require.NoError(t, runError)
	require.Len(t, *fixture.revision.createdSubjects, 1)
	assert.Equal(t, string(vo.AssistantRevisionSubjectTradingStrategy), (*fixture.revision.createdSubjects)[0].SubjectKind)
	assert.Equal(t, assistantTradingStrategyID, (*fixture.revision.createdSubjects)[0].SubjectID)
}

func TestStrategyScriptCreateAssistantQueryStillReportsTheCreationWhenItCannotBeRemembered(t *testing.T) {
	// Reporting a failure would make the assistant create the same script again.
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	*fixture.revision.createdSubjectFailure = errors.New("storage unreachable")
	fixture.strategyScriptRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(aStoredStrategyScriptWithKnobs(8, "二十根均線"), nil)

	outcome, runError := fixture.createAssistantQuery.Run(t.Context(), assistantOrigin,
		`{"name":"二十根均線","script":"func Calculate() {}","resultType":"floatList"}`)

	require.NoError(t, runError)
	assert.Contains(t, outcome, "二十根均線")
}

func TestStrategyScriptUpdateAssistantQueryReportsAProposalThatCouldNotBeStored(t *testing.T) {
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.expectTheStoredScript()
	storageFailure := errors.New("storage unreachable")
	fixture.revision.pendingRevisionRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.AssistantPendingRevision{}, storageFailure)

	_, runError := fixture.updateAssistantQuery.Run(t.Context(), assistantOrigin, aStrategyScriptRewrite)

	require.ErrorIs(t, runError, storageFailure)
}

func TestAssistantRevisionApplicationConfirmReportsASubjectThatIsGoneAndLeavesItPending(t *testing.T) {
	// No status move is stubbed: the proposal must stay pending so the owner can reject it.
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.expectTheProposal(aProposedStrategyScriptRewrite(vo.AssistantPendingRevisionPending))
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(1)).
		Return(entities.StrategyScript{}, domains.StrategyScriptNotFound(1))

	_, confirmError := fixture.revision.application.ConfirmPendingRevision(
		t.Context(), assistantViewerID, proposedPendingRevisionID)

	require.ErrorIs(t, confirmError, domains.ErrStrategyScriptNotFound)
}

func TestAssistantRevisionApplicationRefusesAKindNobodyCarriesOut(t *testing.T) {
	unknownKindProposal := aProposedStrategyScriptRewrite(vo.AssistantPendingRevisionPending)
	unknownKindProposal.SubjectKind = "strategyBot"

	t.Run("when confirming", func(t *testing.T) {
		fixture := newStrategyScriptAssistantQueriesUnderTest(t)
		fixture.expectTheProposal(unknownKindProposal)

		_, confirmError := fixture.revision.application.ConfirmPendingRevision(
			t.Context(), assistantViewerID, proposedPendingRevisionID)

		assert.ErrorContains(t, confirmError, "strategyBot")
	})

	t.Run("when revising", func(t *testing.T) {
		fixture := newStrategyScriptAssistantQueriesUnderTest(t)

		_, reviseError := fixture.revision.application.Revise(
			t.Context(), assistantOrigin, "strategyBot", `{}`)

		assert.ErrorContains(t, reviseError, "strategyBot")
	})
}

func TestAssistantRevisionAppliersRefuseContentTheyCannotRead(t *testing.T) {
	strategyScriptFixture := newStrategyScriptAssistantQueriesUnderTest(t)
	tradingStrategyFixture := newTradingStrategyAssistantQueriesUnderTest(t)

	testCases := map[string]func() error{
		"a strategy script rewrite, when writing it": func() error {
			_, applyError := strategyScriptFixture.revision.application.Revise(
				t.Context(), assistantOrigin, vo.AssistantRevisionSubjectStrategyScript, `not json`)

			return applyError
		},
		"a trading strategy rewrite, when checking it": func() error {
			_, runError := tradingStrategyFixture.updateAssistantQuery.Run(t.Context(), assistantOrigin, `not json`)

			return runError
		},
	}

	for name, act := range testCases {
		t.Run(name, func(t *testing.T) {
			runError := act()

			require.ErrorIs(t, runError, domains.ErrAssistantQueryArgument)
			assert.Contains(t, runError.Error(), "不是合法的 JSON")
		})
	}
}

func TestAssistantRevisionApplicationConfirmOfATradingStrategyIsRefusedWhileABotRunsIt(t *testing.T) {
	fixture := newTradingStrategyAssistantQueriesUnderTest(t)
	storedStrategy := aStoredTradingStrategy(assistantTradingStrategyID, "動能追蹤", assistantViewerID)
	fixture.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), assistantTradingStrategyID).
		Return(storedStrategy, nil).AnyTimes()
	fixture.strategyBotRepository.EXPECT().FindAllByTradingStrategy(gomock.Any(), assistantTradingStrategyID).
		Return([]entities.StrategyBot{
			{ID: 3, Name: "夜班", TradingStrategyID: assistantTradingStrategyID, RunState: "running"},
		}, nil).AnyTimes()
	proposal := entities.AssistantPendingRevision{
		ID: proposedPendingRevisionID, OwnerID: assistantViewerID,
		SubjectKind: string(vo.AssistantRevisionSubjectTradingStrategy), SubjectID: assistantTradingStrategyID,
		Content: `{"tradingStrategyId": 11, "name": "動能追蹤",
		  "signalSources": [{"label": "A", "strategyScriptId": 9, "aggregationInterval": "1h"}],
		  "buyCondition": {"sourceLabel": "A", "signal": "buy"},
		  "sellCondition": {"sourceLabel": "A", "signal": "sell"}}`,
		SubjectUpdatedAt: storedStrategy.UpdatedAt,
		Status:           string(vo.AssistantPendingRevisionPending),
	}
	fixture.revision.pendingRevisionRepository.EXPECT().FindOne(gomock.Any(), proposedPendingRevisionID).
		Return(proposal, nil).AnyTimes()
	gomock.InOrder(
		fixture.revision.pendingRevisionRepository.EXPECT().
			TransitionStatus(gomock.Any(), proposedPendingRevisionID, "pending", "confirmed").Return(true, nil),
		fixture.revision.pendingRevisionRepository.EXPECT().
			TransitionStatus(gomock.Any(), proposedPendingRevisionID, "confirmed", "pending").Return(true, nil),
	)

	_, confirmError := fixture.revision.application.ConfirmPendingRevision(
		t.Context(), assistantViewerID, proposedPendingRevisionID)

	require.ErrorIs(t, confirmError, domains.ErrTradingStrategyBotRunning)
	assert.Contains(t, confirmError.Error(), "夜班")
}

func TestAssistantRevisionAppliersRefuseToWriteContentTheyCannotRead(t *testing.T) {
	testCases := map[string]func() (string, error){
		"a strategy script": func() (string, error) {
			return assistantqueries.NewStrategyScriptRevisionApplier(nil).Apply(t.Context(), assistantViewerID, `not json`)
		},
		"a trading strategy": func() (string, error) {
			return assistantqueries.NewTradingStrategyRevisionApplier(nil).Apply(t.Context(), assistantViewerID, `not json`)
		},
	}

	for name, apply := range testCases {
		t.Run(name, func(t *testing.T) {
			_, applyError := apply()

			require.ErrorIs(t, applyError, domains.ErrAssistantQueryArgument)
		})
	}
}

func TestAssistantRevisionApplicationConfirmOfOneProposalVoidsTheOtherOnTheSameScript(t *testing.T) {
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	storedScript := aStoredStrategyScriptWithKnobs(1, "二十根均線")
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(1)).
		DoAndReturn(func(context.Context, uint) (entities.StrategyScript, error) {
			return storedScript, nil
		}).AnyTimes()
	fixture.strategyScriptRepository.EXPECT().Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, rewritten entities.StrategyScript) (entities.StrategyScript, error) {
			rewritten.UpdatedAt = scriptLastChangedAt.Add(time.Minute)
			storedScript = rewritten

			return rewritten, nil
		})
	firstProposal := aProposedStrategyScriptRewrite(vo.AssistantPendingRevisionPending)
	secondProposal := aProposedStrategyScriptRewrite(vo.AssistantPendingRevisionPending)
	secondProposal.ID = proposedPendingRevisionID + 1
	fixture.expectTheProposal(firstProposal)
	fixture.expectTheProposal(secondProposal)
	fixture.expectStatusMove("pending", "confirmed", true)

	_, firstError := fixture.revision.application.ConfirmPendingRevision(t.Context(), assistantViewerID, firstProposal.ID)
	_, secondError := fixture.revision.application.ConfirmPendingRevision(t.Context(), assistantViewerID, secondProposal.ID)

	require.NoError(t, firstError)
	require.ErrorIs(t, secondError, domains.ErrAssistantPendingRevisionStale)
	assert.Contains(t, secondError.Error(), "提出這筆修改之後，它已經被改過了，請請助手重新提出")
}

func TestAssistantRevisionApplicationConfirmHandsTheProposalBackEvenWhenThePressIsAbandoned(t *testing.T) {
	// The press is cancelled while the rewrite runs; the hand-back must still reach storage.
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	fixture.expectTheStoredScript()
	fixture.expectTheProposal(aProposedStrategyScriptRewrite(vo.AssistantPendingRevisionPending))
	pressContext, abandonPress := context.WithCancel(t.Context())
	fixture.expectStatusMove("pending", "confirmed", true)
	fixture.strategyScriptRepository.EXPECT().Update(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, entities.StrategyScript) (entities.StrategyScript, error) {
			abandonPress()

			return entities.StrategyScript{}, context.Canceled
		})
	handedBack := false
	fixture.revision.pendingRevisionRepository.EXPECT().
		TransitionStatus(gomock.Any(), proposedPendingRevisionID, "confirmed", "pending").
		DoAndReturn(func(handBackContext context.Context, _ uint, _ string, _ string) (bool, error) {
			handedBack = handBackContext.Err() == nil

			return handedBack, handBackContext.Err()
		})

	_, confirmError := fixture.revision.application.ConfirmPendingRevision(
		pressContext, assistantViewerID, proposedPendingRevisionID)

	require.ErrorIs(t, confirmError, context.Canceled)
	assert.True(t, handedBack, "the revision must be pending again so the owner can press once more")
}

func TestAssistantRevisionAppliersRefuseFieldsTheirCapabilityDoesNotTake(t *testing.T) {
	// Proposed content is shown to the owner verbatim, so nothing in it may be silently dropped on writing.
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)

	_, runError := fixture.updateAssistantQuery.Run(t.Context(), assistantOrigin,
		`{"strategyScriptId":1,"name":"六十根均線","script":"x","resultType":"floatList","ownerId":2}`)

	require.ErrorIs(t, runError, domains.ErrAssistantQueryArgument)
	assert.Contains(t, runError.Error(), "ownerId")
}

func TestAssistantRevisionAppliersRefuseAnythingAfterTheArguments(t *testing.T) {
	// A second object would be shown to the owner but never written, and would break reading the conversation back.
	strategyScriptFixture := newStrategyScriptAssistantQueriesUnderTest(t)
	tradingStrategyFixture := newTradingStrategyAssistantQueriesUnderTest(t)

	testCases := map[string]func() error{
		"a strategy script rewrite": func() error {
			_, runError := strategyScriptFixture.updateAssistantQuery.Run(t.Context(), assistantOrigin,
				aStrategyScriptRewrite+` {"name":"另一個"}`)

			return runError
		},
		"a trading strategy rewrite": func() error {
			_, runError := tradingStrategyFixture.updateAssistantQuery.Run(t.Context(), assistantOrigin,
				aTradingStrategyRewrite+` {"name":"另一個"}`)

			return runError
		},
	}

	for name, act := range testCases {
		t.Run(name, func(t *testing.T) {
			runError := act()

			require.ErrorIs(t, runError, domains.ErrAssistantQueryArgument)
			assert.Contains(t, runError.Error(), "後面不得再有其他內容")
		})
	}
}

func TestStrategyScriptUpdateAssistantQueryCannotProposeRewritingAMarketplaceCopy(t *testing.T) {
	// Neither Save nor Update is stubbed: nothing may be proposed or written.
	fixture := newStrategyScriptAssistantQueriesUnderTest(t)
	copied := aStoredStrategyScriptWithKnobs(1, "二十根均線")
	copied.IsAdoptedFromMarketplace = true
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(1)).Return(copied, nil).AnyTimes()

	_, runError := fixture.updateAssistantQuery.Run(t.Context(), assistantOrigin, aStrategyScriptRewrite)

	require.ErrorIs(t, runError, domains.ErrStrategyScriptFromMarketplace)
	assert.Contains(t, runError.Error(), "從市集加入的策略腳本不能改寫")
}
