package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
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

// backtestNow sits well after every replayed stretch, so no replay is refused for reaching into an
// unfinished hour.
var backtestNow = time.Date(2026, 8, 30, 0, 0, 0, 0, time.UTC)

var backtestStart = time.Date(2026, 8, 29, 0, 0, 0, 0, time.UTC)

const (
	backtestViewerID         = uint(1)
	backtestStrategyScriptID = uint(9)
)

type backtestUnderTest struct {
	backtestApplication  *application.BacktestApplication
	backtestService      *service.BacktestService
	runStore             *backtestRunStore
	kCandleRepository    *mocks.MockIKCandleRepository
	indicatorScriptProxy *mocks.MockIIndicatorScriptProxy
}

// newBacktestUnderTest wires the real domain services and models, mocking only storage and script
// execution.
func newBacktestUnderTest(t *testing.T) backtestUnderTest {
	controller := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(backtestNow).AnyTimes()

	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
	strategyScriptRepository.EXPECT().FindOne(gomock.Any(), backtestStrategyScriptID).
		Return(entities.StrategyScript{
			ID: backtestStrategyScriptID, OwnerID: backtestViewerID, Name: "均線", Script: "the script",
		}, nil).AnyTimes()
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
	publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), gomock.Any()).
		Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()

	backtestService := service.NewBacktestService(
		kCandleRepository, indicatorScriptProxy, clockProxy, queryMaxResults, 0)
	runStore := newBacktestRunStore(controller)

	return backtestUnderTest{
		backtestApplication: application.NewBacktestApplication(
			service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository),
			backtestService, nil,
			newQueueingBacktestRunService(t, controller, runStore, backtestService, nil)),
		backtestService:      backtestService,
		runStore:             runStore,
		kCandleRepository:    kCandleRepository,
		indicatorScriptProxy: indicatorScriptProxy,
	}
}

func backtestRequestDto() dto.BacktestRequestDto {
	return dto.BacktestRequestDto{
		Symbol:              "BTCUSDT",
		AggregationInterval: "1h",
		StartTime:           backtestStart,
		EndTime:             backtestStart.Add(4 * time.Hour),
		InitialCapital:      decimal.NewFromInt(10000),
		PositionSizingMode:  "allIn",
	}
}

// replayRequestDto is the request as the queue later hands it to the replay, script already resolved.
func replayRequestDto() dto.BacktestRequestDto {
	requestDto := backtestRequestDto()
	requestDto.Script = "the script"

	return requestDto
}

// storedHourlyCandle builds a stored candle that many hours into the stretch.
func storedHourlyCandle(hour int, closePrice string) entities.KCandle {
	return entities.KCandle{
		Symbol:   "BTCUSDT",
		OpenTime: backtestStart.Add(time.Duration(hour) * time.Hour),
		Open:     decimal.RequireFromString(closePrice),
		High:     decimal.RequireFromString(closePrice),
		Low:      decimal.RequireFromString(closePrice),
		Close:    decimal.RequireFromString(closePrice),
	}
}

// signalsSaying builds one signal per candle, filed under the well-known key, as a signal-kind
// script would.
func signalsSaying(signals ...vo.SignalVo) []map[string]vo.IndicatorValueVo {
	perCandleIndicatorValues := make([]map[string]vo.IndicatorValueVo, 0, len(signals))
	for _, signal := range signals {
		perCandleIndicatorValues = append(perCandleIndicatorValues,
			map[string]vo.IndicatorValueVo{
				vo.SignalIndicatorKey: {Signal: signal},
			})
	}

	return perCandleIndicatorValues
}

func TestSubmitBacktest(t *testing.T) {
	t.Run("a well formed request is queued at once, named after its script, without reading any candle", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)

		runDto, err := fixture.backtestApplication.SubmitBacktest(
			t.Context(), backtestViewerID, namingStrategyScript(t, backtestStrategyScriptID), backtestRequestDto())

		require.NoError(t, err)
		assert.Equal(t, "queued", runDto.Status)
		assert.Equal(t, "spotScript", runDto.Kind)
		assert.Equal(t, 0, runDto.ProgressPercent)
		assert.Equal(t, "BTCUSDT", runDto.Symbol)
		assert.Equal(t, "1h", runDto.Interval)
		assert.Equal(t, "均線", runDto.SubjectName)
		require.NotNil(t, runDto.StrategyScriptID)
		assert.Equal(t, backtestStrategyScriptID, *runDto.StrategyScriptID)
		assert.Nil(t, runDto.Leverage)
		assert.Nil(t, runDto.TotalReturnRate)
		saved := fixture.runStore.savedRuns()
		require.Len(t, saved, 1)
		assert.Equal(t, backtestViewerID, saved[0].OwnerID)
		assert.Equal(t, "the script", saved[0].Request.SpotScript.Script)
	})

	t.Run("a very short stretch is still queued rather than replayed on the spot", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		requestDto := backtestRequestDto()
		requestDto.EndTime = backtestStart.Add(2 * time.Hour)

		runDto, err := fixture.backtestApplication.SubmitBacktest(
			t.Context(), backtestViewerID, namingStrategyScript(t, backtestStrategyScriptID), requestDto)

		require.NoError(t, err)
		assert.Equal(t, "queued", runDto.Status)
	})

	t.Run("an algorithm written in the request is queued with no script to name", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)

		runDto, err := fixture.backtestApplication.SubmitBacktest(
			t.Context(), backtestViewerID, carrying(t, "written here"), backtestRequestDto())

		require.NoError(t, err)
		assert.Nil(t, runDto.StrategyScriptID)
		assert.Equal(t, "", runDto.SubjectName)
		assert.Equal(t, "written here", fixture.runStore.savedRuns()[0].Request.SpotScript.Script)
	})

	t.Run("a stretch that ends before it starts is refused and nothing is queued", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		requestDto := backtestRequestDto()
		requestDto.StartTime = backtestStart.Add(10 * time.Hour)
		requestDto.EndTime = backtestStart

		_, err := fixture.backtestApplication.SubmitBacktest(
			t.Context(), backtestViewerID, namingStrategyScript(t, backtestStrategyScriptID), requestDto)

		assert.ErrorIs(t, err, domains.ErrBacktestValidation)
		assert.Empty(t, fixture.runStore.savedRuns())
	})

	t.Run("starting capital of zero is refused and nothing is queued", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		requestDto := backtestRequestDto()
		requestDto.InitialCapital = decimal.Zero

		_, err := fixture.backtestApplication.SubmitBacktest(
			t.Context(), backtestViewerID, namingStrategyScript(t, backtestStrategyScriptID), requestDto)

		assert.ErrorIs(t, err, domains.ErrBacktestValidation)
		assert.Empty(t, fixture.runStore.savedRuns())
	})

	t.Run("a knob nobody declared is refused by name and nothing is queued", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		requestDto := backtestRequestDto()
		requestDto.ParameterValues = []dto.StrategyScriptParameterValueDto{{Name: "不存在的旋鈕", Value: 3}}

		_, err := fixture.backtestApplication.SubmitBacktest(
			t.Context(), backtestViewerID, namingStrategyScript(t, backtestStrategyScriptID), requestDto)

		require.ErrorIs(t, err, domains.ErrBacktestValidation)
		assert.Contains(t, err.Error(), "不存在的旋鈕")
		assert.Empty(t, fixture.runStore.savedRuns())
	})

	t.Run("an owner at the limit is refused with the limit, and the request comes before the limit", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.runStore.activeCount = backtestRunOwnerActiveLimit

		_, err := fixture.backtestApplication.SubmitBacktest(
			t.Context(), backtestViewerID, namingStrategyScript(t, backtestStrategyScriptID), backtestRequestDto())

		require.ErrorIs(t, err, domains.ErrBacktestRunOwnerLimitReached)
		limit, limitReached := domains.BacktestRunOwnerLimit(err)
		assert.True(t, limitReached)
		assert.Equal(t, backtestRunOwnerActiveLimit, limit)
		assert.Contains(t, err.Error(), "你同時最多 3 筆回測在排隊或進行中，等其中一筆結束再送")
		assert.Empty(t, fixture.runStore.savedRuns())

		invalid := backtestRequestDto()
		invalid.InitialCapital = decimal.Zero
		_, invalidError := fixture.backtestApplication.SubmitBacktest(
			t.Context(), backtestViewerID, namingStrategyScript(t, backtestStrategyScriptID), invalid)

		assert.ErrorIs(t, invalidError, domains.ErrBacktestValidation)
		assert.NotErrorIs(t, invalidError, domains.ErrBacktestRunOwnerLimitReached)
	})

	t.Run("one below the limit is still queued", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.runStore.activeCount = backtestRunOwnerActiveLimit - 1

		_, err := fixture.backtestApplication.SubmitBacktest(
			t.Context(), backtestViewerID, namingStrategyScript(t, backtestStrategyScriptID), backtestRequestDto())

		require.NoError(t, err)
		assert.Len(t, fixture.runStore.savedRuns(), 1)
	})
}

func TestRunBacktest(t *testing.T) {
	t.Run("reads the stretch asked for, stopping at the hour still running", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.kCandleRepository.EXPECT().
			FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				_ context.Context, query domains.KCandleQueryDomain, limit int,
			) ([]entities.KCandle, error) {
				assert.Equal(t, "BTCUSDT", query.Symbol())
				assert.Equal(t, backtestStart, query.StartTime())
				assert.Equal(t, backtestStart.Add(4*time.Hour), query.EndTime())
				// Five hourly buckets plus one spare, sixty one-minute candles each.
				assert.Equal(t, 6*60, limit)
				return []entities.KCandle{
					storedHourlyCandle(0, "100"), storedHourlyCandle(1, "110"),
				}, nil
			})
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(signalsSaying(vo.SignalHold, vo.SignalHold), nil)

		_, err := fixture.backtestService.RunBacktest(t.Context(), replayRequestDto(), nil)

		assert.NoError(t, err)
	})

	t.Run("replays the script over the finished hours, oldest first", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{
				storedHourlyCandle(0, "100"),
				storedHourlyCandle(1, "110"),
				storedHourlyCandle(2, "120"),
			}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), "the script", gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				_ context.Context,
				_ string,
				resultType domains.IndicatorResultTypeDomain,
				kCandleVos []vo.KCandleVo,
				_ domains.StrategyScriptParametersDomain,
				_ domaininterface.IReplayProgressSink,
			) ([]map[string]vo.IndicatorValueVo, error) {
				// A replay always runs the script under the signal kind.
				assert.True(t, resultType.IsSignal())
				require.Len(t, kCandleVos, 3)
				assert.Equal(t, backtestStart.Unix(), kCandleVos[0].OpenTimeUnixSeconds)
				assert.Equal(t, 100.0, kCandleVos[0].Close)
				assert.Equal(t, 120.0, kCandleVos[2].Close)
				return signalsSaying(vo.SignalHold, vo.SignalHold, vo.SignalHold), nil
			})

		_, err := fixture.backtestService.RunBacktest(t.Context(), replayRequestDto(), nil)

		assert.NoError(t, err)
	})

	t.Run("hands back the report card, the trades and the curve together", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{
				storedHourlyCandle(0, "100"),
				storedHourlyCandle(1, "110"),
				storedHourlyCandle(2, "120"),
			}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(signalsSaying(vo.SignalBuy, vo.SignalSell, vo.SignalHold), nil)

		result, err := fixture.backtestService.RunBacktest(t.Context(), replayRequestDto(), nil)

		require.NoError(t, err)
		assert.Equal(t, "BTCUSDT", result.Symbol)
		assert.Equal(t, "1h", result.Interval)
		assert.Equal(t, backtestStart, result.StartTime)
		assert.Equal(t, backtestStart.Add(2*time.Hour), result.EndTime)
		assert.Equal(t, 3, result.UsedCandleCount)
		assert.Len(t, result.EquityCurve, 3)
		// Bought at 100 and sold at 110 makes 1,000; the account then sits in cash
		// through the rise to 120.
		require.Len(t, result.ClosedTrades, 1)
		assert.True(t, decimal.NewFromInt(1000).Equal(result.ClosedTrades[0].Profit),
			"profit was %s", result.ClosedTrades[0].Profit)
		assert.Equal(t, 1, result.Summary.PositionOpenCount)
		// One script cannot disagree with itself, so conflicts are always zero when replaying a
		// strategy script.
		assert.Equal(t, 0, result.Summary.ConflictedCandleCount)
	})

	t.Run("a strategy script that only ever holds reports no trades rather than a failure", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{
				storedHourlyCandle(0, "100"), storedHourlyCandle(1, "110"),
			}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(signalsSaying(vo.SignalHold, vo.SignalHold), nil)

		result, err := fixture.backtestService.RunBacktest(t.Context(), replayRequestDto(), nil)

		require.NoError(t, err)
		assert.Empty(t, result.ClosedTrades)
		assert.Equal(t, 0, result.Summary.PositionOpenCount)
		assert.Nil(t, result.Summary.WinRate)
		assert.Len(t, result.EquityCurve, 2)
	})

	t.Run("a stretch holding one finished hour is refused", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{storedHourlyCandle(0, "100")}, nil)

		_, err := fixture.backtestService.RunBacktest(t.Context(), replayRequestDto(), nil)

		assert.ErrorIs(t, err, domains.ErrBacktestValidation)
	})

	t.Run("a stretch holding nothing is refused", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, nil)

		_, err := fixture.backtestService.RunBacktest(t.Context(), replayRequestDto(), nil)

		assert.ErrorIs(t, err, domains.ErrBacktestValidation)
	})

	t.Run("a script that could not run brings the replay down", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{
				storedHourlyCandle(0, "100"), storedHourlyCandle(1, "110"),
			}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, domains.ErrIndicatorScriptFailed)

		_, err := fixture.backtestService.RunBacktest(t.Context(), replayRequestDto(), nil)

		assert.ErrorIs(t, err, domains.ErrIndicatorScriptFailed)
	})

	t.Run("a knob the script reads but nobody declared is reported by name", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return([]entities.KCandle{
				storedHourlyCandle(0, "100"), storedHourlyCandle(1, "110"),
			}, nil)
		fixture.indicatorScriptProxy.EXPECT().
			ExecuteForEachCandle(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, domains.UndeclaredParameter("period"))

		_, err := fixture.backtestService.RunBacktest(t.Context(), replayRequestDto(), nil)

		parameterName, isUndeclared := domains.UndeclaredParameterName(err)
		assert.True(t, isUndeclared)
		assert.Equal(t, "period", parameterName)
	})

	t.Run("storage refusing to answer is passed on as it came", func(t *testing.T) {
		fixture := newBacktestUnderTest(t)
		storageError := errors.New("storage is down")
		fixture.kCandleRepository.EXPECT().FindInRange(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(nil, storageError)

		_, err := fixture.backtestService.RunBacktest(t.Context(), replayRequestDto(), nil)

		assert.ErrorIs(t, err, storageError)
	})
}

func TestSubmitBacktestWalksTheSameGatesACalculationWalks(t *testing.T) {
	// Both use cases share the gates through the same service; this proves it on this path.
	t.Run("somebody else's published strategy script is queued", func(t *testing.T) {
		fixture := newBacktestGateUnderTest(t, true)

		_, err := fixture.backtestApplication.SubmitBacktest(
			t.Context(), backtestStrangerID, namingStrategyScript(t, backtestStrategyScriptID), backtestRequestDto())

		require.NoError(t, err)
		require.Len(t, fixture.runStore.savedRuns(), 1)
		assert.Equal(t, "the script", fixture.runStore.savedRuns()[0].Request.SpotScript.Script)
	})

	t.Run("somebody else's unpublished strategy script is not there", func(t *testing.T) {
		fixture := newBacktestGateUnderTest(t, false)

		_, err := fixture.backtestApplication.SubmitBacktest(
			t.Context(), backtestStrangerID, namingStrategyScript(t, backtestStrategyScriptID), backtestRequestDto())

		require.ErrorIs(t, err, domains.ErrStrategyScriptNotFound)
		assert.Empty(t, fixture.runStore.savedRuns())
	})
}

// backtestStrangerID is somebody who does not own the strategy script being replayed.
const backtestStrangerID = uint(2)

// newBacktestGateUnderTest replays somebody else's strategy script, published to the marketplace or
// not per the argument.
func newBacktestGateUnderTest(t *testing.T, isPublished bool) backtestUnderTest {
	controller := gomock.NewController(t)
	kCandleRepository := mocks.NewMockIKCandleRepository(controller)
	indicatorScriptProxy := mocks.NewMockIIndicatorScriptProxy(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(backtestNow).AnyTimes()

	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
	strategyScriptRepository.EXPECT().FindOne(gomock.Any(), backtestStrategyScriptID).
		Return(entities.StrategyScript{
			ID: backtestStrategyScriptID, OwnerID: backtestViewerID, Script: "the script",
		}, nil).AnyTimes()

	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)
	if isPublished {
		publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), backtestStrategyScriptID).
			Return(entities.PublishedStrategyScript{StrategyScriptID: backtestStrategyScriptID}, nil).AnyTimes()
	} else {
		publishedStrategyScriptRepository.EXPECT().FindOne(gomock.Any(), backtestStrategyScriptID).
			Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished).AnyTimes()
	}

	backtestService := service.NewBacktestService(
		kCandleRepository, indicatorScriptProxy, clockProxy, queryMaxResults, 0)
	runStore := newBacktestRunStore(controller)

	return backtestUnderTest{
		backtestApplication: application.NewBacktestApplication(
			service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository),
			backtestService, nil,
			newQueueingBacktestRunService(t, controller, runStore, backtestService, nil)),
		backtestService:      backtestService,
		runStore:             runStore,
		kCandleRepository:    kCandleRepository,
		indicatorScriptProxy: indicatorScriptProxy,
	}
}

// namingStrategyScript runs the saved strategy script with this identifier.
func namingStrategyScript(t *testing.T, strategyScriptID uint) domains.RunSubjectDomain {
	t.Helper()

	runSubjectDomain, subjectError := domains.NewRunSubjectDomain(strategyScriptID, "", "", nil)
	require.NoError(t, subjectError)

	return runSubjectDomain
}

// carrying is how these tests say "run this algorithm, which was never saved".
func carrying(t *testing.T, script string) domains.RunSubjectDomain {
	t.Helper()

	runSubjectDomain, subjectError := domains.NewRunSubjectDomain(0, script, "", nil)
	require.NoError(t, subjectError)

	return runSubjectDomain
}
