package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// strategyBotOwnerID is whoever these tests act as, and strategyBotStrangerID is
// somebody else — the two identities every ownership rule here is about.
const (
	strategyBotOwnerID    = uint(1)
	strategyBotStrangerID = uint(2)
	strategyBotID         = uint(3)
	botsTradingStrategyID = uint(9)
)

type strategyBotApplicationUnderTest struct {
	strategyBotApplication         *application.StrategyBotApplication
	strategyBotRepository          *mocks.MockIStrategyBotRepository
	strategyBotRunRecordRepository *mocks.MockIStrategyBotRunRecordRepository
	messageDeliveryProxy           *mocks.MockIMessageDeliveryProxy
	announcements                  *[]string
	tradingStrategyRepository      *mocks.MockITradingStrategyRepository
	telegramDeliveryRepository     *mocks.MockITelegramDeliveryRepository
	clockProxy                     *mocks.MockIClockProxy
}

// newStrategyBotApplicationUnderTest wires the real domain services and the real
// models, mocking only the outermost boundaries: storage, the seal, the carrier and
// the clock. Every rule about bots, sources and conditions is therefore exercised
// through this, not stubbed out behind it.
func newStrategyBotApplicationUnderTest(t *testing.T) strategyBotApplicationUnderTest {
	controller := gomock.NewController(t)

	strategyBotRepository := mocks.NewMockIStrategyBotRepository(controller)
	// 歷史是每一輪都會寫的，而它寫不寫得成不是這幾個測試在問的事。
	strategyBotRunRecordRepository := mocks.NewMockIStrategyBotRunRecordRepository(controller)
	strategyBotRunRecordRepository.EXPECT().
		Append(gomock.Any(), gomock.Any()).Return(nil).AnyTimes()
	tradingStrategyRepository := mocks.NewMockITradingStrategyRepository(controller)
	telegramDeliveryRepository := mocks.NewMockITelegramDeliveryRepository(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)

	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)).AnyTimes()

	// 啟動與停止現在會讓機器人說一句它自己的動靜。送不送得出去不是這幾個測試在問的事，
	// 所以整條路一律放行。
	secretSealProxy := mocks.NewMockISecretSealProxy(controller)
	secretSealProxy.EXPECT().Unseal(gomock.Any()).Return("the-token", nil).AnyTimes()
	announcements := &[]string{}
	messageDeliveryProxy := mocks.NewMockIMessageDeliveryProxy(controller)
	messageDeliveryProxy.EXPECT().Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ vo.MessageDeliveryCredentialVo, message string,
		) (vo.DeliveryFailureReasonVo, error) {
			*announcements = append(*announcements, message)

			return vo.DeliveryFailureNone, nil
		}).AnyTimes()

	return strategyBotApplicationUnderTest{
		strategyBotApplication: application.NewStrategyBotApplication(
			service.NewStrategyBotService(
				strategyBotRepository, strategyBotRunRecordRepository, clockProxy),
			service.NewTradingStrategyService(tradingStrategyRepository),
			service.NewTelegramDeliveryService(
				telegramDeliveryRepository, secretSealProxy, messageDeliveryProxy),
		),
		strategyBotRepository:          strategyBotRepository,
		strategyBotRunRecordRepository: strategyBotRunRecordRepository,
		messageDeliveryProxy:           messageDeliveryProxy,
		announcements:                  announcements,
		tradingStrategyRepository:      tradingStrategyRepository,
		telegramDeliveryRepository:     telegramDeliveryRepository,
		clockProxy:                     clockProxy,
	}
}

// expectTheNamedTradingStrategyIsThisPersons is the one question saving a bot asks
// of anything outside itself: may this person use the rules they named?
func (underTest strategyBotApplicationUnderTest) expectTheNamedTradingStrategyIsThisPersons() {
	underTest.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), botsTradingStrategyID).
		Return(entities.TradingStrategy{
			ID: botsTradingStrategyID, OwnerID: strategyBotOwnerID, Name: "黃金交叉",
		}, nil)
}

// announced is every message this bot said about itself so far.
func (underTest strategyBotApplicationUnderTest) announced() []string {
	return *underTest.announcements
}

func aBotWrite() dto.StrategyBotWriteDto {
	return dto.StrategyBotWriteDto{
		Name:                   "早盤突破",
		Symbol:                 "BTCUSDT",
		TradingStrategyID:      botsTradingStrategyID,
		TriggerIntervalMinutes: 5,
	}
}

// storedBot is a bot as it comes back out of storage. It carries no rules of its
// own: it names a set, and the set is a thing of its own.
func storedBot(runState vo.StrategyBotRunStateVo) entities.StrategyBot {
	return entities.StrategyBot{
		ID: strategyBotID, OwnerID: strategyBotOwnerID, Name: "早盤突破", Symbol: "BTCUSDT",
		TradingStrategyID:      botsTradingStrategyID,
		TradingStrategy:        entities.TradingStrategy{ID: botsTradingStrategyID, Name: "黃金交叉"},
		TriggerIntervalMinutes: 5,
		RunState:               string(runState),
	}
}

func TestStrategyBotApplicationCreateChecksTheNamedTradingStrategyIsThisPersons(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.expectTheNamedTradingStrategyIsThisPersons()
	underTest.strategyBotRepository.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) (entities.StrategyBot, error) {
			// The owner is taken from whoever is signed in, never from the body.
			assert.Equal(t, strategyBotOwnerID, bot.OwnerID)
			// A saved bot is stopped. Saving one cannot start it.
			assert.Equal(t, string(vo.StrategyBotStopped), bot.RunState)
			// The rules are named, never copied: a bot carries one identifier.
			assert.Equal(t, botsTradingStrategyID, bot.TradingStrategyID)

			return storedBot(vo.StrategyBotStopped), nil
		})

	botDto, createError := underTest.strategyBotApplication.CreateStrategyBot(
		context.Background(), strategyBotOwnerID, aBotWrite())

	require.NoError(t, createError)
	assert.Equal(t, strategyBotID, botDto.ID)
	assert.Equal(t, string(vo.StrategyBotStopped), botDto.RunState)
}

func TestStrategyBotApplicationCreateRefusesATradingStrategyThisPersonCannotSee(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), botsTradingStrategyID).
		Return(entities.TradingStrategy{
			ID: botsTradingStrategyID, OwnerID: strategyBotStrangerID, Name: "別人的",
		}, nil)

	_, createError := underTest.strategyBotApplication.CreateStrategyBot(
		context.Background(), strategyBotOwnerID, aBotWrite())

	// The same sentence as naming one that does not exist, which is what stops the
	// field becoming a way to probe for other people's trading strategies.
	require.ErrorIs(t, createError, domains.ErrTradingStrategyNotFound)
}

func TestStrategyBotApplicationReadsAndListsOnlyThisPersonsBots(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(storedBot(vo.StrategyBotStopped), nil).Times(2)

	botDto, getError := underTest.strategyBotApplication.GetStrategyBot(
		context.Background(), strategyBotOwnerID, strategyBotID)
	require.NoError(t, getError)
	assert.Equal(t, "早盤突破", botDto.Name)

	_, strangerError := underTest.strategyBotApplication.GetStrategyBot(
		context.Background(), strategyBotStrangerID, strategyBotID)

	// A stranger is owed the same sentence as a bot that is not there.
	require.ErrorIs(t, strangerError, domains.ErrStrategyBotNotFound)
}

func TestStrategyBotApplicationListHandsBackAnEmptyListRatherThanARefusal(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.strategyBotRepository.EXPECT().
		FindAllByOwner(gomock.Any(), strategyBotOwnerID).
		Return([]entities.StrategyBot{}, nil)

	botDtos, listError := underTest.strategyBotApplication.ListStrategyBots(
		context.Background(), strategyBotOwnerID)

	require.NoError(t, listError)
	assert.Empty(t, botDtos)
}

func TestStrategyBotApplicationUpdateRefusesWhileTheBotIsRunning(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.expectTheNamedTradingStrategyIsThisPersons()
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(storedBot(vo.StrategyBotRunning), nil)

	writeDto := aBotWrite()
	writeDto.ID = strategyBotID

	_, updateError := underTest.strategyBotApplication.UpdateStrategyBot(
		context.Background(), strategyBotOwnerID, writeDto)

	// Nobody could say which version a round in flight used, so the answer is to
	// stop it first rather than to guess.
	require.ErrorIs(t, updateError, domains.ErrStrategyBotRunning)
}

func TestStrategyBotApplicationDeleteDoesNotAskForAStopFirst(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(storedBot(vo.StrategyBotRunning), nil)
	underTest.strategyBotRepository.EXPECT().Delete(gomock.Any(), strategyBotID).Return(nil)

	assert.NoError(t, underTest.strategyBotApplication.DeleteStrategyBot(
		context.Background(), strategyBotOwnerID, strategyBotID))
}

func TestStrategyBotApplicationStartRefusesWithNowhereToBeSpokenTo(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.telegramDeliveryRepository.EXPECT().
		FindOneByUser(gomock.Any(), strategyBotOwnerID).
		Return(entities.TelegramDelivery{}, domains.ErrTelegramDeliveryNotConfigured)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(storedBot(vo.StrategyBotStopped), nil)
	underTest.strategyBotRepository.EXPECT().
		CountRunningByOwner(gomock.Any(), strategyBotOwnerID).Return(0, nil)

	_, startError := underTest.strategyBotApplication.StartStrategyBot(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.ErrorIs(t, startError, domains.ErrStrategyBotDeliveryNotConfigured)
}

func TestStrategyBotApplicationStartRefusesAtTheRunningLimitWithoutStoppingAnything(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.telegramDeliveryRepository.EXPECT().
		FindOneByUser(gomock.Any(), strategyBotOwnerID).
		Return(entities.TelegramDelivery{UserID: strategyBotOwnerID}, nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(storedBot(vo.StrategyBotStopped), nil)
	underTest.strategyBotRepository.EXPECT().
		CountRunningByOwner(gomock.Any(), strategyBotOwnerID).Return(10, nil)

	_, startError := underTest.strategyBotApplication.StartStrategyBot(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.ErrorIs(t, startError, domains.ErrStrategyBotRunningLimitReached)
}

func TestStrategyBotApplicationStartPutsTheBotToWorkDueImmediately(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	stopped := storedBot(vo.StrategyBotStopped)
	stopped.LastSentSignal = string(vo.SignalBuy)
	stopped.HaltReason = string(vo.StrategyBotHaltScriptFailed)

	underTest.telegramDeliveryRepository.EXPECT().
		FindOneByUser(gomock.Any(), strategyBotOwnerID).
		Return(entities.TelegramDelivery{UserID: strategyBotOwnerID}, nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(stopped, nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().
		CountRunningByOwner(gomock.Any(), strategyBotOwnerID).Return(0, nil)
	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) error {
			assert.Equal(t, string(vo.StrategyBotRunning), bot.RunState)
			assert.Equal(t, time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC), bot.NextRunAt)
			assert.Empty(t, bot.LastSentSignal)
			assert.Empty(t, bot.HaltReason)

			return nil
		})

	botDto, startError := underTest.strategyBotApplication.StartStrategyBot(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.NoError(t, startError)
	assert.Equal(t, string(vo.StrategyBotRunning), botDto.RunState)
}

func TestStrategyBotApplicationStartingAlreadyRunningChangesNothing(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	running := storedBot(vo.StrategyBotRunning)
	running.LastSentSignal = string(vo.SignalBuy)

	underTest.telegramDeliveryRepository.EXPECT().
		FindOneByUser(gomock.Any(), strategyBotOwnerID).
		Return(entities.TelegramDelivery{UserID: strategyBotOwnerID}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(running, nil).AnyTimes()

	botDto, startError := underTest.strategyBotApplication.StartStrategyBot(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.NoError(t, startError)
	assert.Equal(t, string(vo.StrategyBotRunning), botDto.RunState)
	// A second press must not turn what it already said into a repeat message.
	assert.Equal(t, string(vo.SignalBuy), botDto.LastSentSignal)
}

func TestStrategyBotApplicationStoppingAnAlreadyStoppedBotIsNotAFailure(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(storedBot(vo.StrategyBotStopped), nil)
	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	botDto, stopError := underTest.strategyBotApplication.StopStrategyBot(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.NoError(t, stopError)
	assert.Equal(t, string(vo.StrategyBotStopped), botDto.RunState)
}

func TestStrategyBotApplicationStopRefusesToTouchSomebodyElsesBot(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(storedBot(vo.StrategyBotRunning), nil)

	_, stopError := underTest.strategyBotApplication.StopStrategyBot(
		context.Background(), strategyBotStrangerID, strategyBotID)

	require.ErrorIs(t, stopError, domains.ErrStrategyBotNotFound)
}

func TestStrategyBotApplicationReportsStorageThatCouldNotAnswer(t *testing.T) {
	testCases := []struct {
		name    string
		arrange func(underTest strategyBotApplicationUnderTest)
		act     func(underTest strategyBotApplicationUnderTest) error
	}{
		{
			name: "deleting one that cannot be read",
			arrange: func(underTest strategyBotApplicationUnderTest) {
				underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
					Return(entities.StrategyBot{}, errors.New("the database went away"))
			},
			act: func(underTest strategyBotApplicationUnderTest) error {
				return underTest.strategyBotApplication.DeleteStrategyBot(
					context.Background(), strategyBotOwnerID, strategyBotID)
			},
		},
		{
			name: "listing when the read fails",
			arrange: func(underTest strategyBotApplicationUnderTest) {
				underTest.strategyBotRepository.EXPECT().
					FindAllByOwner(gomock.Any(), strategyBotOwnerID).
					Return(nil, errors.New("the database went away"))
			},
			act: func(underTest strategyBotApplicationUnderTest) error {
				_, listError := underTest.strategyBotApplication.ListStrategyBots(
					context.Background(), strategyBotOwnerID)

				return listError
			},
		},
		{
			name: "starting when the delivery setting cannot be read",
			arrange: func(underTest strategyBotApplicationUnderTest) {
				underTest.telegramDeliveryRepository.EXPECT().
					FindOneByUser(gomock.Any(), strategyBotOwnerID).
					Return(entities.TelegramDelivery{}, errors.New("the database went away"))
			},
			act: func(underTest strategyBotApplicationUnderTest) error {
				_, startError := underTest.strategyBotApplication.StartStrategyBot(
					context.Background(), strategyBotOwnerID, strategyBotID)

				return startError
			},
		},
		{
			name: "starting when the running bots cannot be counted",
			arrange: func(underTest strategyBotApplicationUnderTest) {
				underTest.telegramDeliveryRepository.EXPECT().
					FindOneByUser(gomock.Any(), strategyBotOwnerID).
					Return(entities.TelegramDelivery{UserID: strategyBotOwnerID}, nil)
				underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
					Return(storedBot(vo.StrategyBotStopped), nil)
				underTest.strategyBotRepository.EXPECT().
					CountRunningByOwner(gomock.Any(), strategyBotOwnerID).
					Return(0, errors.New("the database went away"))
			},
			act: func(underTest strategyBotApplicationUnderTest) error {
				_, startError := underTest.strategyBotApplication.StartStrategyBot(
					context.Background(), strategyBotOwnerID, strategyBotID)

				return startError
			},
		},
		{
			name: "starting when the new state cannot be written",
			arrange: func(underTest strategyBotApplicationUnderTest) {
				underTest.telegramDeliveryRepository.EXPECT().
					FindOneByUser(gomock.Any(), strategyBotOwnerID).
					Return(entities.TelegramDelivery{UserID: strategyBotOwnerID}, nil)
				underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
					Return(storedBot(vo.StrategyBotStopped), nil)
				underTest.strategyBotRepository.EXPECT().
					CountRunningByOwner(gomock.Any(), strategyBotOwnerID).Return(0, nil)
				underTest.strategyBotRepository.EXPECT().
					UpdateRunState(gomock.Any(), gomock.Any()).
					Return(errors.New("the database went away"))
			},
			act: func(underTest strategyBotApplicationUnderTest) error {
				_, startError := underTest.strategyBotApplication.StartStrategyBot(
					context.Background(), strategyBotOwnerID, strategyBotID)

				return startError
			},
		},
		{
			name: "stopping when the new state cannot be written",
			arrange: func(underTest strategyBotApplicationUnderTest) {
				underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
					Return(storedBot(vo.StrategyBotRunning), nil)
				underTest.strategyBotRepository.EXPECT().
					UpdateRunState(gomock.Any(), gomock.Any()).
					Return(errors.New("the database went away"))
			},
			act: func(underTest strategyBotApplicationUnderTest) error {
				_, stopError := underTest.strategyBotApplication.StopStrategyBot(
					context.Background(), strategyBotOwnerID, strategyBotID)

				return stopError
			},
		},
		{
			name: "creating when the write fails",
			arrange: func(underTest strategyBotApplicationUnderTest) {
				underTest.expectTheNamedTradingStrategyIsThisPersons()
				underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
					Return(entities.StrategyBot{}, errors.New("the database went away"))
			},
			act: func(underTest strategyBotApplicationUnderTest) error {
				_, createError := underTest.strategyBotApplication.CreateStrategyBot(
					context.Background(), strategyBotOwnerID, aBotWrite())

				return createError
			},
		},
		{
			name: "rewriting when the bot cannot be read",
			arrange: func(underTest strategyBotApplicationUnderTest) {
				underTest.expectTheNamedTradingStrategyIsThisPersons()
				underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
					Return(entities.StrategyBot{}, errors.New("the database went away"))
			},
			act: func(underTest strategyBotApplicationUnderTest) error {
				writeDto := aBotWrite()
				writeDto.ID = strategyBotID
				_, updateError := underTest.strategyBotApplication.UpdateStrategyBot(
					context.Background(), strategyBotOwnerID, writeDto)

				return updateError
			},
		},
		{
			name: "rewriting when the write fails",
			arrange: func(underTest strategyBotApplicationUnderTest) {
				underTest.expectTheNamedTradingStrategyIsThisPersons()
				underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
					Return(storedBot(vo.StrategyBotStopped), nil)
				underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
					Return(entities.StrategyBot{}, errors.New("the database went away"))
			},
			act: func(underTest strategyBotApplicationUnderTest) error {
				writeDto := aBotWrite()
				writeDto.ID = strategyBotID
				_, updateError := underTest.strategyBotApplication.UpdateStrategyBot(
					context.Background(), strategyBotOwnerID, writeDto)

				return updateError
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotApplicationUnderTest(t)
			testCase.arrange(underTest)

			// A failure to read or write must be reported, never quietly answered
			// with nothing: a bot that looks stopped because the write failed is a
			// bot its owner believes they turned off.
			assert.Error(t, testCase.act(underTest))
		})
	}
}

func TestStrategyBotApplicationRewriteRefusesATradingStrategyThisPersonCannotSee(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), botsTradingStrategyID).
		Return(entities.TradingStrategy{
			ID: botsTradingStrategyID, OwnerID: strategyBotStrangerID, Name: "別人的",
		}, nil)

	writeDto := aBotWrite()
	writeDto.ID = strategyBotID

	_, updateError := underTest.strategyBotApplication.UpdateStrategyBot(
		context.Background(), strategyBotOwnerID, writeDto)

	// The gate is walked on a rewrite exactly as on a create: a bot must not be able
	// to acquire rules it could not have been built with.
	require.ErrorIs(t, updateError, domains.ErrTradingStrategyNotFound)
}

func TestStrategyBotApplicationTellsItsOwnerWhenABotStartsAndStops(t *testing.T) {
	// 按下播放然後走開，是機器人的全部重點——而在此之前，第一件證實那一走開有效的事
	// 是一個可能好幾個小時之後才出現的交易訊號。
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.telegramDeliveryRepository.EXPECT().
		FindOneByUser(gomock.Any(), strategyBotOwnerID).
		Return(entities.TelegramDelivery{UserID: strategyBotOwnerID}, nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(storedBot(vo.StrategyBotStopped), nil)
	underTest.strategyBotRepository.EXPECT().
		CountRunningByOwner(gomock.Any(), strategyBotOwnerID).Return(0, nil)
	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	_, startError := underTest.strategyBotApplication.StartStrategyBot(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.NoError(t, startError)
	require.Len(t, underTest.announced(), 1)
	assert.Contains(t, underTest.announced()[0], "【已啟動】早盤突破")
}

func TestStrategyBotApplicationSaysNothingWhenTheButtonChangedNothing(t *testing.T) {
	// 第二次按的是一個它已經在的狀態，而一則說明那件事的訊息是一則關於沒發生的事的訊息。
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.telegramDeliveryRepository.EXPECT().
		FindOneByUser(gomock.Any(), strategyBotOwnerID).
		Return(entities.TelegramDelivery{UserID: strategyBotOwnerID}, nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(storedBot(vo.StrategyBotRunning), nil)
	_, startError := underTest.strategyBotApplication.StartStrategyBot(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.NoError(t, startError)
	assert.Empty(t, underTest.announced())
}

func TestStrategyBotApplicationStopsEvenWhenItCannotSaySo(t *testing.T) {
	// 按下停止不會因為 Telegram 忙就被收回：那台**已經**停了，按鈕做了它說的事。
	// 回報失敗只會讓人對著一台已經關掉的機器人再按一次。
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(storedBot(vo.StrategyBotRunning), nil)
	underTest.strategyBotRepository.EXPECT().
		UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)
	underTest.telegramDeliveryRepository.EXPECT().
		FindOneByUser(gomock.Any(), strategyBotOwnerID).
		Return(entities.TelegramDelivery{}, domains.ErrTelegramDeliveryNotConfigured)

	stoppedBot, stopError := underTest.strategyBotApplication.StopStrategyBot(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.NoError(t, stopError)
	assert.Equal(t, string(vo.StrategyBotStopped), stoppedBot.RunState)
}

func TestStrategyBotApplicationListsWhatABotHasBeenDoing(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(storedBot(vo.StrategyBotRunning), nil)
	underTest.strategyBotRunRecordRepository.EXPECT().
		FindLatestByBot(gomock.Any(), strategyBotID).
		Return([]entities.StrategyBotRunRecord{
			{RunNumber: 2, RanAt: time.Date(2026, 9, 16, 13, 5, 0, 0, time.UTC), Result: "buy"},
			{RunNumber: 1, RanAt: time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC), Result: "hold"},
		}, nil)

	runRecords, listError := underTest.strategyBotApplication.ListRunRecords(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.NoError(t, listError)
	require.Len(t, runRecords, 2)
	assert.Equal(t, 2, runRecords[0].RunNumber)
	assert.Equal(t, "buy", runRecords[0].Result)
	assert.Equal(t, time.Date(2026, 9, 16, 13, 5, 0, 0, time.UTC), runRecords[0].RanAt)
}

func TestStrategyBotApplicationRefusesSomebodyElsesHistory(t *testing.T) {
	// 歷史與機器人本身同一條規則：看不到就是找不到。
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(storedBot(vo.StrategyBotRunning), nil)

	_, listError := underTest.strategyBotApplication.ListRunRecords(
		context.Background(), strategyBotStrangerID, strategyBotID)

	require.ErrorIs(t, listError, domains.ErrStrategyBotNotFound)
}

// Nothing here lends, so a bot may not suggest a loan. The refusal is the replay's own
// sentence, because there is one of it.
func TestStrategyBotApplicationCreateRefusesBorrowing(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.expectTheNamedTradingStrategyIsThisPersons()

	writeDto := aBotWrite()
	writeDto.PositionPlan = dto.PositionPlanSettingsDto{Capital: decimal.NewFromInt(150)}
	writeDto.DeclaredLeverage = decimal.RequireFromString("1.8")

	_, createError := underTest.strategyBotApplication.CreateStrategyBot(
		context.Background(), strategyBotOwnerID, writeDto)

	require.Error(t, createError)
	require.ErrorIs(t, createError, domains.ErrStrategyBotValidation)
	assert.Contains(t, createError.Error(),
		"這個系統只重演現貨，開不了槓桿——現貨是拿現金換東西，沒有人借錢給你")
}
