package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

// attemptedAt and the moments derived from it are written out so assertions state the requirement.
var attemptedAt = time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)

func aQueuedMessage(status vo.PendingMessageStatusVo, attemptCount int) entities.PendingMessage {
	return entities.PendingMessage{
		ID: 1, StrategyBotID: 3, RecipientUserID: 7, Kind: string(vo.PendingMessageRound),
		Signal: string(vo.SignalBuy), Text: "🔖 Run 52", Status: string(status), AttemptCount: attemptCount,
		NextAttemptAt: attemptedAt, ExpiresAt: time.Date(2026, 10, 2, 8, 5, 0, 0, time.UTC),
	}
}

func TestPendingMessageDomainIsTooLateFromItsDeadlineOn(t *testing.T) {
	testCases := []struct {
		name     string
		at       time.Time
		expected bool
	}{
		{name: "at the deadline itself it is too late", at: time.Date(2026, 10, 2, 8, 5, 0, 0, time.UTC), expected: true},
		{name: "a second before the deadline it still counts", at: time.Date(2026, 10, 2, 8, 4, 59, 0, time.UTC), expected: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			message := domains.NewPendingMessageDomain(aQueuedMessage(vo.PendingMessageReady, 0))

			assert.Equal(t, testCase.expected, message.IsExpiredAt(testCase.at))
		})
	}
}

func TestPendingMessageDomainAfterAnAttempt(t *testing.T) {
	testCases := []struct {
		name           string
		attemptCount   int
		deliveryResult vo.DeliveryResultVo
		expected       vo.PendingMessageAttemptOutcomeVo
	}{
		{
			name: "a delivered message is sent", deliveryResult: vo.DeliveryResultVo{},
			expected: vo.PendingMessageAttemptOutcomeVo{Kind: vo.PendingMessageAttemptSent},
		},
		{
			name: "the first failure waits two seconds", attemptCount: 0,
			deliveryResult: vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureUnreachable},
			expected: vo.PendingMessageAttemptOutcomeVo{Kind: vo.PendingMessageAttemptRetry, AttemptCount: 1,
				NextAttemptAt: time.Date(2026, 10, 2, 8, 0, 2, 0, time.UTC)},
		},
		{
			name: "the third failure waits eight seconds", attemptCount: 2,
			deliveryResult: vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureTimedOut},
			expected: vo.PendingMessageAttemptOutcomeVo{Kind: vo.PendingMessageAttemptRetry, AttemptCount: 3,
				NextAttemptAt: time.Date(2026, 10, 2, 8, 0, 8, 0, time.UTC)},
		},
		{
			name: "the wait never grows past five minutes", attemptCount: 20,
			deliveryResult: vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureUnreachable},
			expected: vo.PendingMessageAttemptOutcomeVo{Kind: vo.PendingMessageAttemptRetry, AttemptCount: 21,
				NextAttemptAt: time.Date(2026, 10, 2, 8, 5, 0, 0, time.UTC)},
		},
		{
			name: "Telegram asking for seven seconds is honoured over a shorter wait", attemptCount: 0,
			deliveryResult: vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureUnreachable, RetryAfter: 7 * time.Second},
			expected: vo.PendingMessageAttemptOutcomeVo{Kind: vo.PendingMessageAttemptRetry, AttemptCount: 1,
				NextAttemptAt: time.Date(2026, 10, 2, 8, 0, 7, 0, time.UTC)},
		},
		{
			name: "a longer wait of our own beats a shorter one Telegram asked for", attemptCount: 5,
			deliveryResult: vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureUnreachable, RetryAfter: 7 * time.Second},
			expected: vo.PendingMessageAttemptOutcomeVo{Kind: vo.PendingMessageAttemptRetry, AttemptCount: 6,
				NextAttemptAt: time.Date(2026, 10, 2, 8, 1, 4, 0, time.UTC)},
		},
		{
			name:           "a rejected token is refused and halts the bot",
			deliveryResult: vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureCredentialRejected},
			expected: vo.PendingMessageAttemptOutcomeVo{Kind: vo.PendingMessageAttemptRefused,
				HaltReason: vo.StrategyBotHaltCredentialRejected},
		},
		{
			name:           "an unknown chat is refused and halts the bot",
			deliveryResult: vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureDestinationNotFound},
			expected: vo.PendingMessageAttemptOutcomeVo{Kind: vo.PendingMessageAttemptRefused,
				HaltReason: vo.StrategyBotHaltDestinationNotFound},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			message := domains.NewPendingMessageDomain(aQueuedMessage(vo.PendingMessageSending, testCase.attemptCount))

			assert.Equal(t, testCase.expected, message.AfterAttempt(testCase.deliveryResult, attemptedAt))
		})
	}
}

func TestPendingMessageDomainARemovedDeliverySettingIsRefused(t *testing.T) {
	message := domains.NewPendingMessageDomain(aQueuedMessage(vo.PendingMessageSending, 0))

	assert.Equal(t, vo.PendingMessageAttemptOutcomeVo{
		Kind: vo.PendingMessageAttemptRefused, HaltReason: vo.StrategyBotHaltDeliveryNotConfigured,
	}, message.AfterMissingDeliverySetting())
}

func TestPendingMessageDomainOnlyARoundMessageMakesItsBotForget(t *testing.T) {
	roundMessage := aQueuedMessage(vo.PendingMessageReady, 0)
	lifecycleMessage := aQueuedMessage(vo.PendingMessageReady, 0)
	lifecycleMessage.Kind = string(vo.PendingMessageLifecycle)
	lifecycleMessage.Signal = ""

	assert.True(t, domains.NewPendingMessageDomain(roundMessage).ForgetsSignalWhenAbandoned())
	assert.False(t, domains.NewPendingMessageDomain(lifecycleMessage).ForgetsSignalWhenAbandoned())
}

func TestPendingMessageDomainWhenAMessageMayBeSent(t *testing.T) {
	oneSecondOn := time.Date(2026, 10, 2, 8, 0, 1, 0, time.UTC)
	testCases := []struct {
		name          string
		status        string
		nextAttemptAt time.Time
		claimedUntil  *time.Time
		expected      bool
	}{
		{name: "a ready message whose wait is over", status: string(vo.PendingMessageReady),
			nextAttemptAt: attemptedAt, expected: true},
		{name: "a ready message still waiting", status: string(vo.PendingMessageReady),
			nextAttemptAt: oneSecondOn, expected: false},
		{name: "a message another replica is still sending", status: string(vo.PendingMessageSending),
			claimedUntil: &oneSecondOn, expected: false},
		{name: "a message whose sender's claim ran out", status: string(vo.PendingMessageSending),
			claimedUntil: &attemptedAt, expected: true},
		{name: "a message already sent", status: string(vo.PendingMessageSent), expected: false},
		{name: "a damaged status is read as ready", status: "garbled", nextAttemptAt: attemptedAt, expected: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			stored := aQueuedMessage(vo.PendingMessageReady, 0)
			stored.Status = testCase.status
			stored.NextAttemptAt = testCase.nextAttemptAt
			stored.ClaimedUntil = testCase.claimedUntil

			assert.Equal(t, testCase.expected, domains.NewPendingMessageDomain(stored).IsDispatchableAt(attemptedAt))
		})
	}
}

func TestNewRoundPendingMessageDomainGivesUpAfterTheIntervalButNeverWithinFiveMinutes(t *testing.T) {
	testCases := []struct {
		name              string
		triggerInterval   time.Duration
		expectedExpiresAt time.Time
	}{
		{name: "a one-minute bot still gets five minutes", triggerInterval: time.Minute,
			expectedExpiresAt: time.Date(2026, 10, 2, 8, 5, 0, 0, time.UTC)},
		{name: "a fifteen-minute bot gets its own interval", triggerInterval: 15 * time.Minute,
			expectedExpiresAt: time.Date(2026, 10, 2, 8, 15, 0, 0, time.UTC)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			queued := domains.NewRoundPendingMessageDomain(
				3, 7, attemptedAt.Add(-time.Minute), vo.SignalBuy, "🔖 Run 52", testCase.triggerInterval, attemptedAt).
				ToEntity()

			assert.Equal(t, testCase.expectedExpiresAt, queued.ExpiresAt)
			assert.Equal(t, string(vo.PendingMessageRound), queued.Kind)
			assert.Equal(t, string(vo.PendingMessageReady), queued.Status)
			assert.Equal(t, attemptedAt, queued.NextAttemptAt)
		})
	}
}

func TestNewLifecyclePendingMessageDomainGivesUpAfterAnHour(t *testing.T) {
	queued := domains.NewLifecyclePendingMessageDomain(3, 7, "已停止", attemptedAt).ToEntity()

	assert.Equal(t, time.Date(2026, 10, 2, 9, 0, 0, 0, time.UTC), queued.ExpiresAt)
	assert.Equal(t, string(vo.PendingMessageLifecycle), queued.Kind)
	assert.Nil(t, queued.RoundDueAt)
}

func TestPendingMessageQueueDomainSendsEachPersonsOldestMessageFirst(t *testing.T) {
	testCases := []struct {
		name        string
		messages    []entities.PendingMessage
		expectedIDs []uint
	}{
		{
			name: "one head per person, people never wait on each other",
			messages: []entities.PendingMessage{
				{ID: 1, RecipientUserID: 1, Status: string(vo.PendingMessageReady), NextAttemptAt: attemptedAt},
				{ID: 2, RecipientUserID: 1, Status: string(vo.PendingMessageReady), NextAttemptAt: attemptedAt},
				{ID: 3, RecipientUserID: 2, Status: string(vo.PendingMessageReady), NextAttemptAt: attemptedAt},
			},
			expectedIDs: []uint{1, 3},
		},
		{
			name: "a later message waits while the earlier one is still waiting to be retried",
			messages: []entities.PendingMessage{
				{ID: 1, RecipientUserID: 1, Status: string(vo.PendingMessageReady),
					NextAttemptAt: attemptedAt.Add(time.Minute)},
				{ID: 2, RecipientUserID: 1, Status: string(vo.PendingMessageReady), NextAttemptAt: attemptedAt},
			},
			expectedIDs: []uint{},
		},
		{name: "an empty queue has nothing to send", messages: nil, expectedIDs: []uint{}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			heads := domains.NewPendingMessageQueueDomain(testCase.messages).HeadsDispatchableAt(attemptedAt)

			headIDs := []uint{}
			for _, head := range heads {
				headIDs = append(headIDs, head.ID())
			}
			assert.Equal(t, testCase.expectedIDs, headIDs)
		})
	}
}
