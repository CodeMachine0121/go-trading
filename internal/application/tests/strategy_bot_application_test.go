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
)

type strategyBotApplicationUnderTest struct {
	strategyBotApplication      *application.StrategyBotApplication
	strategyBotRepository       *mocks.MockIStrategyBotRepository
	strategyRepository          *mocks.MockIStrategyRepository
	publishedStrategyRepository *mocks.MockIPublishedStrategyRepository
	telegramDeliveryRepository  *mocks.MockITelegramDeliveryRepository
	clockProxy                  *mocks.MockIClockProxy
}

// newStrategyBotApplicationUnderTest wires the real domain services and the real
// models, mocking only the outermost boundaries: storage, the seal, the carrier and
// the clock. Every rule about bots, sources and conditions is therefore exercised
// through this, not stubbed out behind it.
func newStrategyBotApplicationUnderTest(t *testing.T) strategyBotApplicationUnderTest {
	controller := gomock.NewController(t)

	strategyBotRepository := mocks.NewMockIStrategyBotRepository(controller)
	strategyRepository := mocks.NewMockIStrategyRepository(controller)
	publishedStrategyRepository := mocks.NewMockIPublishedStrategyRepository(controller)
	telegramDeliveryRepository := mocks.NewMockITelegramDeliveryRepository(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)

	clockProxy.EXPECT().Now().Return(time.Date(2026, 9, 16, 13, 0, 0, 0, time.UTC)).AnyTimes()

	return strategyBotApplicationUnderTest{
		strategyBotApplication: application.NewStrategyBotApplication(
			service.NewStrategyBotService(strategyBotRepository, clockProxy),
			service.NewStrategyService(strategyRepository, publishedStrategyRepository),
			service.NewTelegramDeliveryService(
				telegramDeliveryRepository,
				mocks.NewMockISecretSealProxy(controller),
				mocks.NewMockIMessageDeliveryProxy(controller),
			),
		),
		strategyBotRepository:       strategyBotRepository,
		strategyRepository:          strategyRepository,
		publishedStrategyRepository: publishedStrategyRepository,
		telegramDeliveryRepository:  telegramDeliveryRepository,
		clockProxy:                  clockProxy,
	}
}

// expectNoMarketplaceQuestion pins the other half of resolving a strategy: when it
// is the caller's own, the marketplace is never asked.
//
// It is an expectation rather than an absence, because the saving is the point — a
// standing bot resolves every signal source on every round, for ever, and those are
// almost always its owner's own strategies.
func (underTest strategyBotApplicationUnderTest) expectNoMarketplaceQuestion() {
	underTest.publishedStrategyRepository.EXPECT().
		FindOne(gomock.Any(), gomock.Any()).Times(0)
}

// expectMarketplaceQuestion is the other case: somebody else's strategy, where the
// third gate is the only one that can still open.
func (underTest strategyBotApplicationUnderTest) expectMarketplaceQuestion() {
	underTest.publishedStrategyRepository.EXPECT().
		FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategy{}, domains.ErrStrategyNotPublished).AnyTimes()
}

// ownedStrategy is a strategy belonging to whoever these tests act as, declaring one
// knob so that "a value set on a knob nobody declared" has something to fail against.
func ownedStrategy(id uint) entities.Strategy {
	return entities.Strategy{
		ID: id, OwnerID: strategyBotOwnerID, Name: "均線", Script: "//", ResultType: "signal",
		Parameters: []entities.StrategyParameter{
			{StrategyID: id, Name: "回看根數", Kind: "lookbackCount", DefaultValue: 20},
		},
	}
}

func aBotWrite() dto.StrategyBotWriteDto {
	return dto.StrategyBotWriteDto{
		Name:                   "早盤突破",
		Symbol:                 "BTCUSDT",
		TriggerIntervalMinutes: 5,
		SignalSources: []dto.StrategyBotSignalSourceWriteDto{
			{Label: "A", StrategyID: 9, AggregationInterval: "1h"},
		},
		BuyCondition:  dto.StrategyBotConditionDto{SourceLabel: "A", Signal: string(vo.SignalBuy)},
		SellCondition: dto.StrategyBotConditionDto{SourceLabel: "A", Signal: string(vo.SignalSell)},
	}
}

// storedBot is a bot as it comes back out of storage.
func storedBot(runState vo.StrategyBotRunStateVo) entities.StrategyBot {
	parentID := uint(10)

	return entities.StrategyBot{
		ID: strategyBotID, OwnerID: strategyBotOwnerID, Name: "早盤突破", Symbol: "BTCUSDT",
		TriggerIntervalMinutes: 5,
		RunState:               string(runState),
		SignalSources: []entities.StrategyBotSignalSource{
			{ID: 20, StrategyBotID: strategyBotID, Label: "A", StrategyID: 9, AggregationInterval: "1h"},
		},
		ConditionNodes: []entities.StrategyBotConditionNode{
			{ID: parentID, StrategyBotID: strategyBotID, Side: "buy",
				SourceLabel: "A", ExpectedSignal: string(vo.SignalBuy)},
			{ID: 11, StrategyBotID: strategyBotID, Side: "sell",
				SourceLabel: "A", ExpectedSignal: string(vo.SignalSell)},
		},
	}
}

func TestStrategyBotApplicationCreateResolvesEveryNamedStrategyThroughTheGates(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(ownedStrategy(9), nil)
	underTest.strategyBotRepository.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) (entities.StrategyBot, error) {
			// The owner is taken from whoever is signed in, never from the body.
			assert.Equal(t, strategyBotOwnerID, bot.OwnerID)
			// A saved bot is stopped. Saving one cannot start it.
			assert.Equal(t, string(vo.StrategyBotStopped), bot.RunState)

			return storedBot(vo.StrategyBotStopped), nil
		})

	botDto, createError := underTest.strategyBotApplication.CreateStrategyBot(
		context.Background(), strategyBotOwnerID, aBotWrite())

	require.NoError(t, createError)
	assert.Equal(t, strategyBotID, botDto.ID)
	assert.Equal(t, string(vo.StrategyBotStopped), botDto.RunState)
}

func TestStrategyBotApplicationCreateRefusesAStrategyThisPersonCannotSee(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	strangersStrategy := ownedStrategy(9)
	strangersStrategy.OwnerID = strategyBotStrangerID

	underTest.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(strangersStrategy, nil)
	underTest.expectMarketplaceQuestion()

	_, createError := underTest.strategyBotApplication.CreateStrategyBot(
		context.Background(), strategyBotOwnerID, aBotWrite())

	// The same sentence as naming a strategy that does not exist, which is what
	// stops a bot's sources becoming a way to probe for other people's strategies.
	require.ErrorIs(t, createError, domains.ErrStrategyNotFound)
}

func TestStrategyBotApplicationCreateRefusesAValueOnAKnobNobodyDeclared(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	underTest.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(ownedStrategy(9), nil)

	writeDto := aBotWrite()
	writeDto.SignalSources[0].ParameterValues = []dto.StrategyParameterValueDto{
		{Name: "週期", Value: 14},
	}

	_, createError := underTest.strategyBotApplication.CreateStrategyBot(
		context.Background(), strategyBotOwnerID, writeDto)

	require.ErrorIs(t, createError, domains.ErrStrategyBotValidation)
	assert.ErrorContains(t, createError, "沒有宣告這個名字")
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

	underTest.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(ownedStrategy(9), nil)
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
		Return(entities.TelegramDelivery{UserID: strategyBotOwnerID}, nil)
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
		Return(entities.TelegramDelivery{UserID: strategyBotOwnerID}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(stopped, nil)
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
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(running, nil)

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
				underTest.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
					Return(ownedStrategy(9), nil)
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
				underTest.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
					Return(ownedStrategy(9), nil)
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
				underTest.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
					Return(ownedStrategy(9), nil)
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

func TestStrategyBotApplicationRewriteRefusesAStrategyThisPersonCannotSee(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	strangersStrategy := ownedStrategy(9)
	strangersStrategy.OwnerID = strategyBotStrangerID

	underTest.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(strangersStrategy, nil)
	underTest.expectMarketplaceQuestion()

	writeDto := aBotWrite()
	writeDto.ID = strategyBotID

	_, updateError := underTest.strategyBotApplication.UpdateStrategyBot(
		context.Background(), strategyBotOwnerID, writeDto)

	// The gates are walked on a rewrite exactly as on a create: a bot must not be
	// able to acquire a source it could not have been built with.
	require.ErrorIs(t, updateError, domains.ErrStrategyNotFound)
}
