package persistence_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

var queuedAt = time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)

type pendingMessageTestBed struct {
	database   *gorm.DB
	repository *persistence.PendingMessageRepository
	botID      uint
}

func newPendingMessageTestBed(t *testing.T) pendingMessageTestBed {
	t.Helper()

	database := newStrategyBotTestDatabase(t)
	bot, saveError := persistence.NewStrategyBotRepository(database).Save(t.Context(), aBotRow("早盤突破"))
	require.NoError(t, saveError)

	return pendingMessageTestBed{
		database: database, repository: persistence.NewPendingMessageRepository(database), botID: bot.ID,
	}
}

func (testBed pendingMessageTestBed) aRoundMessage(roundDueAt time.Time, text string) entities.PendingMessage {
	return entities.PendingMessage{
		StrategyBotID: testBed.botID, RoundDueAt: &roundDueAt, RecipientUserID: botRowOwnerID,
		Kind: string(vo.PendingMessageRound), Signal: string(vo.SignalBuy), Text: text,
		Status: string(vo.PendingMessageReady), NextAttemptAt: queuedAt, ExpiresAt: queuedAt.Add(5 * time.Minute),
		CreatedAt: queuedAt,
	}
}

func (testBed pendingMessageTestBed) aLifecycleMessage(text string) entities.PendingMessage {
	return entities.PendingMessage{
		StrategyBotID: testBed.botID, RecipientUserID: botRowOwnerID,
		Kind: string(vo.PendingMessageLifecycle), Text: text,
		Status: string(vo.PendingMessageReady), NextAttemptAt: queuedAt, ExpiresAt: queuedAt.Add(time.Hour),
		CreatedAt: queuedAt,
	}
}

func (testBed pendingMessageTestBed) storedMessages(t *testing.T) []entities.PendingMessage {
	t.Helper()

	stored := []entities.PendingMessage{}
	require.NoError(t, testBed.database.WithContext(t.Context()).Order("id ASC").Find(&stored).Error)

	return stored
}

func textsOf(pendingMessages []entities.PendingMessage) []string {
	texts := make([]string, 0, len(pendingMessages))
	for _, pendingMessage := range pendingMessages {
		texts = append(texts, pendingMessage.Text)
	}

	return texts
}

func TestPendingMessageRepositoryQueuesOneMessageARound(t *testing.T) {
	testBed := newPendingMessageTestBed(t)

	require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.aRoundMessage(queuedAt, "Run 52 first")))
	require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.aRoundMessage(queuedAt, "Run 52 again")))

	assert.Equal(t, []string{"Run 52 first"}, textsOf(testBed.storedMessages(t)))
}

func TestPendingMessageRepositoryKeepsEveryMessageABotSaysAboutItself(t *testing.T) {
	testBed := newPendingMessageTestBed(t)

	require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.aLifecycleMessage("started")))
	require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.aLifecycleMessage("stopped")))

	assert.Equal(t, []string{"started", "stopped"}, textsOf(testBed.storedMessages(t)))
}

func TestPendingMessageRepositoryClaim(t *testing.T) {
	testCases := []struct {
		name          string
		firstClaimant string
		secondAt      time.Time
		expected      bool
	}{
		{name: "a message one replica is sending cannot be taken by another", firstClaimant: "replica-a",
			secondAt: queuedAt.Add(time.Minute), expected: false},
		{name: "a message whose sender's claim ran out can be taken again", firstClaimant: "replica-a",
			secondAt: queuedAt.Add(3 * time.Minute), expected: true},
		{name: "a ready message due now is taken", secondAt: queuedAt, expected: true},
		{name: "a ready message not yet due is not taken", secondAt: queuedAt.Add(-time.Second), expected: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testBed := newPendingMessageTestBed(t)
			require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.aRoundMessage(queuedAt, "Run 52")))
			messageID := testBed.storedMessages(t)[0].ID
			if testCase.firstClaimant != "" {
				claimed, claimError := testBed.repository.Claim(
					t.Context(), messageID, testCase.firstClaimant, queuedAt, queuedAt.Add(2*time.Minute))
				require.NoError(t, claimError)
				require.True(t, claimed)
			}

			claimed, claimError := testBed.repository.Claim(
				t.Context(), messageID, "replica-b", testCase.secondAt, testCase.secondAt.Add(2*time.Minute))
			require.NoError(t, claimError)

			assert.Equal(t, testCase.expected, claimed)
		})
	}
}

func TestPendingMessageRepositoryOnlyTheSenderSettlesAMessage(t *testing.T) {
	testCases := []struct {
		name           string
		settledBy      string
		expectedStatus vo.PendingMessageStatusVo
		expectUnsettle bool
	}{
		{name: "the sender marks it sent and it leaves the queue", settledBy: "replica-a",
			expectedStatus: vo.PendingMessageSent, expectUnsettle: false},
		{name: "anyone else's mark changes nothing", settledBy: "replica-b",
			expectedStatus: vo.PendingMessageSending, expectUnsettle: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testBed := newPendingMessageTestBed(t)
			require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.aRoundMessage(queuedAt, "Run 52")))
			messageID := testBed.storedMessages(t)[0].ID
			_, _ = testBed.repository.Claim(t.Context(), messageID, "replica-a", queuedAt, queuedAt.Add(2*time.Minute))

			require.NoError(t, testBed.repository.MarkSent(t.Context(), messageID, testCase.settledBy, queuedAt))

			assert.Equal(t, string(testCase.expectedStatus), testBed.storedMessages(t)[0].Status)
			unsettled, findError := testBed.repository.FindUnsettled(t.Context(), 10)
			require.NoError(t, findError)
			assert.Equal(t, testCase.expectUnsettle, len(unsettled) == 1)
		})
	}
}

func TestPendingMessageRepositoryRescheduleAndAbandon(t *testing.T) {
	testBed := newPendingMessageTestBed(t)
	require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.aRoundMessage(queuedAt, "Run 52")))
	require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.aLifecycleMessage("stopped")))
	stored := testBed.storedMessages(t)
	_, _ = testBed.repository.Claim(t.Context(), stored[0].ID, "replica-a", queuedAt, queuedAt.Add(2*time.Minute))
	_, _ = testBed.repository.Claim(t.Context(), stored[1].ID, "replica-a", queuedAt, queuedAt.Add(2*time.Minute))

	require.NoError(t, testBed.repository.Reschedule(
		t.Context(), stored[0].ID, "replica-a", 3, queuedAt.Add(8*time.Second)))
	require.NoError(t, testBed.repository.Abandon(
		t.Context(), stored[1].ID, "replica-a", "expired", queuedAt))

	settled := testBed.storedMessages(t)
	assert.Equal(t, string(vo.PendingMessageReady), settled[0].Status)
	assert.Equal(t, 3, settled[0].AttemptCount)
	assert.True(t, queuedAt.Add(8*time.Second).Equal(settled[0].NextAttemptAt))
	assert.Empty(t, settled[0].ClaimedBy)
	assert.Equal(t, string(vo.PendingMessageAbandoned), settled[1].Status)
	assert.Equal(t, "expired", settled[1].AbandonReason)
	unsettled, findError := testBed.repository.FindUnsettled(t.Context(), 10)
	require.NoError(t, findError)
	assert.Equal(t, []string{"Run 52"}, textsOf(unsettled))
}

func TestPendingMessageRepositoryFindsUnsettledInQueueOrderUpToALimit(t *testing.T) {
	testBed := newPendingMessageTestBed(t)
	for index, text := range []string{"first", "second", "third"} {
		require.NoError(t, testBed.repository.Enqueue(t.Context(),
			testBed.aRoundMessage(queuedAt.Add(time.Duration(index)*time.Minute), text)))
	}

	unsettled, findError := testBed.repository.FindUnsettled(t.Context(), 2)
	require.NoError(t, findError)

	assert.Equal(t, []string{"first", "second"}, textsOf(unsettled))
}

func TestPendingMessageRepositoryDropsOnlyWhatSettledLongEnoughAgo(t *testing.T) {
	testBed := newPendingMessageTestBed(t)
	require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.aLifecycleMessage("sent eight days ago")))
	require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.aLifecycleMessage("abandoned six days ago")))
	require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.aLifecycleMessage("still waiting")))
	stored := testBed.storedMessages(t)
	_, _ = testBed.repository.Claim(t.Context(), stored[0].ID, "replica-a", queuedAt, queuedAt.Add(2*time.Minute))
	_, _ = testBed.repository.Claim(t.Context(), stored[1].ID, "replica-a", queuedAt, queuedAt.Add(2*time.Minute))
	require.NoError(t, testBed.repository.MarkSent(t.Context(), stored[0].ID, "replica-a", queuedAt.Add(-8*24*time.Hour)))
	require.NoError(t, testBed.repository.Abandon(
		t.Context(), stored[1].ID, "replica-a", "expired", queuedAt.Add(-6*24*time.Hour)))

	require.NoError(t, testBed.repository.DeleteSettledBefore(t.Context(), queuedAt.Add(-7*24*time.Hour)))

	assert.Equal(t, []string{"abandoned six days ago", "still waiting"}, textsOf(testBed.storedMessages(t)))
}

func TestPendingMessageRepositoryForgetsWhatADeletedBotHadNotSaid(t *testing.T) {
	testBed := newPendingMessageTestBed(t)
	require.NoError(t, testBed.repository.Enqueue(t.Context(), testBed.aRoundMessage(queuedAt, "Run 52")))

	require.NoError(t, persistence.NewStrategyBotRepository(testBed.database).Delete(t.Context(), testBed.botID))

	assert.Empty(t, testBed.storedMessages(t))
}

func TestTransactionRepositoryRollsBackEveryWriteWhenTheWorkFails(t *testing.T) {
	testBed := newPendingMessageTestBed(t)
	transactionRepository := persistence.NewTransactionRepository(testBed.database)
	workFailure := errors.New("the round could not be booked")

	atomicError := transactionRepository.Atomically(t.Context(), func(transactionContext context.Context) error {
		require.NoError(t, testBed.repository.Enqueue(transactionContext, testBed.aRoundMessage(queuedAt, "Run 52")))

		return workFailure
	})

	assert.ErrorIs(t, atomicError, workFailure)
	assert.Empty(t, testBed.storedMessages(t))
}

func TestTransactionRepositoryKeepsEveryWriteWhenTheWorkSucceeds(t *testing.T) {
	testBed := newPendingMessageTestBed(t)
	transactionRepository := persistence.NewTransactionRepository(testBed.database)

	atomicError := transactionRepository.Atomically(t.Context(), func(transactionContext context.Context) error {
		// A nested call joins the outer transaction instead of committing on its own.
		return transactionRepository.Atomically(transactionContext, func(innerContext context.Context) error {
			return testBed.repository.Enqueue(innerContext, testBed.aRoundMessage(queuedAt, "Run 52"))
		})
	})

	require.NoError(t, atomicError)
	assert.Equal(t, []string{"Run 52"}, textsOf(testBed.storedMessages(t)))
}

func TestTransactionRepositoryNestedWorkRollsBackWithTheOuterWork(t *testing.T) {
	testBed := newPendingMessageTestBed(t)
	transactionRepository := persistence.NewTransactionRepository(testBed.database)

	atomicError := transactionRepository.Atomically(t.Context(), func(transactionContext context.Context) error {
		require.NoError(t, transactionRepository.Atomically(transactionContext, func(innerContext context.Context) error {
			return testBed.repository.Enqueue(innerContext, testBed.aRoundMessage(queuedAt, "Run 52"))
		}))

		return errors.New("the outer work failed afterwards")
	})

	require.Error(t, atomicError)
	assert.Empty(t, testBed.storedMessages(t))
}

func TestOutboxAndDutyStorageSaySoWhenStorageCannotAnswer(t *testing.T) {
	database := closedDatabase(t)
	pendingMessageRepository := persistence.NewPendingMessageRepository(database)
	leaseRepository := persistence.NewJobLeadershipLeaseRepository(database)
	strategyBotRepository := persistence.NewStrategyBotRepository(database)

	assert.Error(t, pendingMessageRepository.Enqueue(t.Context(), entities.PendingMessage{}))
	_, findError := pendingMessageRepository.FindUnsettled(t.Context(), 10)
	assert.Error(t, findError)
	_, claimError := pendingMessageRepository.Claim(t.Context(), 1, "replica-a", queuedAt, queuedAt)
	assert.Error(t, claimError)
	assert.Error(t, pendingMessageRepository.MarkSent(t.Context(), 1, "replica-a", queuedAt))
	assert.Error(t, pendingMessageRepository.Reschedule(t.Context(), 1, "replica-a", 1, queuedAt))
	assert.Error(t, pendingMessageRepository.Abandon(t.Context(), 1, "replica-a", "expired", queuedAt))
	assert.Error(t, pendingMessageRepository.DeleteSettledBefore(t.Context(), queuedAt))
	_, acquireError := leaseRepository.Acquire(t.Context(), "background-jobs", "replica-a", queuedAt, queuedAt)
	assert.Error(t, acquireError)
	assert.Error(t, leaseRepository.Release(t.Context(), "background-jobs", "replica-a"))
	_, lockedError := strategyBotRepository.FindOneLocked(t.Context(), 1)
	assert.Error(t, lockedError)
	assert.Error(t, strategyBotRepository.ForgetSentSignal(t.Context(), 1, "buy"))
}

// A round is booked through three repositories; this proves all three roll back together, which the round's own tests cannot since they mock the transaction.
func TestTransactionRepositoryRollsBackARoundsStateHistoryAndMessageTogether(t *testing.T) {
	testBed := newPendingMessageTestBed(t)
	strategyBotRepository := persistence.NewStrategyBotRepository(testBed.database)
	strategyBotRunRecordRepository := persistence.NewStrategyBotRunRecordRepository(testBed.database)
	transactionRepository := persistence.NewTransactionRepository(testBed.database)

	atomicError := transactionRepository.Atomically(t.Context(), func(transactionContext context.Context) error {
		bot, findError := strategyBotRepository.FindOneLocked(transactionContext, testBed.botID)
		require.NoError(t, findError)
		bot.LastSentSignal = string(vo.SignalBuy)
		require.NoError(t, strategyBotRepository.UpdateRunState(transactionContext, bot))
		_, appendError := strategyBotRunRecordRepository.Append(transactionContext, dto.StrategyBotRunRecordWriteDto{
			StrategyBotID: testBed.botID, RanAt: queuedAt, Result: "buy"})
		require.NoError(t, appendError)
		require.NoError(t, testBed.repository.Enqueue(transactionContext, testBed.aRoundMessage(queuedAt, "Run 1")))

		return errors.New("the round could not be booked")
	})

	require.Error(t, atomicError)
	storedBot, findError := strategyBotRepository.FindOne(t.Context(), testBed.botID)
	require.NoError(t, findError)
	assert.Empty(t, storedBot.LastSentSignal)
	history, historyError := strategyBotRunRecordRepository.FindLatestByBot(t.Context(), testBed.botID)
	require.NoError(t, historyError)
	assert.Empty(t, history)
	assert.Empty(t, testBed.storedMessages(t))
}
