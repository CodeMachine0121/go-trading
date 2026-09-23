package application_test

import (
	"context"
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

const replayedTradingStrategyID = uint(11)

type tradingStrategyBacktestUnderTest struct {
	application               *application.TradingStrategyBacktestApplication
	kCandleRepository         *mocks.MockIKCandleRepository
	indicatorScriptProxy      *mocks.MockIIndicatorScriptProxy
	tradingStrategyRepository *mocks.MockITradingStrategyRepository
	strategyScriptRepository  *mocks.MockIStrategyScriptRepository
}

// newTradingStrategyBacktestUnderTest wires the real domain services and real domain
// models, mocking only the outermost boundaries: storage and script execution. Every
// rule about coarseness, conditions and conflicts is therefore exercised through
// this, not stubbed out behind it.
func newTradingStrategyBacktestUnderTest(t *testing.T) tradingStrategyBacktestUnderTest {
	controller := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(backtestNow).AnyTimes()

	tradingStrategyRepository := mocks.NewMockITradingStrategyRepository(controller)
	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
	strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, id uint) (entities.StrategyScript, error) {
			return entities.StrategyScript{
				ID: id, OwnerID: backtestViewerID, Script: "the script",
			}, nil
		}).AnyTimes()
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	return tradingStrategyBacktestUnderTest{
		application: application.NewTradingStrategyBacktestApplication(
			service.NewTradingStrategyService(tradingStrategyRepository),
			service.NewStrategyScriptService(
				strategyScriptRepository, publishedStrategyScriptRepository),
			service.NewBacktestService(
				kCandleRepository, indicatorScriptProxy, clockProxy, queryMaxResults, time.Minute), nil),
		kCandleRepository:         kCandleRepository,
		indicatorScriptProxy:      indicatorScriptProxy,
		tradingStrategyRepository: tradingStrategyRepository,
		strategyScriptRepository:  strategyScriptRepository,
	}
}

// aReplayedTradingStrategy is two sources, A and B, both reading the same coarseness,
// with "buy when both say buy" and "sell when A says sell".
func aReplayedTradingStrategy(
	intervals ...string,
) entities.TradingStrategy {
	sources := make([]entities.TradingStrategySignalSource, 0, len(intervals))
	for index, interval := range intervals {
		sources = append(sources, entities.TradingStrategySignalSource{
			ID:                  uint(20 + index),
			TradingStrategyID:   replayedTradingStrategyID,
			Label:               string(rune('A' + index)),
			StrategyScriptID:    uint(9 + index),
			AggregationInterval: interval,
		})
	}

	// A condition may only name sources that were declared, so the buy tree is built
	// to match however many there are: one source buys on its own word, two buy only
	// when both agree.
	parentID := uint(10)
	conditionNodes := []entities.TradingStrategyConditionNode{
		{ID: 13, TradingStrategyID: replayedTradingStrategyID, Side: "sell",
			SourceLabel: "A", ExpectedSignal: "sell"},
	}

	if len(intervals) == 1 {
		conditionNodes = append(conditionNodes, entities.TradingStrategyConditionNode{
			ID: parentID, TradingStrategyID: replayedTradingStrategyID, Side: "buy",
			SourceLabel: "A", ExpectedSignal: "buy",
		})
	} else {
		conditionNodes = append(conditionNodes,
			entities.TradingStrategyConditionNode{
				ID: parentID, TradingStrategyID: replayedTradingStrategyID, Side: "buy",
				Operator: string(vo.ConditionOperatorAnd),
			},
			entities.TradingStrategyConditionNode{
				ID: 11, TradingStrategyID: replayedTradingStrategyID, Side: "buy",
				ParentID: &parentID, Position: 0, SourceLabel: "A", ExpectedSignal: "buy",
			},
			entities.TradingStrategyConditionNode{
				ID: 12, TradingStrategyID: replayedTradingStrategyID, Side: "buy",
				ParentID: &parentID, Position: 1, SourceLabel: "B", ExpectedSignal: "buy",
			})
	}

	return entities.TradingStrategy{
		ID: replayedTradingStrategyID, OwnerID: backtestViewerID, Name: "黃金交叉",
		SignalSources:  sources,
		ConditionNodes: conditionNodes,
	}
}

func tradingStrategyBacktestRequestDto() dto.TradingStrategyBacktestRequestDto {
	return dto.TradingStrategyBacktestRequestDto{
		Symbol:             "BTCUSDT",
		StartTime:          backtestStart,
		EndTime:            backtestStart.Add(4 * time.Hour),
		InitialCapital:     decimal.NewFromInt(10000),
		PositionSizingMode: "allIn",
	}
}

func (fixture tradingStrategyBacktestUnderTest) run() (dto.BacktestResultDto, error) {
	return fixture.application.RunTradingStrategyBacktest(
		context.Background(), backtestViewerID, replayedTradingStrategyID,
		tradingStrategyBacktestRequestDto())
}

// expectSourceSignals makes each source's script say these things, in the order the
// sources were declared.
func (fixture tradingStrategyBacktestUnderTest) expectSourceSignals(
	signalsBySource ...[]vo.SignalVo,
) {
	for _, signals := range signalsBySource {
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(signalsSaying(signals...), nil)
	}
}

func TestTradingStrategyBacktestReplaysTheWholeSetOfRules(t *testing.T) {
	fixture := newTradingStrategyBacktestUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), replayedTradingStrategyID).
		Return(aReplayedTradingStrategy("1h", "1h"), nil)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{
			storedHourlyCandle(0, "100"),
			storedHourlyCandle(1, "110"),
			storedHourlyCandle(2, "120"),
		}, nil)
	// A and B both say buy on the first candle, so the buy condition holds there and
	// the account goes long at 100; A says sell on the last, so it closes at 120.
	fixture.expectSourceSignals(
		[]vo.SignalVo{vo.SignalBuy, vo.SignalHold, vo.SignalSell},
		[]vo.SignalVo{vo.SignalBuy, vo.SignalHold, vo.SignalHold},
	)

	resultDto, err := fixture.run()

	require.NoError(t, err)
	assert.Equal(t, "1h", resultDto.Interval)
	assert.Equal(t, 3, resultDto.UsedCandleCount)
	// One round trip: long at 100, reversed at 120.
	require.Len(t, resultDto.ClosedTrades, 1)
	assert.Equal(t, string(vo.PositionDirectionLong), resultDto.ClosedTrades[0].Direction)
	assert.Equal(t, 0, resultDto.Summary.ConflictedCandleCount)
}

func TestTradingStrategyBacktestTakesItsCoarsenessFromTheSources(t *testing.T) {
	fixture := newTradingStrategyBacktestUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), replayedTradingStrategyID).
		Return(aReplayedTradingStrategy("1h"), nil)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{
			storedHourlyCandle(0, "100"), storedHourlyCandle(1, "110"),
		}, nil)
	fixture.expectSourceSignals([]vo.SignalVo{vo.SignalHold, vo.SignalHold})

	resultDto, err := fixture.run()

	require.NoError(t, err)
	assert.Equal(t, "1h", resultDto.Interval)
}

func TestTradingStrategyBacktestRefusesSourcesThatDisagreeAboutCoarseness(t *testing.T) {
	// Replaying walks one candle at a time, and an hour's candle and a five-minute
	// candle are not the same one. The refusal names both, because what the person
	// has to go and do is make them the same.
	fixture := newTradingStrategyBacktestUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), replayedTradingStrategyID).
		Return(aReplayedTradingStrategy("1h", "5m"), nil)

	_, err := fixture.run()

	require.ErrorIs(t, err, domains.ErrBacktestValidation)
	assert.ErrorContains(t, err, "1h")
	assert.ErrorContains(t, err, "5m")
}

func TestTradingStrategyBacktestCountsNoConflictWhenTheTwoTreesCannotBothHold(t *testing.T) {
	// This trading strategy buys only when both sources say buy and sells when A says
	// sell, so no candle can satisfy both. The count has to be zero — a count that
	// drifts upward on its own would make every replay look like a broken strategy.
	fixture := newTradingStrategyBacktestUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), replayedTradingStrategyID).
		Return(aReplayedTradingStrategy("1h", "1h"), nil)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{
			storedHourlyCandle(0, "100"),
			storedHourlyCandle(1, "110"),
			storedHourlyCandle(2, "120"),
		}, nil)
	fixture.expectSourceSignals(
		[]vo.SignalVo{vo.SignalHold, vo.SignalSell, vo.SignalHold},
		[]vo.SignalVo{vo.SignalHold, vo.SignalBuy, vo.SignalHold},
	)

	resultDto, err := fixture.run()

	require.NoError(t, err)
	assert.Equal(t, 0, resultDto.Summary.ConflictedCandleCount)
}

// aConflictingTradingStrategy buys when B says buy and sells when A says sell, so one
// candle can satisfy both at once.
func aConflictingTradingStrategy() entities.TradingStrategy {
	tradingStrategy := aReplayedTradingStrategy("1h", "1h")
	tradingStrategy.ConditionNodes = []entities.TradingStrategyConditionNode{
		{ID: 10, TradingStrategyID: replayedTradingStrategyID, Side: "buy",
			SourceLabel: "B", ExpectedSignal: "buy"},
		{ID: 13, TradingStrategyID: replayedTradingStrategyID, Side: "sell",
			SourceLabel: "A", ExpectedSignal: "sell"},
	}

	return tradingStrategy
}

func TestTradingStrategyBacktestTreatsAConflictAsDoingNothingAndSaysHowOften(t *testing.T) {
	// Picking a side would hand somebody an opinion the system invented, and they
	// would act on it without ever tracing it back. The count is the only way they
	// find out — and a replay that barely traded reads as a very steady strategy.
	fixture := newTradingStrategyBacktestUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), replayedTradingStrategyID).
		Return(aConflictingTradingStrategy(), nil)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{
			storedHourlyCandle(0, "100"),
			storedHourlyCandle(1, "110"),
			storedHourlyCandle(2, "120"),
		}, nil)
	// Both conditions hold on the first two candles.
	fixture.expectSourceSignals(
		[]vo.SignalVo{vo.SignalSell, vo.SignalSell, vo.SignalHold},
		[]vo.SignalVo{vo.SignalBuy, vo.SignalBuy, vo.SignalHold},
	)

	resultDto, err := fixture.run()

	require.NoError(t, err)
	assert.Equal(t, 2, resultDto.Summary.ConflictedCandleCount)
	// Nothing was ever opened: every conflicted candle does nothing at all.
	assert.Equal(t, 0, resultDto.Summary.PositionOpenCount)
}

func TestTradingStrategyBacktestRefusesOneThatIsNotThisPersons(t *testing.T) {
	fixture := newTradingStrategyBacktestUnderTest(t)
	strangers := aReplayedTradingStrategy("1h")
	strangers.OwnerID = backtestViewerID + 1
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), replayedTradingStrategyID).Return(strangers, nil)

	_, err := fixture.run()

	// The same sentence as naming one that does not exist.
	require.ErrorIs(t, err, domains.ErrTradingStrategyNotFound)
}

// A trading strategy whose only source is A, buying on A's buy and selling on A's
// sell, is the same question as replaying that script on its own. The two must give
// the same report card — otherwise "rehearse what you will run" is not true.
func TestTradingStrategyBacktestOfOneSourceMatchesReplayingThatScript(t *testing.T) {
	signals := []vo.SignalVo{vo.SignalBuy, vo.SignalHold, vo.SignalSell}
	candles := []entities.KCandle{
		storedHourlyCandle(0, "100"),
		storedHourlyCandle(1, "110"),
		storedHourlyCandle(2, "120"),
	}

	scriptFixture := newBacktestUnderTest(t)
	scriptFixture.kCandleRepository.EXPECT().
		FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(candles, nil)
	scriptFixture.indicatorScriptProxy.EXPECT().
		ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		Return(signalsSaying(signals...), nil)

	scriptResult, scriptError := scriptFixture.backtestApplication.RunBacktest(
		t.Context(), backtestViewerID,
		namingStrategyScript(t, backtestStrategyScriptID), backtestRequestDto())
	require.NoError(t, scriptError)

	tradingStrategy := aReplayedTradingStrategy("1h")
	tradingStrategy.ConditionNodes = []entities.TradingStrategyConditionNode{
		{ID: 10, TradingStrategyID: replayedTradingStrategyID, Side: "buy",
			SourceLabel: "A", ExpectedSignal: "buy"},
		{ID: 13, TradingStrategyID: replayedTradingStrategyID, Side: "sell",
			SourceLabel: "A", ExpectedSignal: "sell"},
	}

	tradingStrategyFixture := newTradingStrategyBacktestUnderTest(t)
	tradingStrategyFixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), replayedTradingStrategyID).Return(tradingStrategy, nil)
	tradingStrategyFixture.kCandleRepository.EXPECT().
		FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).Return(candles, nil)
	tradingStrategyFixture.expectSourceSignals(signals)

	tradingStrategyResult, tradingStrategyError := tradingStrategyFixture.run()
	require.NoError(t, tradingStrategyError)

	assert.Equal(t, scriptResult.Summary, tradingStrategyResult.Summary)
	assert.Equal(t, scriptResult.ClosedTrades, tradingStrategyResult.ClosedTrades)
	assert.Equal(t, scriptResult.EquityCurve, tradingStrategyResult.EquityCurve)
}

// Naming one that does not exist and naming somebody else's are answered with the
// same sentence. Two different answers would turn this field into a way to find out
// which trading strategies other people have.
func TestTradingStrategyBacktestAnswersTheSameForOneThatIsNotThere(t *testing.T) {
	fixture := newTradingStrategyBacktestUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), replayedTradingStrategyID).
		Return(entities.TradingStrategy{},
			domains.TradingStrategyNotFound(replayedTradingStrategyID))

	_, err := fixture.run()

	require.ErrorIs(t, err, domains.ErrTradingStrategyNotFound)
}

// A source may name a script its owner has since deleted. Half a replay — the other
// sources' opinions with this one silently missing — would be a report card for a
// strategy nobody wrote, so the whole run is refused.
func TestTradingStrategyBacktestRefusesWhenASourcesScriptCannotBeRead(t *testing.T) {
	controller := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(backtestNow).AnyTimes()

	tradingStrategyRepository := mocks.NewMockITradingStrategyRepository(controller)
	tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), replayedTradingStrategyID).
		Return(aReplayedTradingStrategy("1h"), nil)

	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
	strategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.StrategyScript{}, domains.StrategyScriptNotFound(9))
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	backtestApplication := application.NewTradingStrategyBacktestApplication(
		service.NewTradingStrategyService(tradingStrategyRepository),
		service.NewStrategyScriptService(
			strategyScriptRepository, publishedStrategyScriptRepository),
		service.NewBacktestService(
			kCandleRepository, indicatorScriptProxy, clockProxy, queryMaxResults, time.Minute), nil)

	_, err := backtestApplication.RunTradingStrategyBacktest(
		t.Context(), backtestViewerID, replayedTradingStrategyID,
		tradingStrategyBacktestRequestDto())

	require.ErrorIs(t, err, domains.ErrStrategyScriptNotFound)
}

// The rules a replay is refused under do not change because several scripts read the
// candles instead of one. One candle is not a stretch either way.
func TestTradingStrategyBacktestRefusesAStretchWithTooFewCandles(t *testing.T) {
	fixture := newTradingStrategyBacktestUnderTest(t)
	fixture.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), replayedTradingStrategyID).
		Return(aReplayedTradingStrategy("1h"), nil)
	fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{storedHourlyCandle(0, "100")}, nil)

	_, err := fixture.run()

	require.ErrorIs(t, err, domains.ErrBacktestValidation)
}
