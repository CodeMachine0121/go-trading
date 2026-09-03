package service_test

import (
	"context"
	"errors"
	"strconv"
	"sync/atomic"
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

// followSettleTime is how long a lifecycle change is given to take effect. Watching
// and leaving both hand work to a goroutine, so a test that asked the instant after
// would be asking too early.
const followSettleTime = 200 * time.Millisecond

// liveFeed is one market's feed under the test's control: it decides what the
// source reports and when the feed ends.
type liveFeed struct {
	kCandles chan vo.LiveKCandleVo
}

func newLiveFeed() *liveFeed {
	return &liveFeed{kCandles: make(chan vo.LiveKCandleVo, 8)}
}

func (feed *liveFeed) report(liveKCandle vo.LiveKCandleVo) {
	feed.kCandles <- liveKCandle
}

func (feed *liveFeed) end() {
	close(feed.kCandles)
}

func liveKCandleAt(openTime time.Time, closePrice string, closed bool) vo.LiveKCandleVo {
	return vo.LiveKCandleVo{
		Symbol:   "BTCUSDT",
		OpenTime: openTime,
		Open:     decimal.RequireFromString("100"),
		High:     decimal.RequireFromString("120"),
		Low:      decimal.RequireFromString("90"),
		Close:    decimal.RequireFromString(closePrice),
		Volume:   decimal.RequireFromString("1"),
		Closed:   closed,
	}
}

// followTestBed wires a follow service whose every timing rule is small enough for a
// test to outrun, and whose feed the test hands out itself.
type followTestBed struct {
	service        *service.KCandleFollowService
	kCandleReposit *mocks.MockIKCandleRepository
	feedsRequested chan string
}

func newFollowTestBed(t *testing.T, feedFor func(symbol string) (<-chan vo.LiveKCandleVo, error)) *followTestBed {
	return newFollowTestBedWith(t, time.Nanosecond, time.Hour, feedFor)
}

func newFollowTestBedWithQuietTimeout(
	t *testing.T, quietTimeout time.Duration,
	feedFor func(symbol string) (<-chan vo.LiveKCandleVo, error),
) *followTestBed {
	return newFollowTestBedWith(t, time.Nanosecond, quietTimeout, feedFor)
}

func newFollowTestBedWithCeiling(
	t *testing.T, updateIntervalCeiling time.Duration,
	feedFor func(symbol string) (<-chan vo.LiveKCandleVo, error),
) *followTestBed {
	return newFollowTestBedWith(t, updateIntervalCeiling, time.Hour, feedFor)
}

func newFollowTestBedWith(
	t *testing.T, updateIntervalCeiling time.Duration, quietTimeout time.Duration,
	feedFor func(symbol string) (<-chan vo.LiveKCandleVo, error),
) *followTestBed {
	t.Helper()

	mockController := gomock.NewController(t)
	liveMarketDataProxy := mocks.NewMockILiveMarketDataProxy(mockController)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)

	// The clock moves on every reading. A frozen one would mean no time ever passes,
	// and the ceiling below would then hold back every forming candle — which is the
	// throttle working correctly, but it would be all these tests ever measured.
	// What the throttle does with a given moment is pinned by the domain's own tests.
	var readings atomic.Int64
	clockProxy.EXPECT().Now().DoAndReturn(func() time.Time {
		return followStartedAt.Add(time.Duration(readings.Add(1)) * time.Second)
	}).AnyTimes()

	testBed := &followTestBed{
		kCandleReposit: kCandleRepository,
		feedsRequested: make(chan string, 16),
	}

	liveMarketDataProxy.EXPECT().FollowKCandles(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, symbol string) (<-chan vo.LiveKCandleVo, error) {
			// Dropped rather than blocked: a retry loop that outruns the test must not
			// be able to wedge the follow it is being watched through.
			select {
			case testBed.feedsRequested <- symbol:
			default:
			}

			return feedFor(symbol)
		}).AnyTimes()

	// The ceiling is zero-ish so nothing is throttled away in a lifecycle test; the
	// throttle itself is pinned by the domain's tests. The quiet threshold is an hour
	// so that silence is never mistaken for death here — the one test that is about
	// death sets its own.
	testBed.service = service.NewKCandleFollowService(
		liveMarketDataProxy, kCandleRepository, clockProxy,
		updateIntervalCeiling, quietTimeout, 10*time.Millisecond,
	)
	t.Cleanup(testBed.service.Stop)

	return testBed
}

var followStartedAt = time.Date(2026, 9, 3, 9, 7, 0, 0, time.UTC)

// followOpenTime is the candle being followed: the one still running at 09:07, so
// it sits on a five-minute mark and is not in the future.
var followOpenTime = time.Date(2026, 9, 3, 9, 5, 0, 0, time.UTC)

// The unit of following is the market, not the person looking at it: ten viewers on
// one symbol are one follow, because the market has only one answer.
func TestOneFollowPerSymbolNoMatterHowManyAreWatching(t *testing.T) {
	feed := newLiveFeed()
	testBed := newFollowTestBed(t, func(string) (<-chan vo.LiveKCandleVo, error) {
		return feed.kCandles, nil
	})

	firstViewer, cancelFirstViewer := context.WithCancel(context.Background())
	defer cancelFirstViewer()
	_, firstError := testBed.service.WatchKCandles(firstViewer, "BTCUSDT")
	require.NoError(t, firstError)
	assert.Equal(t, 1, testBed.service.FollowedSymbolCount())

	secondViewer, cancelSecondViewer := context.WithCancel(context.Background())
	defer cancelSecondViewer()
	secondUpdates, secondError := testBed.service.WatchKCandles(secondViewer, "BTCUSDT")
	require.NoError(t, secondError)

	assert.Equal(t, 1, testBed.service.FollowedSymbolCount(),
		"第二個觀看者不該讓系統跟第二份")

	firstUpdates, _ := testBed.service.WatchKCandles(firstViewer, "BTCUSDT")
	feed.report(liveKCandleAt(followOpenTime, "115", false))

	assert.Equal(t, "115", (<-firstUpdates).KCandle.Close.String())
	assert.Equal(t, "115", (<-secondUpdates).KCandle.Close.String(),
		"一份跟盤的答案要送到每一個在看的人手上")
}

// Following a market nobody is looking at buys nothing the five-minute round would
// not deliver anyway, so the last viewer leaving is what ends it.
func TestTheFollowEndsOnlyWhenTheLastViewerLeaves(t *testing.T) {
	feed := newLiveFeed()
	testBed := newFollowTestBed(t, func(string) (<-chan vo.LiveKCandleVo, error) {
		return feed.kCandles, nil
	})

	firstViewer, firstViewerLeaves := context.WithCancel(context.Background())
	secondViewer, secondViewerLeaves := context.WithCancel(context.Background())
	firstUpdates, _ := testBed.service.WatchKCandles(firstViewer, "BTCUSDT")
	_, _ = testBed.service.WatchKCandles(secondViewer, "BTCUSDT")

	secondViewerLeaves()
	time.Sleep(followSettleTime)

	assert.Equal(t, 1, testBed.service.FollowedSymbolCount(), "還有人在看就該繼續跟")
	feed.report(liveKCandleAt(followOpenTime, "115", false))
	assert.Eventually(t, func() bool { return len(firstUpdates) > 0 }, time.Second, 10*time.Millisecond,
		"剩下那位觀看者應照常收到更新")

	firstViewerLeaves()

	assert.Eventually(t, func() bool { return testBed.service.FollowedSymbolCount() == 0 },
		time.Second, 10*time.Millisecond, "最後一個觀看者離開後就該停止跟盤")
}

// Following answers "who is looking at what", the watchlist answers "which markets
// are worth keeping data for". A symbol absent from the second is still followable.
func TestASymbolOffTheWatchlistIsStillFollowed(t *testing.T) {
	feed := newLiveFeed()
	testBed := newFollowTestBed(t, func(string) (<-chan vo.LiveKCandleVo, error) {
		return feed.kCandles, nil
	})

	viewer, cancelViewer := context.WithCancel(context.Background())
	defer cancelViewer()
	updates, watchError := testBed.service.WatchKCandles(viewer, "SOLUSDT")
	require.NoError(t, watchError)

	assert.Equal(t, "SOLUSDT", <-testBed.feedsRequested)
	feed.report(vo.LiveKCandleVo{Symbol: "SOLUSDT", OpenTime: followOpenTime})
	update := <-updates
	assert.Equal(t, "SOLUSDT", update.Symbol)
}

// Changing symbol is leaving one market and joining another; the viewer must stop
// hearing about the one they left.
func TestChangingSymbolLeavesTheOldMarketBehind(t *testing.T) {
	feeds := map[string]*liveFeed{"BTCUSDT": newLiveFeed(), "ETHUSDT": newLiveFeed()}
	testBed := newFollowTestBed(t, func(symbol string) (<-chan vo.LiveKCandleVo, error) {
		return feeds[symbol].kCandles, nil
	})

	firstViewing, stopViewingBitcoin := context.WithCancel(context.Background())
	_, _ = testBed.service.WatchKCandles(firstViewing, "BTCUSDT")

	stopViewingBitcoin()
	secondViewing, cancelSecondViewing := context.WithCancel(context.Background())
	defer cancelSecondViewing()
	etherUpdates, _ := testBed.service.WatchKCandles(secondViewing, "ETHUSDT")

	assert.Eventually(t, func() bool { return testBed.service.FollowedSymbolCount() == 1 },
		time.Second, 10*time.Millisecond, "只該剩下 ETHUSDT 一份跟盤")

	feeds["ETHUSDT"].report(vo.LiveKCandleVo{Symbol: "ETHUSDT", OpenTime: followOpenTime})
	update := <-etherUpdates
	assert.Equal(t, "ETHUSDT", update.Symbol, "他不該再收到 BTCUSDT 的更新")
}

// Arriving mid-candle must not mean an empty chart until the market next moves.
func TestAViewerArrivingMidCandleIsGivenTheShapeSoFar(t *testing.T) {
	feed := newLiveFeed()
	testBed := newFollowTestBed(t, func(string) (<-chan vo.LiveKCandleVo, error) {
		return feed.kCandles, nil
	})

	firstViewer, cancelFirstViewer := context.WithCancel(context.Background())
	defer cancelFirstViewer()
	firstUpdates, _ := testBed.service.WatchKCandles(firstViewer, "BTCUSDT")
	feed.report(liveKCandleAt(followOpenTime, "115", false))
	require.Equal(t, "115", (<-firstUpdates).KCandle.Close.String())

	lateViewer, cancelLateViewer := context.WithCancel(context.Background())
	defer cancelLateViewer()
	lateUpdates, _ := testBed.service.WatchKCandles(lateViewer, "BTCUSDT")

	select {
	case update := <-lateUpdates:
		assert.Equal(t, dto.KCandleFollowStatusForming, update.Status)
		assert.Equal(t, "115", update.KCandle.Close.String())
	case <-time.After(time.Second):
		t.Fatal("後來加入的觀看者沒有立刻收到目前進行中的那一根")
	}
}

// Nothing has been reported yet, so there is nothing to hand over — and that is a
// successful join, not a failure.
func TestJoiningBeforeTheMarketHasTradedHandsOverNothingAndStillSucceeds(t *testing.T) {
	feed := newLiveFeed()
	testBed := newFollowTestBed(t, func(string) (<-chan vo.LiveKCandleVo, error) {
		return feed.kCandles, nil
	})

	viewer, cancelViewer := context.WithCancel(context.Background())
	defer cancelViewer()
	updates, watchError := testBed.service.WatchKCandles(viewer, "BTCUSDT")

	require.NoError(t, watchError)
	assert.Empty(t, updates, "這五分鐘還沒有成交，就不該有任何一根被送出")
}

// A candle's last word is stored the moment it is spoken; a shape that will still
// move is shown and never stored.
func TestOnlyAClosedCandleIsStored(t *testing.T) {
	feed := newLiveFeed()
	testBed := newFollowTestBed(t, func(string) (<-chan vo.LiveKCandleVo, error) {
		return feed.kCandles, nil
	})

	stored := make(chan entities.KCandle, 4)
	testBed.kCandleReposit.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, kCandle entities.KCandle) (entities.KCandle, error) {
			stored <- kCandle

			return kCandle, nil
		}).AnyTimes()

	viewer, cancelViewer := context.WithCancel(context.Background())
	defer cancelViewer()
	updates, _ := testBed.service.WatchKCandles(viewer, "BTCUSDT")

	feed.report(liveKCandleAt(followOpenTime, "115", false))
	assert.Equal(t, dto.KCandleFollowStatusForming, (<-updates).Status)

	feed.report(liveKCandleAt(followOpenTime, "118", true))
	assert.Equal(t, dto.KCandleFollowStatusClosed, (<-updates).Status)

	select {
	case kCandle := <-stored:
		assert.Equal(t, "118", kCandle.Close.String(), "存下來的該是走完那一刻的最終數字")
	case <-time.After(time.Second):
		t.Fatal("走完的那一根沒有被存入")
	}
	assert.Empty(t, stored, "進行中的那一根不該被存入")
}

// The ordinary K candle rules apply to a candle arriving live exactly as they do to
// a fetched one — and a candle breaking one ends itself, not the follow.
func TestACandleBreakingARuleIsSkippedAndTheFollowCarriesOn(t *testing.T) {
	feed := newLiveFeed()
	testBed := newFollowTestBed(t, func(string) (<-chan vo.LiveKCandleVo, error) {
		return feed.kCandles, nil
	})

	stored := make(chan entities.KCandle, 4)
	testBed.kCandleReposit.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, kCandle entities.KCandle) (entities.KCandle, error) {
			stored <- kCandle

			return kCandle, nil
		}).AnyTimes()

	viewer, cancelViewer := context.WithCancel(context.Background())
	defer cancelViewer()
	updates, _ := testBed.service.WatchKCandles(viewer, "BTCUSDT")

	brokenKCandle := liveKCandleAt(followOpenTime.Add(-10*time.Minute), "118", true)
	brokenKCandle.High = decimal.RequireFromString("80")
	brokenKCandle.Low = decimal.RequireFromString("90")
	feed.report(brokenKCandle)
	<-updates

	assert.Empty(t, stored, "最高價低於最低價的那一根不該被存入")

	goodKCandle := liveKCandleAt(followOpenTime.Add(-5*time.Minute), "118", true)
	feed.report(goodKCandle)
	<-updates

	select {
	case kCandle := <-stored:
		assert.Equal(t, goodKCandle.OpenTime, kCandle.OpenTime.UTC(),
			"違規的那一根之後，跟盤仍應照常存入下一根")
	case <-time.After(time.Second):
		t.Fatal("違規的那一根把整份跟盤帶垮了")
	}
}

// A picture that stopped updating but still looks normal is more dangerous than one
// that says it stopped, because the viewer acts on it.
func TestTheViewerIsToldWhenTheFeedStops(t *testing.T) {
	feed := newLiveFeed()
	testBed := newFollowTestBed(t, func(string) (<-chan vo.LiveKCandleVo, error) {
		return feed.kCandles, nil
	})

	viewer, cancelViewer := context.WithCancel(context.Background())
	defer cancelViewer()
	updates, _ := testBed.service.WatchKCandles(viewer, "BTCUSDT")

	feed.end()

	select {
	case update := <-updates:
		assert.Equal(t, dto.KCandleFollowStatusStalled, update.Status)
	case <-time.After(time.Second):
		t.Fatal("跟盤停了卻沒有告訴觀看者")
	}
}

// Arriving during an outage must not look like arriving during a quiet market: the
// last candle is worth handing over, but on its own it would look live.
func TestAViewerArrivingWhileStalledIsHandedTheLastCandleAndTheBadNews(t *testing.T) {
	feed := newLiveFeed()
	feedsHandedOut := 0
	testBed := newFollowTestBed(t, func(string) (<-chan vo.LiveKCandleVo, error) {
		feedsHandedOut++
		if feedsHandedOut > 1 {
			// Never answers again, so the follow stays stalled for the rest of the test.
			return make(chan vo.LiveKCandleVo), nil
		}

		return feed.kCandles, nil
	})

	firstViewer, cancelFirstViewer := context.WithCancel(context.Background())
	defer cancelFirstViewer()
	firstUpdates, _ := testBed.service.WatchKCandles(firstViewer, "BTCUSDT")
	feed.report(liveKCandleAt(followOpenTime, "115", false))
	require.Equal(t, "115", (<-firstUpdates).KCandle.Close.String())

	feed.end()
	require.Equal(t, dto.KCandleFollowStatusStalled, (<-firstUpdates).Status)

	lateViewer, cancelLateViewer := context.WithCancel(context.Background())
	defer cancelLateViewer()
	lateUpdates, _ := testBed.service.WatchKCandles(lateViewer, "BTCUSDT")

	firstSeen := <-lateUpdates
	assert.Equal(t, dto.KCandleFollowStatusForming, firstSeen.Status,
		"交給新來的人的該是最後那一根真的 K 線，不是一根全是零的")
	assert.Equal(t, "115", firstSeen.KCandle.Close.String())

	select {
	case update := <-lateUpdates:
		assert.Equal(t, dto.KCandleFollowStatusStalled, update.Status,
			"跟不動期間才加入的人必須知道畫面不是活的")
	case <-time.After(time.Second):
		t.Fatal("跟不動期間加入的觀看者沒有被告知即時更新已停止")
	}
}

// The source refusing outright is the same news to a viewer as a feed that dropped,
// and it must not stop the service from trying again.
func TestASourceThatRefusesIsReportedAndRetried(t *testing.T) {
	var attempts atomic.Int32
	testBed := newFollowTestBed(t, func(string) (<-chan vo.LiveKCandleVo, error) {
		attempts.Add(1)

		return nil, errors.New("行情來源拒絕連線")
	})

	viewer, cancelViewer := context.WithCancel(context.Background())
	defer cancelViewer()
	updates, watchError := testBed.service.WatchKCandles(viewer, "BTCUSDT")

	require.NoError(t, watchError, "來源連不上不該讓觀看者連跟都跟不上")
	assert.Equal(t, dto.KCandleFollowStatusStalled, (<-updates).Status)
	assert.Eventually(t, func() bool { return attempts.Load() >= 2 }, time.Second, 10*time.Millisecond,
		"連不上時應該自己重試，不需要人介入")
}

// A connection that stays open but stops delivering is how this feed usually dies.
// The viewer must be told, without the source ever saying anything at all.
func TestAFeedThatGoesSilentIsTreatedAsStopped(t *testing.T) {
	// Nothing is ever reported down this feed, and it never ends either.
	silentFeed := make(chan vo.LiveKCandleVo)
	testBed := newFollowTestBedWithQuietTimeout(t, 20*time.Millisecond,
		func(string) (<-chan vo.LiveKCandleVo, error) { return silentFeed, nil })

	viewer, cancelViewer := context.WithCancel(context.Background())
	defer cancelViewer()
	updates, _ := testBed.service.WatchKCandles(viewer, "BTCUSDT")

	select {
	case update := <-updates:
		assert.Equal(t, dto.KCandleFollowStatusStalled, update.Status,
			"通道還連著卻不再送資料，觀看者仍必須知道畫面不是活的")
	case <-time.After(2 * time.Second):
		t.Fatal("安靜的通道沒有被當成跟不動")
	}
}

// Storage refusing one candle is not a reason to stop showing the market: the
// five-minute round will store it, and the viewer needs the picture either way.
func TestACandleThatCannotBeStoredDoesNotEndTheFollow(t *testing.T) {
	feed := newLiveFeed()
	testBed := newFollowTestBed(t, func(string) (<-chan vo.LiveKCandleVo, error) {
		return feed.kCandles, nil
	})
	testBed.kCandleReposit.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(entities.KCandle{}, errors.New("資料庫寫不進去")).AnyTimes()

	viewer, cancelViewer := context.WithCancel(context.Background())
	defer cancelViewer()
	updates, _ := testBed.service.WatchKCandles(viewer, "BTCUSDT")

	feed.report(liveKCandleAt(followOpenTime, "118", true))
	require.Equal(t, dto.KCandleFollowStatusClosed, (<-updates).Status)

	feed.report(liveKCandleAt(followOpenTime.Add(5*time.Minute), "121", false))
	select {
	case update := <-updates:
		assert.Equal(t, dto.KCandleFollowStatusForming, update.Status,
			"存不進去之後，跟盤仍應照常把行情送給觀看者")
	case <-time.After(time.Second):
		t.Fatal("一次存入失敗把整份跟盤帶垮了")
	}
}

// Shutting down and a viewer walking away can happen at the same moment, and
// neither may be left holding a follow the other already ended.
func TestAViewerLeavingAfterShutdownChangesNothing(t *testing.T) {
	feed := newLiveFeed()
	testBed := newFollowTestBed(t, func(string) (<-chan vo.LiveKCandleVo, error) {
		return feed.kCandles, nil
	})

	viewer, viewerLeaves := context.WithCancel(context.Background())
	_, _ = testBed.service.WatchKCandles(viewer, "BTCUSDT")
	testBed.service.Stop()

	viewerLeaves()
	time.Sleep(followSettleTime)

	assert.Equal(t, 0, testBed.service.FollowedSymbolCount())
}

// A quiet threshold left unset must not mean "check the silence constantly", which
// is what a zero interval would ask for.
func TestAnUnsetQuietThresholdStillFollows(t *testing.T) {
	feed := newLiveFeed()
	testBed := newFollowTestBedWithQuietTimeout(t, 0, func(string) (<-chan vo.LiveKCandleVo, error) {
		return feed.kCandles, nil
	})

	viewer, cancelViewer := context.WithCancel(context.Background())
	defer cancelViewer()
	updates, watchError := testBed.service.WatchKCandles(viewer, "BTCUSDT")
	require.NoError(t, watchError)

	feed.report(liveKCandleAt(followOpenTime, "115", false))
	select {
	case update := <-updates:
		assert.Equal(t, dto.KCandleFollowStatusForming, update.Status)
	case <-time.After(time.Second):
		t.Fatal("門檻沒設定就跟不動了")
	}
}

// The throttle is the domain's rule; what this pins is that the follow actually
// asks it, rather than forwarding everything the market says.
func TestTheFollowHoldsBackWhatTheThrottleRefuses(t *testing.T) {
	feed := newLiveFeed()
	testBed := newFollowTestBedWithCeiling(t, time.Hour, func(string) (<-chan vo.LiveKCandleVo, error) {
		return feed.kCandles, nil
	})

	viewer, cancelViewer := context.WithCancel(context.Background())
	defer cancelViewer()
	updates, _ := testBed.service.WatchKCandles(viewer, "BTCUSDT")

	feed.report(liveKCandleAt(followOpenTime, "115", false))
	feed.report(liveKCandleAt(followOpenTime, "116", false))
	time.Sleep(followSettleTime)

	assert.Empty(t, updates,
		"上限是一小時，這兩根進行中的都還不該被送出")
}

// A viewer who cannot keep up must not be able to hold up the market for everybody
// else. The update they miss is superseded by the next one anyway.
func TestAViewerWhoCannotKeepUpDoesNotStallTheOthers(t *testing.T) {
	feed := newLiveFeed()
	testBed := newFollowTestBed(t, func(string) (<-chan vo.LiveKCandleVo, error) {
		return feed.kCandles, nil
	})

	slowViewer, cancelSlowViewer := context.WithCancel(context.Background())
	defer cancelSlowViewer()
	// Never read from, so its buffer fills and stays full.
	_, _ = testBed.service.WatchKCandles(slowViewer, "BTCUSDT")

	keepingUpViewer, cancelKeepingUpViewer := context.WithCancel(context.Background())
	defer cancelKeepingUpViewer()
	keepingUpUpdates, _ := testBed.service.WatchKCandles(keepingUpViewer, "BTCUSDT")

	for closePrice := range 40 {
		feed.report(liveKCandleAt(followOpenTime, decimalString(100+closePrice), false))
		select {
		case <-keepingUpUpdates:
		case <-time.After(time.Second):
			t.Fatalf("跟得上的那位在第 %d 筆之後就收不到了——慢的那位把大家卡住了", closePrice+1)
		}
	}
}

func decimalString(value int) string {
	return strconv.Itoa(value)
}

// Coming back means showing what the market looks like now. Replaying what was
// missed would make the chart re-live a stretch of trading that is already over,
// and the person watching only wants to know where things stand.
func TestComingBackGivesTheShapeNowAndReplaysNothing(t *testing.T) {
	firstFeed := newLiveFeed()
	secondFeed := newLiveFeed()
	feedsHandedOut := 0
	testBed := newFollowTestBed(t, func(string) (<-chan vo.LiveKCandleVo, error) {
		feedsHandedOut++
		if feedsHandedOut == 1 {
			return firstFeed.kCandles, nil
		}

		return secondFeed.kCandles, nil
	})

	viewer, cancelViewer := context.WithCancel(context.Background())
	defer cancelViewer()
	updates, _ := testBed.service.WatchKCandles(viewer, "BTCUSDT")

	firstFeed.report(liveKCandleAt(followOpenTime, "115", false))
	require.Equal(t, "115", (<-updates).KCandle.Close.String())
	firstFeed.end()
	require.Equal(t, dto.KCandleFollowStatusStalled, (<-updates).Status)

	// While it was down the market moved on; only where it stands now is sent.
	secondFeed.report(liveKCandleAt(followOpenTime, "131", false))

	update := <-updates
	assert.Equal(t, dto.KCandleFollowStatusForming, update.Status)
	assert.Equal(t, "131", update.KCandle.Close.String(),
		"重新跟上之後收到的該是現在的樣子")
	assert.Empty(t, updates, "中斷期間錯過的變動不該被補播")
}

// Shutting down must reach the viewers: a channel nobody will ever feed again is
// the one thing worse than being told it stopped.
func TestStoppingEndsEveryFollowAndEveryViewer(t *testing.T) {
	feed := newLiveFeed()
	testBed := newFollowTestBed(t, func(string) (<-chan vo.LiveKCandleVo, error) {
		return feed.kCandles, nil
	})

	viewer, cancelViewer := context.WithCancel(context.Background())
	defer cancelViewer()
	updates, _ := testBed.service.WatchKCandles(viewer, "BTCUSDT")

	testBed.service.Stop()

	assert.Equal(t, 0, testBed.service.FollowedSymbolCount())
	assert.Eventually(t, func() bool {
		for range updates {
			continue
		}

		return true
	}, time.Second, 10*time.Millisecond, "觀看者的更新沒有被收掉")

	_, watchError := testBed.service.WatchKCandles(viewer, "BTCUSDT")
	assert.ErrorIs(t, watchError, service.ErrKCandleFollowStopped,
		"已經停止之後再來的觀看者應該被明白回絕，而不是掛在一個沒有人餵的通道上")
}
