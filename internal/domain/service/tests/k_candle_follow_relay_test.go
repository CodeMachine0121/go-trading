package service_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// relayTestBed is a Taiwan roster whose source and snapshot store are both in the test's hands.
type relayTestBed struct {
	service        *service.KCandleFollowService
	sourceFeed     chan vo.LiveKCandleVo
	sourceAsked    chan string
	sourceEnded    chan string
	savedSnapshots chan entities.LiveKCandleSnapshot
	mutex          sync.Mutex
	snapshots      []entities.LiveKCandleSnapshot
}

func newRelayTestBed(t *testing.T) *relayTestBed {
	t.Helper()

	return newRelayTestBedGoingQuietAfter(t, time.Hour)
}

// newRelayTestBedGoingQuietAfter treats a line silent for quietTimeout as stalled.
func newRelayTestBedGoingQuietAfter(t *testing.T, quietTimeout time.Duration) *relayTestBed {
	t.Helper()

	mockController := gomock.NewController(t)
	testBed := &relayTestBed{
		sourceFeed:     make(chan vo.LiveKCandleVo, 8),
		sourceAsked:    make(chan string, 16),
		sourceEnded:    make(chan string, 16),
		savedSnapshots: make(chan entities.LiveKCandleSnapshot, 16),
	}
	liveMarketDataProxy := mocks.NewMockILiveMarketDataProxy(mockController)
	liveMarketDataProxy.EXPECT().FollowKCandles(gomock.Any(), gomock.Any()).DoAndReturn(
		func(executionContext context.Context, channel vo.LiveFollowChannelVo) (<-chan vo.LiveKCandleVo, error) {
			testBed.sourceAsked <- channel.Key
			go func() {
				<-executionContext.Done()
				testBed.sourceEnded <- channel.Key
			}()

			return testBed.sourceFeed, nil
		}).AnyTimes()
	snapshotRepository := mocks.NewMockILiveKCandleSnapshotRepository(mockController)
	snapshotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, snapshot entities.LiveKCandleSnapshot) error {
			testBed.savedSnapshots <- snapshot

			return nil
		}).AnyTimes()
	snapshotRepository.EXPECT().FindBySymbols(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, []string) ([]entities.LiveKCandleSnapshot, error) {
			testBed.mutex.Lock()
			defer testBed.mutex.Unlock()

			return append([]entities.LiveKCandleSnapshot{}, testBed.snapshots...), nil
		}).AnyTimes()
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	kCandleRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Return(entities.KCandle{}, nil).AnyTimes()
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	watched := []entities.TradingSymbol{{Symbol: "2330", Market: string(vo.MarketTaiwanStock), IsWatched: true}}
	tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(watched, nil).AnyTimes()
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").Return(watched[0], true, nil).AnyTimes()
	// Moves a millisecond each read, so the viewer throttle sees time pass between updates.
	clockMutex := sync.Mutex{}
	clockReading := taipeiFollowAt(t, "2026-09-08T10:00:00+08:00")
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().DoAndReturn(func() time.Time {
		clockMutex.Lock()
		defer clockMutex.Unlock()
		clockReading = clockReading.Add(time.Millisecond)

		return clockReading
	}).AnyTimes()

	testBed.service = service.NewKCandleFollowService(
		liveMarketDataProxy, kCandleRepository, tradingSymbolRepository, clockProxy,
		followMarketCatalog(), time.Nanosecond, quietTimeout, time.Hour, snapshotRepository, 5*time.Millisecond)
	t.Cleanup(testBed.service.Stop)

	return testBed
}

func (testBed *relayTestBed) theReplicaOnDutySaw(closePrice string, observedAt time.Time) {
	testBed.mutex.Lock()
	defer testBed.mutex.Unlock()

	testBed.snapshots = []entities.LiveKCandleSnapshot{{
		Symbol: "2330", OpenTime: time.Date(2026, 9, 8, 1, 59, 0, 0, time.UTC),
		Open: decimal.RequireFromString("1000"), High: decimal.RequireFromString("1010"),
		Low: decimal.RequireFromString("990"), Close: decimal.RequireFromString(closePrice),
		Volume: decimal.RequireFromString("12"), ObservedAt: observedAt,
	}}
}

func taiwanLiveKCandle(closePrice string) vo.LiveKCandleVo {
	return vo.LiveKCandleVo{
		Symbol: "2330", OpenTime: time.Date(2026, 9, 8, 1, 59, 0, 0, time.UTC),
		Open: decimal.RequireFromString("1000"), High: decimal.RequireFromString("1010"),
		Low: decimal.RequireFromString("990"), Close: decimal.RequireFromString(closePrice),
		Volume: decimal.RequireFromString("12"),
	}
}

func nextOf[Value any](t *testing.T, values chan Value) Value {
	t.Helper()

	select {
	case value := <-values:
		return value
	case <-time.After(2 * time.Second):
		t.Fatal("nothing arrived in time")

		var nothing Value

		return nothing
	}
}

func TestTheReplicaOnDutyPassesEveryRosteredCandleOnToTheOthers(t *testing.T) {
	testBed := newRelayTestBed(t)
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	nextOf(t, testBed.sourceAsked)

	testBed.sourceFeed <- taiwanLiveKCandle("1005")

	saved := nextOf(t, testBed.savedSnapshots)
	assert.Equal(t, "2330", saved.Symbol)
	assert.Equal(t, "1005", saved.Close.String())
	assert.False(t, saved.Closed)
}

func TestAReplicaOffDutyShowsItsViewersWhatTheReplicaOnDutySaw(t *testing.T) {
	testBed := newRelayTestBed(t)
	testBed.theReplicaOnDutySaw("1007", time.Date(2026, 9, 8, 2, 0, 0, 0, time.UTC))
	require.NoError(t, testBed.service.RefreshRelayedFollows(t.Context()))

	updates, watchError := testBed.service.WatchKCandles(t.Context(), "2330")
	require.NoError(t, watchError)

	update := firstCandleUpdateFrom(t, updates)
	assert.Equal(t, "1007", update.KCandle.Close.String())
	assert.Empty(t, testBed.sourceAsked, "off duty, the source is never asked: its places belong to the replica on duty")
}

func TestARelayPassesOnlyWhatTheReplicaOnDutyHeardAgain(t *testing.T) {
	testBed := newRelayTestBed(t)
	firstSeenAt := time.Date(2026, 9, 8, 2, 0, 0, 0, time.UTC)
	testBed.theReplicaOnDutySaw("1007", firstSeenAt)
	require.NoError(t, testBed.service.RefreshRelayedFollows(t.Context()))
	updates, watchError := testBed.service.WatchKCandles(t.Context(), "2330")
	require.NoError(t, watchError)
	firstCandleUpdateFrom(t, updates)

	testBed.theReplicaOnDutySaw("1009", firstSeenAt.Add(5*time.Second))

	assert.Equal(t, "1009", firstCandleUpdateFrom(t, updates).KCandle.Close.String())
}

func TestTakingDutySwapsARelayedRosterForTheSource(t *testing.T) {
	testBed := newRelayTestBed(t)
	require.NoError(t, testBed.service.RefreshRelayedFollows(t.Context()))
	assert.Empty(t, testBed.sourceAsked)

	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Equal(t, vo.NewLiveFollowChannelVo(vo.MarketTaiwanStock, []string{"2330"}).Key, nextOf(t, testBed.sourceAsked))
	assert.Equal(t, 1, testBed.service.FollowedSymbolCount())
}

func TestLeavingDutyLetsGoOfTheSourceAtOnce(t *testing.T) {
	testBed := newRelayTestBed(t)
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	channelKey := nextOf(t, testBed.sourceAsked)

	require.NoError(t, testBed.service.RefreshRelayedFollows(t.Context()))

	assert.Equal(t, channelKey, nextOf(t, testBed.sourceEnded), "the place goes back before the next replica can take it")
	assert.Equal(t, 1, testBed.service.FollowedSymbolCount())
}

// firstCandleUpdateFrom skips status updates until a candle arrives.
func firstCandleUpdateFrom(t *testing.T, updates <-chan dto.KCandleFollowUpdateDto) dto.KCandleFollowUpdateDto {
	t.Helper()

	for {
		update := firstUpdateFrom(t, updates)
		if update.Status == dto.KCandleFollowStatusForming || update.Status == dto.KCandleFollowStatusClosed {
			return update
		}
	}
}

func TestARelayedViewerIsToldWhenTheReplicaOnDutyGoesSilent(t *testing.T) {
	testBed := newRelayTestBedGoingQuietAfter(t, 50*time.Millisecond)
	testBed.theReplicaOnDutySaw("1007", time.Date(2026, 9, 8, 2, 0, 0, 0, time.UTC))
	require.NoError(t, testBed.service.RefreshRelayedFollows(t.Context()))
	updates, watchError := testBed.service.WatchKCandles(t.Context(), "2330")
	require.NoError(t, watchError)
	firstCandleUpdateFrom(t, updates)

	// The snapshot is never refreshed again, as when the replica on duty dies.
	deadline := time.After(2 * time.Second)
	for {
		select {
		case update := <-updates:
			if update.Status == dto.KCandleFollowStatusStalled {
				return
			}
		case <-deadline:
			t.Fatal("a relay that kept repeating an old candle hid that the replica on duty went silent")
		}
	}
}
