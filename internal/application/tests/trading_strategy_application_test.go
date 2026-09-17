package application_test

import (
	"context"
	"errors"
	"testing"

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

const tradingStrategyID = uint(11)

type tradingStrategyApplicationUnderTest struct {
	tradingStrategyApplication        *application.TradingStrategyApplication
	tradingStrategyRepository         *mocks.MockITradingStrategyRepository
	strategyBotRepository             *mocks.MockIStrategyBotRepository
	strategyScriptRepository          *mocks.MockIStrategyScriptRepository
	publishedStrategyScriptRepository *mocks.MockIPublishedStrategyScriptRepository
}

// newTradingStrategyApplicationUnderTest wires the real domain services and the real
// models, mocking only the outermost boundaries. Every rule about names, sources and
// conditions is therefore exercised through this, not stubbed out behind it.
func newTradingStrategyApplicationUnderTest(t *testing.T) tradingStrategyApplicationUnderTest {
	controller := gomock.NewController(t)

	tradingStrategyRepository := mocks.NewMockITradingStrategyRepository(controller)
	strategyBotRepository := mocks.NewMockIStrategyBotRepository(controller)
	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
	strategyBotRunRecordRepository := mocks.NewMockIStrategyBotRunRecordRepository(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)

	return tradingStrategyApplicationUnderTest{
		tradingStrategyApplication: application.NewTradingStrategyApplication(
			service.NewTradingStrategyService(tradingStrategyRepository),
			service.NewStrategyScriptService(
				strategyScriptRepository, publishedStrategyScriptRepository),
			service.NewStrategyBotService(
				strategyBotRepository, strategyBotRunRecordRepository, clockProxy),
		),
		tradingStrategyRepository:         tradingStrategyRepository,
		strategyBotRepository:             strategyBotRepository,
		strategyScriptRepository:          strategyScriptRepository,
		publishedStrategyScriptRepository: publishedStrategyScriptRepository,
	}
}

// expectNoMarketplaceQuestion pins the other half of resolving a strategy script:
// when it is the caller's own, the marketplace is never asked.
func (underTest tradingStrategyApplicationUnderTest) expectNoMarketplaceQuestion() {
	underTest.publishedStrategyScriptRepository.EXPECT().
		FindOne(gomock.Any(), gomock.Any()).Times(0)
}

// expectMarketplaceQuestion is the other case: somebody else's strategy script, where
// the third gate is the only one that can still open.
func (underTest tradingStrategyApplicationUnderTest) expectMarketplaceQuestion() {
	underTest.publishedStrategyScriptRepository.EXPECT().
		FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()
}

// expectFollowingBots is what the bots say when asked who is using these rules.
func (underTest tradingStrategyApplicationUnderTest) expectFollowingBots(bots ...entities.StrategyBot) {
	underTest.strategyBotRepository.EXPECT().
		FindAllByTradingStrategy(gomock.Any(), tradingStrategyID).Return(bots, nil)
}

// aScriptOwnedByTheCaller declares one knob, so that "a value set on a knob nobody
// declared" has something to fail against.
func aScriptOwnedByTheCaller(id uint) entities.StrategyScript {
	return entities.StrategyScript{
		ID: id, OwnerID: strategyBotOwnerID, Name: "均線", Script: "//", ResultType: "signal",
		Parameters: []entities.StrategyScriptParameter{
			{StrategyScriptID: id, Name: "回看根數", Kind: "lookbackCount", DefaultValue: 20},
		},
	}
}

func aTradingStrategyWrite() dto.TradingStrategyWriteDto {
	return dto.TradingStrategyWriteDto{
		Name: "黃金交叉",
		SignalSources: []dto.TradingStrategySignalSourceWriteDto{
			{Label: "A", StrategyScriptID: 9, AggregationInterval: "1h"},
		},
		BuyCondition:  dto.TradingStrategyConditionDto{SourceLabel: "A", Signal: string(vo.SignalBuy)},
		SellCondition: dto.TradingStrategyConditionDto{SourceLabel: "A", Signal: string(vo.SignalSell)},
	}
}

// storedTradingStrategy is one as it comes back out of storage.
func storedTradingStrategy() entities.TradingStrategy {
	return entities.TradingStrategy{
		ID: tradingStrategyID, OwnerID: strategyBotOwnerID, Name: "黃金交叉",
		SignalSources: []entities.TradingStrategySignalSource{
			{ID: 20, TradingStrategyID: tradingStrategyID, Label: "A",
				StrategyScriptID: 9, AggregationInterval: "1h"},
		},
		ConditionNodes: []entities.TradingStrategyConditionNode{
			{ID: 10, TradingStrategyID: tradingStrategyID, Side: "buy",
				SourceLabel: "A", ExpectedSignal: string(vo.SignalBuy)},
			{ID: 11, TradingStrategyID: tradingStrategyID, Side: "sell",
				SourceLabel: "A", ExpectedSignal: string(vo.SignalSell)},
		},
	}
}

// aBotFollowingTheRules is one bot pointed at them, in the state named.
func aBotFollowingTheRules(name string, runState vo.StrategyBotRunStateVo) entities.StrategyBot {
	return entities.StrategyBot{
		ID: strategyBotID, OwnerID: strategyBotOwnerID, Name: name,
		TradingStrategyID: tradingStrategyID,
		RunState:          string(runState),
	}
}

func TestTradingStrategyApplicationCreateResolvesEveryNamedScriptThroughTheGates(t *testing.T) {
	underTest := newTradingStrategyApplicationUnderTest(t)

	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(aScriptOwnedByTheCaller(9), nil)
	underTest.expectNoMarketplaceQuestion()
	underTest.tradingStrategyRepository.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, tradingStrategy entities.TradingStrategy,
		) (entities.TradingStrategy, error) {
			// The owner is taken from whoever is signed in, never from the body.
			assert.Equal(t, strategyBotOwnerID, tradingStrategy.OwnerID)

			return storedTradingStrategy(), nil
		})

	tradingStrategyDto, createError := underTest.tradingStrategyApplication.CreateTradingStrategy(
		context.Background(), strategyBotOwnerID, aTradingStrategyWrite())

	require.NoError(t, createError)
	assert.Equal(t, tradingStrategyID, tradingStrategyDto.ID)
	assert.Equal(t, "黃金交叉", tradingStrategyDto.Name)
	require.Len(t, tradingStrategyDto.SignalSources, 1)
	assert.Equal(t, "A", tradingStrategyDto.SignalSources[0].Label)
}

func TestTradingStrategyApplicationCreateRefusesAScriptThisPersonCannotSee(t *testing.T) {
	underTest := newTradingStrategyApplicationUnderTest(t)

	strangersScript := aScriptOwnedByTheCaller(9)
	strangersScript.OwnerID = strategyBotStrangerID

	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(strangersScript, nil)
	underTest.expectMarketplaceQuestion()

	_, createError := underTest.tradingStrategyApplication.CreateTradingStrategy(
		context.Background(), strategyBotOwnerID, aTradingStrategyWrite())

	// The same sentence as naming a script that does not exist, which is what stops
	// a source becoming a way to probe for other people's scripts.
	require.ErrorIs(t, createError, domains.ErrStrategyScriptNotFound)
}

func TestTradingStrategyApplicationCreateRefusesAValueOnAKnobNobodyDeclared(t *testing.T) {
	underTest := newTradingStrategyApplicationUnderTest(t)

	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(aScriptOwnedByTheCaller(9), nil)
	underTest.expectNoMarketplaceQuestion()

	writeDto := aTradingStrategyWrite()
	writeDto.SignalSources[0].ParameterValues = []dto.StrategyScriptParameterValueDto{
		{Name: "週期", Value: 20},
	}

	_, createError := underTest.tradingStrategyApplication.CreateTradingStrategy(
		context.Background(), strategyBotOwnerID, writeDto)

	// Caught now rather than at three in the morning, when the same mistake would
	// come back as a script failure and stop every bot following these rules.
	require.ErrorIs(t, createError, domains.ErrTradingStrategyValidation)
	assert.ErrorContains(t, createError, "沒有宣告這個名字")
}

func TestTradingStrategyApplicationReadsAndListsOnlyThisPersons(t *testing.T) {
	underTest := newTradingStrategyApplicationUnderTest(t)

	underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).
		Return(storedTradingStrategy(), nil).Times(2)

	own, readError := underTest.tradingStrategyApplication.GetTradingStrategy(
		context.Background(), strategyBotOwnerID, tradingStrategyID)
	require.NoError(t, readError)
	assert.Equal(t, "黃金交叉", own.Name)

	_, strangersError := underTest.tradingStrategyApplication.GetTradingStrategy(
		context.Background(), strategyBotStrangerID, tradingStrategyID)

	// Somebody else's and one that does not exist share one sentence, so that
	// walking the identifiers teaches nobody what exists.
	require.ErrorIs(t, strangersError, domains.ErrTradingStrategyNotFound)
}

func TestTradingStrategyApplicationListHandsBackAnEmptyListRatherThanARefusal(t *testing.T) {
	underTest := newTradingStrategyApplicationUnderTest(t)

	underTest.tradingStrategyRepository.EXPECT().
		FindAllByOwner(gomock.Any(), strategyBotOwnerID).
		Return([]entities.TradingStrategy{}, nil)

	tradingStrategyDtos, listError := underTest.tradingStrategyApplication.ListTradingStrategies(
		context.Background(), strategyBotOwnerID)

	// Having none is the ordinary state of somebody who has not built one yet.
	require.NoError(t, listError)
	assert.Empty(t, tradingStrategyDtos)
}

func TestTradingStrategyApplicationUpdateRefusesWhileABotFollowingItIsRunning(t *testing.T) {
	underTest := newTradingStrategyApplicationUnderTest(t)

	underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).
		Return(storedTradingStrategy(), nil)
	underTest.expectFollowingBots(
		aBotFollowingTheRules("幣安盯盤", vo.StrategyBotRunning),
		aBotFollowingTheRules("已停止的", vo.StrategyBotStopped))

	writeDto := aTradingStrategyWrite()
	writeDto.ID = tradingStrategyID

	_, updateError := underTest.tradingStrategyApplication.UpdateTradingStrategy(
		context.Background(), strategyBotOwnerID, writeDto)

	require.ErrorIs(t, updateError, domains.ErrTradingStrategyBotRunning)
	// The refusal names the bot to go and stop; a count alone would leave somebody
	// opening every bot they own to find out which one.
	assert.ErrorContains(t, updateError, "幣安盯盤")
}

func TestTradingStrategyApplicationUpdateGoesAheadWhenEveryFollowingBotIsStopped(t *testing.T) {
	underTest := newTradingStrategyApplicationUnderTest(t)

	// Read twice: once here, to settle that these are this person's before anybody
	// is told about the bots, and once inside the service that does the writing.
	underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).
		Return(storedTradingStrategy(), nil).Times(2)
	underTest.expectFollowingBots(aBotFollowingTheRules("已停止的", vo.StrategyBotStopped))
	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(aScriptOwnedByTheCaller(9), nil)
	underTest.expectNoMarketplaceQuestion()
	underTest.tradingStrategyRepository.EXPECT().
		Save(gomock.Any(), gomock.Any()).Return(storedTradingStrategy(), nil)

	writeDto := aTradingStrategyWrite()
	writeDto.ID = tradingStrategyID

	_, updateError := underTest.tradingStrategyApplication.UpdateTradingStrategy(
		context.Background(), strategyBotOwnerID, writeDto)

	// A stopped bot picks the new rules up the next time it is started, and that is
	// the whole point of several bots sharing one set.
	require.NoError(t, updateError)
}

func TestTradingStrategyApplicationDeleteRefusesWhileAnyBotFollowsIt(t *testing.T) {
	testCases := []struct {
		name     string
		runState vo.StrategyBotRunStateVo
	}{
		{name: "一台執行中的機器人在用它", runState: vo.StrategyBotRunning},
		{name: "一台已停止的機器人也一樣在用它", runState: vo.StrategyBotStopped},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newTradingStrategyApplicationUnderTest(t)

			underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).
				Return(storedTradingStrategy(), nil)
			underTest.expectFollowingBots(aBotFollowingTheRules("在用它的", testCase.runState))

			deleteError := underTest.tradingStrategyApplication.DeleteTradingStrategy(
				context.Background(), strategyBotOwnerID, tradingStrategyID)

			// Whether it is switched on makes no difference: deleting would leave it
			// pointing at something that is gone.
			require.ErrorIs(t, deleteError, domains.ErrTradingStrategyInUse)
			assert.ErrorContains(t, deleteError, "1 台")
		})
	}
}

func TestTradingStrategyApplicationDeleteGoesAheadWhenNothingFollowsIt(t *testing.T) {
	underTest := newTradingStrategyApplicationUnderTest(t)

	underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).
		Return(storedTradingStrategy(), nil).Times(2)
	underTest.expectFollowingBots()
	underTest.tradingStrategyRepository.EXPECT().
		Delete(gomock.Any(), tradingStrategyID).Return(nil)

	deleteError := underTest.tradingStrategyApplication.DeleteTradingStrategy(
		context.Background(), strategyBotOwnerID, tradingStrategyID)

	require.NoError(t, deleteError)
}

func TestTradingStrategyApplicationRefusesToTouchSomebodyElses(t *testing.T) {
	testCases := []struct {
		name string
		act  func(underTest tradingStrategyApplicationUnderTest) error
	}{
		{
			name: "rewriting somebody else's",
			act: func(underTest tradingStrategyApplicationUnderTest) error {
				writeDto := aTradingStrategyWrite()
				writeDto.ID = tradingStrategyID
				_, updateError := underTest.tradingStrategyApplication.UpdateTradingStrategy(
					context.Background(), strategyBotStrangerID, writeDto)

				return updateError
			},
		},
		{
			name: "deleting somebody else's",
			act: func(underTest tradingStrategyApplicationUnderTest) error {
				return underTest.tradingStrategyApplication.DeleteTradingStrategy(
					context.Background(), strategyBotStrangerID, tradingStrategyID)
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newTradingStrategyApplicationUnderTest(t)
			underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).
				Return(storedTradingStrategy(), nil)

			// Refused before anything is read about who is following it: a stranger
			// must not learn how many bots somebody else has.
			require.ErrorIs(t, testCase.act(underTest), domains.ErrTradingStrategyNotFound)
		})
	}
}

func TestTradingStrategyApplicationReportsStorageThatCouldNotAnswer(t *testing.T) {
	testCases := []struct {
		name    string
		arrange func(underTest tradingStrategyApplicationUnderTest)
		act     func(underTest tradingStrategyApplicationUnderTest) error
	}{
		{
			name: "listing when the read fails",
			arrange: func(underTest tradingStrategyApplicationUnderTest) {
				underTest.tradingStrategyRepository.EXPECT().
					FindAllByOwner(gomock.Any(), strategyBotOwnerID).
					Return(nil, errors.New("the database went away"))
			},
			act: func(underTest tradingStrategyApplicationUnderTest) error {
				_, listError := underTest.tradingStrategyApplication.ListTradingStrategies(
					context.Background(), strategyBotOwnerID)

				return listError
			},
		},
		{
			name: "deleting when the bots cannot be read",
			arrange: func(underTest tradingStrategyApplicationUnderTest) {
				underTest.tradingStrategyRepository.EXPECT().
					FindOne(gomock.Any(), tradingStrategyID).Return(storedTradingStrategy(), nil)
				underTest.strategyBotRepository.EXPECT().
					FindAllByTradingStrategy(gomock.Any(), tradingStrategyID).
					Return(nil, errors.New("the database went away"))
			},
			act: func(underTest tradingStrategyApplicationUnderTest) error {
				return underTest.tradingStrategyApplication.DeleteTradingStrategy(
					context.Background(), strategyBotOwnerID, tradingStrategyID)
			},
		},
		{
			name: "rewriting when the bots cannot be read",
			arrange: func(underTest tradingStrategyApplicationUnderTest) {
				underTest.tradingStrategyRepository.EXPECT().
					FindOne(gomock.Any(), tradingStrategyID).Return(storedTradingStrategy(), nil)
				underTest.strategyBotRepository.EXPECT().
					FindAllByTradingStrategy(gomock.Any(), tradingStrategyID).
					Return(nil, errors.New("the database went away"))
			},
			act: func(underTest tradingStrategyApplicationUnderTest) error {
				writeDto := aTradingStrategyWrite()
				writeDto.ID = tradingStrategyID
				_, updateError := underTest.tradingStrategyApplication.UpdateTradingStrategy(
					context.Background(), strategyBotOwnerID, writeDto)

				return updateError
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newTradingStrategyApplicationUnderTest(t)
			testCase.arrange(underTest)

			// Never quietly answered with nothing: a delete that went ahead because
			// "no bots came back" would break every bot that was actually there.
			assert.Error(t, testCase.act(underTest))
		})
	}
}
