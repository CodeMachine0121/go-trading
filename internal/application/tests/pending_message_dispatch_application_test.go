package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var (
	dispatchNow          = time.Date(2026, 10, 2, 8, 0, 0, 0, time.UTC)
	dispatchClaimedUntil = time.Date(2026, 10, 2, 8, 2, 0, 0, time.UTC)
	sevenDaysBefore      = time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
)

const (
	queuedMessageID  = uint(41)
	queuedBotID      = uint(3)
	queuedRecipient  = uint(7)
	queuedRoundText  = "🟢【買入】早盤突破 · BTCUSDT\n\n🔖 Run 52"
	dispatchReplica  = "replica-a"
	dispatchStopText = "⏹️【已停止】早盤突破 · BTCUSDT"
)

type pendingMessageDispatchUnderTest struct {
	dispatchApplication        *application.PendingMessageDispatchApplication
	pendingMessageRepository   *mocks.MockIPendingMessageRepository
	strategyBotRepository      *mocks.MockIStrategyBotRepository
	telegramDeliveryRepository *mocks.MockITelegramDeliveryRepository
	messageDeliveryProxy       *mocks.MockIMessageDeliveryProxy
	// trimmedBefore lists the cutoff of every trim of settled messages.
	trimmedBefore *[]time.Time
}

func newPendingMessageDispatchUnderTest(t *testing.T) pendingMessageDispatchUnderTest {
	controller := gomock.NewController(t)
	pendingMessageRepository := mocks.NewMockIPendingMessageRepository(controller)
	strategyBotRepository := mocks.NewMockIStrategyBotRepository(controller)
	transactionRepository := mocks.NewMockITransactionRepository(controller)
	transactionRepository.EXPECT().Atomically(gomock.Any(), gomock.Any()).
		DoAndReturn(func(executionContext context.Context, work func(context.Context) error) error {
			return work(executionContext)
		}).AnyTimes()
	telegramDeliveryRepository := mocks.NewMockITelegramDeliveryRepository(controller)
	secretSealProxy := mocks.NewMockISecretSealProxy(controller)
	secretSealProxy.EXPECT().Unseal("sealed").Return("the-token", nil).AnyTimes()
	messageDeliveryProxy := mocks.NewMockIMessageDeliveryProxy(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(dispatchNow).AnyTimes()
	trimmedBefore := []time.Time{}
	pendingMessageRepository.EXPECT().DeleteSettledBefore(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, cutoff time.Time) error {
			trimmedBefore = append(trimmedBefore, cutoff)

			return nil
		}).AnyTimes()

	return pendingMessageDispatchUnderTest{
		dispatchApplication: application.NewPendingMessageDispatchApplication(service.NewPendingMessageService(
			pendingMessageRepository, strategyBotRepository, transactionRepository,
			service.NewTelegramDeliveryService(telegramDeliveryRepository, secretSealProxy, messageDeliveryProxy),
			clockProxy, dispatchReplica, 2*time.Minute, 8)),
		pendingMessageRepository:   pendingMessageRepository,
		strategyBotRepository:      strategyBotRepository,
		telegramDeliveryRepository: telegramDeliveryRepository,
		messageDeliveryProxy:       messageDeliveryProxy,
		trimmedBefore:              &trimmedBefore,
	}
}

func aDueRoundMessage() entities.PendingMessage {
	return entities.PendingMessage{
		ID: queuedMessageID, StrategyBotID: queuedBotID, RecipientUserID: queuedRecipient,
		Kind: string(vo.PendingMessageRound), Signal: string(vo.SignalBuy), Text: queuedRoundText,
		Status: string(vo.PendingMessageReady), NextAttemptAt: dispatchNow,
		ExpiresAt: dispatchNow.Add(5 * time.Minute),
	}
}

func (underTest pendingMessageDispatchUnderTest) queueHolds(messages ...entities.PendingMessage) {
	underTest.pendingMessageRepository.EXPECT().FindUnsettled(gomock.Any(), gomock.Any()).Return(messages, nil)
}

func (underTest pendingMessageDispatchUnderTest) claimSucceeds() {
	underTest.pendingMessageRepository.EXPECT().
		Claim(gomock.Any(), queuedMessageID, dispatchReplica, dispatchNow, dispatchClaimedUntil).Return(true, nil)
}

func (underTest pendingMessageDispatchUnderTest) recipientHasADeliverySetting() {
	underTest.telegramDeliveryRepository.EXPECT().FindOneByUser(gomock.Any(), queuedRecipient).
		Return(entities.TelegramDelivery{UserID: queuedRecipient, SealedBotToken: "sealed", ChatID: "987654"}, nil).
		AnyTimes()
}

func (underTest pendingMessageDispatchUnderTest) telegramAnswers(deliveryResult vo.DeliveryResultVo) {
	underTest.messageDeliveryProxy.EXPECT().Deliver(gomock.Any(), gomock.Any(), queuedRoundText).
		Return(deliveryResult, nil)
}

func aBotThatIs(runState vo.StrategyBotRunStateVo) entities.StrategyBot {
	return entities.StrategyBot{ID: queuedBotID, OwnerID: queuedRecipient, RunState: string(runState),
		LastSentSignal: string(vo.SignalBuy), TriggerIntervalMinutes: 5}
}

func TestPendingMessageDispatchSendsAQueuedMessageAndMarksItSent(t *testing.T) {
	underTest := newPendingMessageDispatchUnderTest(t)
	underTest.queueHolds(aDueRoundMessage())
	underTest.claimSucceeds()
	underTest.recipientHasADeliverySetting()
	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), vo.MessageDeliveryCredentialVo{BotToken: "the-token", ChatID: "987654"}, queuedRoundText).
		Return(vo.DeliveryResultVo{}, nil)
	underTest.pendingMessageRepository.EXPECT().MarkSent(gomock.Any(), queuedMessageID, dispatchReplica, dispatchNow).
		Return(nil)

	deliveredCount, dispatchError := underTest.dispatchApplication.DispatchPendingMessages(t.Context())

	require.NoError(t, dispatchError)
	assert.Equal(t, 1, deliveredCount)
}

func TestPendingMessageDispatchLeavesAMessageAnotherReplicaTook(t *testing.T) {
	underTest := newPendingMessageDispatchUnderTest(t)
	underTest.queueHolds(aDueRoundMessage())
	underTest.pendingMessageRepository.EXPECT().Claim(gomock.Any(), queuedMessageID, gomock.Any(), gomock.Any(), gomock.Any()).
		Return(false, nil)
	// No Deliver expectation: the mock fails the test if Telegram is asked.

	deliveredCount, dispatchError := underTest.dispatchApplication.DispatchPendingMessages(t.Context())

	require.NoError(t, dispatchError)
	assert.Equal(t, 0, deliveredCount)
}

func TestPendingMessageDispatchPutsATemporaryFailureBackToWait(t *testing.T) {
	testCases := []struct {
		name                  string
		deliveryResult        vo.DeliveryResultVo
		expectedNextAttemptAt time.Time
	}{
		{name: "Telegram could not be reached", deliveryResult: vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureUnreachable},
			expectedNextAttemptAt: time.Date(2026, 10, 2, 8, 0, 2, 0, time.UTC)},
		{name: "Telegram asked to wait seven seconds",
			deliveryResult:        vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureUnreachable, RetryAfter: 7 * time.Second},
			expectedNextAttemptAt: time.Date(2026, 10, 2, 8, 0, 7, 0, time.UTC)},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newPendingMessageDispatchUnderTest(t)
			underTest.queueHolds(aDueRoundMessage())
			underTest.claimSucceeds()
			underTest.recipientHasADeliverySetting()
			underTest.telegramAnswers(testCase.deliveryResult)
			underTest.pendingMessageRepository.EXPECT().
				Reschedule(gomock.Any(), queuedMessageID, dispatchReplica, 1, testCase.expectedNextAttemptAt).Return(nil)

			_, dispatchError := underTest.dispatchApplication.DispatchPendingMessages(t.Context())

			require.NoError(t, dispatchError)
		})
	}
}

func TestPendingMessageDispatchRetriesWhenThisSideFails(t *testing.T) {
	underTest := newPendingMessageDispatchUnderTest(t)
	underTest.queueHolds(aDueRoundMessage())
	underTest.claimSucceeds()
	underTest.recipientHasADeliverySetting()
	underTest.messageDeliveryProxy.EXPECT().Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		Return(vo.DeliveryResultVo{}, errors.New("the request could not be built"))
	underTest.pendingMessageRepository.EXPECT().
		Reschedule(gomock.Any(), queuedMessageID, dispatchReplica, 1, time.Date(2026, 10, 2, 8, 0, 2, 0, time.UTC)).
		Return(nil)

	_, dispatchError := underTest.dispatchApplication.DispatchPendingMessages(t.Context())

	require.NoError(t, dispatchError)
}

func TestPendingMessageDispatchHaltsTheBotOnARefusalThatWillNotFixItself(t *testing.T) {
	testCases := []struct {
		name               string
		deliveryResult     vo.DeliveryResultVo
		deliverySetting    bool
		expectedHaltReason vo.StrategyBotHaltReasonVo
	}{
		{name: "the token is rejected", deliverySetting: true,
			deliveryResult:     vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureCredentialRejected},
			expectedHaltReason: vo.StrategyBotHaltCredentialRejected},
		{name: "the chat is unknown", deliverySetting: true,
			deliveryResult:     vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureDestinationNotFound},
			expectedHaltReason: vo.StrategyBotHaltDestinationNotFound},
		{name: "the owner removed the delivery setting", deliverySetting: false,
			expectedHaltReason: vo.StrategyBotHaltDeliveryNotConfigured},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newPendingMessageDispatchUnderTest(t)
			underTest.queueHolds(aDueRoundMessage())
			underTest.claimSucceeds()
			if testCase.deliverySetting {
				underTest.recipientHasADeliverySetting()
				underTest.telegramAnswers(testCase.deliveryResult)
			} else {
				underTest.telegramDeliveryRepository.EXPECT().FindOneByUser(gomock.Any(), queuedRecipient).
					Return(entities.TelegramDelivery{}, domains.ErrTelegramDeliveryNotConfigured)
			}
			underTest.strategyBotRepository.EXPECT().FindOneLocked(gomock.Any(), queuedBotID).
				Return(aBotThatIs(vo.StrategyBotRunning), nil)
			underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, halted entities.StrategyBot) error {
					assert.Equal(t, string(vo.StrategyBotStopped), halted.RunState)
					assert.Equal(t, string(testCase.expectedHaltReason), halted.HaltReason)

					return nil
				})
			underTest.pendingMessageRepository.EXPECT().
				Abandon(gomock.Any(), queuedMessageID, dispatchReplica, string(testCase.expectedHaltReason), dispatchNow).
				Return(nil)

			deliveredCount, dispatchError := underTest.dispatchApplication.DispatchPendingMessages(t.Context())

			require.NoError(t, dispatchError)
			assert.Equal(t, 0, deliveredCount)
		})
	}
}

func TestPendingMessageDispatchOnlyAbandonsWhenTheBotIsAlreadyStopped(t *testing.T) {
	underTest := newPendingMessageDispatchUnderTest(t)
	underTest.queueHolds(aDueRoundMessage())
	underTest.claimSucceeds()
	underTest.recipientHasADeliverySetting()
	underTest.telegramAnswers(vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureCredentialRejected})
	underTest.strategyBotRepository.EXPECT().FindOneLocked(gomock.Any(), queuedBotID).
		Return(aBotThatIs(vo.StrategyBotStopped), nil)
	// No UpdateRunState expectation: a stopped bot is left as it is.
	underTest.pendingMessageRepository.EXPECT().
		Abandon(gomock.Any(), queuedMessageID, dispatchReplica, string(vo.StrategyBotHaltCredentialRejected), dispatchNow).
		Return(nil)

	_, dispatchError := underTest.dispatchApplication.DispatchPendingMessages(t.Context())

	require.NoError(t, dispatchError)
}

func TestPendingMessageDispatchLeavesTheMessageWhenTheBotCannotBeHalted(t *testing.T) {
	underTest := newPendingMessageDispatchUnderTest(t)
	underTest.queueHolds(aDueRoundMessage())
	underTest.claimSucceeds()
	underTest.recipientHasADeliverySetting()
	underTest.telegramAnswers(vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureCredentialRejected})
	underTest.strategyBotRepository.EXPECT().FindOneLocked(gomock.Any(), queuedBotID).
		Return(entities.StrategyBot{}, errors.New("the database went away"))
	// No Abandon expectation: the message stays taken and is tried again once the claim runs out.

	_, dispatchError := underTest.dispatchApplication.DispatchPendingMessages(t.Context())

	require.NoError(t, dispatchError)
}

func TestPendingMessageDispatchGivesUpAMessageThatCameTooLate(t *testing.T) {
	testCases := []struct {
		name          string
		kind          vo.PendingMessageKindVo
		signal        string
		expectsForget bool
	}{
		{name: "a round's message makes its bot forget the signal", kind: vo.PendingMessageRound,
			signal: string(vo.SignalBuy), expectsForget: true},
		{name: "a bot's message about itself forgets nothing", kind: vo.PendingMessageLifecycle,
			expectsForget: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newPendingMessageDispatchUnderTest(t)
			expired := aDueRoundMessage()
			expired.Kind = string(testCase.kind)
			expired.Signal = testCase.signal
			expired.ExpiresAt = dispatchNow
			underTest.queueHolds(expired)
			underTest.claimSucceeds()
			forgetCall := (*gomock.Call)(nil)
			if testCase.expectsForget {
				forgetCall = underTest.strategyBotRepository.EXPECT().
					ForgetSentSignal(gomock.Any(), queuedBotID, string(vo.SignalBuy)).Return(nil)
			}
			abandonCall := underTest.pendingMessageRepository.EXPECT().
				Abandon(gomock.Any(), queuedMessageID, dispatchReplica, "expired", dispatchNow).Return(nil)
			if forgetCall != nil {
				abandonCall.After(forgetCall)
			}
			// No Deliver expectation: a message too late is never sent.

			_, dispatchError := underTest.dispatchApplication.DispatchPendingMessages(t.Context())

			require.NoError(t, dispatchError)
		})
	}
}

func TestPendingMessageDispatchSendsEachPersonsMessagesInOrder(t *testing.T) {
	underTest := newPendingMessageDispatchUnderTest(t)
	laterForSamePerson := aDueRoundMessage()
	laterForSamePerson.ID = queuedMessageID + 1
	laterForSamePerson.Kind = string(vo.PendingMessageLifecycle)
	laterForSamePerson.Text = dispatchStopText
	underTest.queueHolds(aDueRoundMessage(), laterForSamePerson)
	underTest.claimSucceeds()
	underTest.recipientHasADeliverySetting()
	underTest.telegramAnswers(vo.DeliveryResultVo{})
	underTest.pendingMessageRepository.EXPECT().MarkSent(gomock.Any(), queuedMessageID, gomock.Any(), gomock.Any()).
		Return(nil)
	// No expectation for the later message: it waits for a later look at the queue.

	deliveredCount, dispatchError := underTest.dispatchApplication.DispatchPendingMessages(t.Context())

	require.NoError(t, dispatchError)
	assert.Equal(t, 1, deliveredCount)
}

func TestPendingMessageDispatchReportsAQueueItCannotRead(t *testing.T) {
	underTest := newPendingMessageDispatchUnderTest(t)
	underTest.pendingMessageRepository.EXPECT().FindUnsettled(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("the database went away"))

	_, dispatchError := underTest.dispatchApplication.DispatchPendingMessages(t.Context())

	assert.Error(t, dispatchError)
}

func TestPendingMessageDispatchCountsAMessageItSentButCouldNotMark(t *testing.T) {
	underTest := newPendingMessageDispatchUnderTest(t)
	underTest.queueHolds(aDueRoundMessage())
	underTest.claimSucceeds()
	underTest.recipientHasADeliverySetting()
	underTest.telegramAnswers(vo.DeliveryResultVo{})
	underTest.pendingMessageRepository.EXPECT().MarkSent(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(errors.New("the database went away"))

	deliveredCount, dispatchError := underTest.dispatchApplication.DispatchPendingMessages(t.Context())

	require.NoError(t, dispatchError)
	assert.Equal(t, 1, deliveredCount)
}

func TestPendingMessageDispatchDropsWhatSettledAWeekAgoOnEveryLook(t *testing.T) {
	underTest := newPendingMessageDispatchUnderTest(t)
	underTest.queueHolds()

	_, dispatchError := underTest.dispatchApplication.DispatchPendingMessages(t.Context())

	require.NoError(t, dispatchError)
	assert.Equal(t, []time.Time{sevenDaysBefore}, *underTest.trimmedBefore)
}

var dispatchStorageFailure = errors.New("the database went away")

func TestPendingMessageDispatchReportsATrimItCannotDo(t *testing.T) {
	controller := gomock.NewController(t)
	pendingMessageRepository := mocks.NewMockIPendingMessageRepository(controller)
	pendingMessageRepository.EXPECT().DeleteSettledBefore(gomock.Any(), gomock.Any()).Return(dispatchStorageFailure)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(dispatchNow).AnyTimes()
	dispatchApplication := application.NewPendingMessageDispatchApplication(service.NewPendingMessageService(
		pendingMessageRepository, mocks.NewMockIStrategyBotRepository(controller),
		mocks.NewMockITransactionRepository(controller), nil, clockProxy, dispatchReplica, 2*time.Minute, 8))

	_, dispatchError := dispatchApplication.DispatchPendingMessages(t.Context())

	assert.ErrorIs(t, dispatchError, dispatchStorageFailure)
}

func TestPendingMessageDispatchLeavesAMessageItCouldNotTake(t *testing.T) {
	underTest := newPendingMessageDispatchUnderTest(t)
	underTest.queueHolds(aDueRoundMessage())
	underTest.pendingMessageRepository.EXPECT().Claim(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(false, dispatchStorageFailure)
	// No Deliver expectation: a message not taken is never sent.

	deliveredCount, dispatchError := underTest.dispatchApplication.DispatchPendingMessages(t.Context())

	require.NoError(t, dispatchError)
	assert.Equal(t, 0, deliveredCount)
}

func TestPendingMessageDispatchKeepsAnExpiredMessageWhileItsBotCannotForget(t *testing.T) {
	underTest := newPendingMessageDispatchUnderTest(t)
	expired := aDueRoundMessage()
	expired.ExpiresAt = dispatchNow
	underTest.queueHolds(expired)
	underTest.claimSucceeds()
	underTest.strategyBotRepository.EXPECT().ForgetSentSignal(gomock.Any(), queuedBotID, string(vo.SignalBuy)).
		Return(dispatchStorageFailure)
	// No Abandon expectation: the message stays taken, so it is given up only once its bot has forgotten.

	_, dispatchError := underTest.dispatchApplication.DispatchPendingMessages(t.Context())

	require.NoError(t, dispatchError)
}

func TestPendingMessageDispatchCarriesOnWhenAWriteAfterSendingFails(t *testing.T) {
	testCases := []struct {
		name    string
		arrange func(underTest pendingMessageDispatchUnderTest)
	}{
		{
			name: "an expired message cannot be marked given up",
			arrange: func(underTest pendingMessageDispatchUnderTest) {
				expired := aDueRoundMessage()
				expired.Kind = string(vo.PendingMessageLifecycle)
				expired.ExpiresAt = dispatchNow
				underTest.queueHolds(expired)
				underTest.claimSucceeds()
				underTest.pendingMessageRepository.EXPECT().
					Abandon(gomock.Any(), queuedMessageID, dispatchReplica, "expired", dispatchNow).
					Return(dispatchStorageFailure)
			},
		},
		{
			name: "a refused message cannot be marked given up",
			arrange: func(underTest pendingMessageDispatchUnderTest) {
				underTest.queueHolds(aDueRoundMessage())
				underTest.claimSucceeds()
				underTest.recipientHasADeliverySetting()
				underTest.telegramAnswers(vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureCredentialRejected})
				underTest.strategyBotRepository.EXPECT().FindOneLocked(gomock.Any(), queuedBotID).
					Return(aBotThatIs(vo.StrategyBotStopped), nil)
				underTest.pendingMessageRepository.EXPECT().
					Abandon(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(dispatchStorageFailure)
			},
		},
		{
			name: "a failed message cannot be put back to wait",
			arrange: func(underTest pendingMessageDispatchUnderTest) {
				underTest.queueHolds(aDueRoundMessage())
				underTest.claimSucceeds()
				underTest.recipientHasADeliverySetting()
				underTest.telegramAnswers(vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureUnreachable})
				underTest.pendingMessageRepository.EXPECT().
					Reschedule(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(dispatchStorageFailure)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newPendingMessageDispatchUnderTest(t)
			testCase.arrange(underTest)

			deliveredCount, dispatchError := underTest.dispatchApplication.DispatchPendingMessages(t.Context())

			require.NoError(t, dispatchError)
			assert.Equal(t, 0, deliveredCount)
		})
	}
}

func TestPendingMessageDispatchGivesUpARefusedMessageForABotThatIsGone(t *testing.T) {
	underTest := newPendingMessageDispatchUnderTest(t)
	underTest.queueHolds(aDueRoundMessage())
	underTest.claimSucceeds()
	underTest.recipientHasADeliverySetting()
	underTest.telegramAnswers(vo.DeliveryResultVo{FailureReason: vo.DeliveryFailureDestinationNotFound})
	underTest.strategyBotRepository.EXPECT().FindOneLocked(gomock.Any(), queuedBotID).
		Return(entities.StrategyBot{}, domains.StrategyBotNotFound(queuedBotID))
	underTest.pendingMessageRepository.EXPECT().
		Abandon(gomock.Any(), queuedMessageID, dispatchReplica, string(vo.StrategyBotHaltDestinationNotFound), dispatchNow).
		Return(nil)

	_, dispatchError := underTest.dispatchApplication.DispatchPendingMessages(t.Context())

	require.NoError(t, dispatchError)
}
