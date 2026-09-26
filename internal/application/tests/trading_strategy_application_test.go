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

// newTradingStrategyApplicationUnderTest wires the real domain services and models, mocking only
// the outermost boundaries, so every name, source and condition rule runs for real.
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
				strategyBotRepository, strategyBotRunRecordRepository,
				mocks.NewMockIContractTradingSymbolRepository(controller),
				mocks.NewMockIContractMaintenanceMarginTierRepository(controller),
				mocks.NewMockIContractFundingRateSettlementRepository(controller),
				clockProxy),
		),
		tradingStrategyRepository:         tradingStrategyRepository,
		strategyBotRepository:             strategyBotRepository,
		strategyScriptRepository:          strategyScriptRepository,
		publishedStrategyScriptRepository: publishedStrategyScriptRepository,
	}
}

// expectNoMarketplaceQuestion pins that resolving the caller's own script never asks the
// marketplace.
func (underTest tradingStrategyApplicationUnderTest) expectNoMarketplaceQuestion() {
	underTest.publishedStrategyScriptRepository.EXPECT().
		FindOne(gomock.Any(), gomock.Any()).Times(0)
}

// expectMarketplaceQuestion covers somebody else's script, where only the marketplace gate can
// still open.
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

	// Same sentence as a nonexistent script, so a source cannot probe for other people's scripts.
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

	// Caught at save time rather than as a script failure that would stop every following bot.
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
	// The refusal names the bot to stop, not just a count.
	assert.ErrorContains(t, updateError, "幣安盯盤")
}

func TestTradingStrategyApplicationUpdateGoesAheadWhenEveryFollowingBotIsStopped(t *testing.T) {
	underTest := newTradingStrategyApplicationUnderTest(t)

	// Read twice: here to confirm ownership before revealing bots, and again inside the writing
	// service.
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

	// A stopped bot picks up the new rules on its next start.
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

			// A failed bot lookup must not be read as "no bots", or the delete would break the bots
			// that exist.
			assert.Error(t, testCase.act(underTest))
		})
	}
}

// aMixedCoarsenessTradingStrategy predates the coarseness rule: two sources on different clocks,
// never migrated.
func aMixedCoarsenessTradingStrategy() entities.TradingStrategy {
	tradingStrategy := storedTradingStrategy()
	tradingStrategy.SignalSources = append(tradingStrategy.SignalSources,
		entities.TradingStrategySignalSource{
			ID: 21, TradingStrategyID: tradingStrategyID, Label: "B",
			StrategyScriptID: 9, AggregationInterval: "5m",
		})

	return tradingStrategy
}

// Rules saved before the coarseness rule read back unchanged; silently fixing them would go
// unnoticed.
func TestTradingStrategyApplicationReadsAMixedOneBackUnchanged(t *testing.T) {
	underTest := newTradingStrategyApplicationUnderTest(t)
	underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).
		Return(aMixedCoarsenessTradingStrategy(), nil)

	tradingStrategyDto, readError := underTest.tradingStrategyApplication.GetTradingStrategy(
		context.Background(), strategyBotOwnerID, tradingStrategyID)

	require.NoError(t, readError)
	require.Len(t, tradingStrategyDto.SignalSources, 2)
	assert.Equal(t, "1h", tradingStrategyDto.SignalSources[0].AggregationInterval)
	assert.Equal(t, "5m", tradingStrategyDto.SignalSources[1].AggregationInterval)
}

// Any rewrite of a mixed one must reconcile it, even a name-only edit, because the rule applies to
// what is saved.
func TestTradingStrategyApplicationUpdateRefusesAMixedOneEvenWhenOnlyTheNameChanged(t *testing.T) {
	underTest := newTradingStrategyApplicationUnderTest(t)
	underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).
		Return(aMixedCoarsenessTradingStrategy(), nil).AnyTimes()
	underTest.expectFollowingBots()
	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(aScriptOwnedByTheCaller(9), nil).AnyTimes()
	underTest.expectNoMarketplaceQuestion()
	underTest.tradingStrategyRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	writeDto := aTradingStrategyWrite()
	writeDto.ID = tradingStrategyID
	writeDto.Name = "改個名字"
	writeDto.SignalSources = append(writeDto.SignalSources,
		dto.TradingStrategySignalSourceWriteDto{
			Label: "B", StrategyScriptID: 9, AggregationInterval: "5m"})

	_, updateError := underTest.tradingStrategyApplication.UpdateTradingStrategy(
		context.Background(), strategyBotOwnerID, writeDto)

	require.ErrorIs(t, updateError, domains.ErrTradingStrategyValidation)
	assert.ErrorContains(t, updateError, "1h")
	assert.ErrorContains(t, updateError, "5m")
}

// Making the coarsenesses match is all it takes.
func TestTradingStrategyApplicationUpdateAcceptsAMixedOneOnceItIsReconciled(t *testing.T) {
	underTest := newTradingStrategyApplicationUnderTest(t)
	underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).
		Return(aMixedCoarsenessTradingStrategy(), nil).Times(2)
	underTest.expectFollowingBots()
	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(aScriptOwnedByTheCaller(9), nil).AnyTimes()
	underTest.expectNoMarketplaceQuestion()
	underTest.tradingStrategyRepository.EXPECT().
		Save(gomock.Any(), gomock.Any()).Return(storedTradingStrategy(), nil)

	writeDto := aTradingStrategyWrite()
	writeDto.ID = tradingStrategyID
	writeDto.SignalSources = append(writeDto.SignalSources,
		dto.TradingStrategySignalSourceWriteDto{
			Label: "B", StrategyScriptID: 9, AggregationInterval: "1h"})

	_, updateError := underTest.tradingStrategyApplication.UpdateTradingStrategy(
		context.Background(), strategyBotOwnerID, writeDto)

	require.NoError(t, updateError)
}

func TestTradingStrategyApplicationBuildsOnlyOnThisPersonsOwnScripts(t *testing.T) {
	strangersPublishedScript := aScriptOwnedByTheCaller(9)
	strangersPublishedScript.OwnerID = strategyBotStrangerID
	strangersPublishedScript.Publication = &entities.PublishedStrategyScript{StrategyScriptID: 9}
	adoptedCopy := aScriptOwnedByTheCaller(9)
	adoptedCopy.IsAdoptedFromMarketplace = true

	testCases := []struct {
		name          string
		namedScript   entities.StrategyScript
		expectedError error
	}{
		{name: "someone else's published script is refused with a way forward", namedScript: strangersPublishedScript,
			expectedError: domains.ErrStrategyScriptNotYours},
		{name: "a copy adopted from the marketplace is this person's own", namedScript: adoptedCopy},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newTradingStrategyApplicationUnderTest(t)
			underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).Return(testCase.namedScript, nil)
			underTest.expectMarketplaceQuestion()
			if testCase.expectedError == nil {
				underTest.tradingStrategyRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(storedTradingStrategy(), nil)
			}

			_, createError := underTest.tradingStrategyApplication.CreateTradingStrategy(
				context.Background(), strategyBotOwnerID, aTradingStrategyWrite())

			if testCase.expectedError == nil {
				require.NoError(t, createError)
				return
			}
			require.ErrorIs(t, createError, testCase.expectedError)
			assert.Contains(t, createError.Error(), "不是你的，請先把它加入你的策略腳本")
		})
	}
}
