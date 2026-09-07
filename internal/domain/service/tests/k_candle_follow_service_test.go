package service_test

import (
	"context"
	"errors"
	"strconv"
	"sync"
	"sync/atomic"
	"testing"
	"time"

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
	service                 *service.KCandleFollowService
	kCandleReposit          *mocks.MockIKCandleRepository
	tradingSymbolRepository *mocks.MockITradingSymbolRepository
	feedsRequested          chan string
}

// followMarketCatalog is the two markets these tests are written against: the
// round-the-clock one, whose follows are driven by viewers, and a Taiwan session that
// closes and hands out only so many live places at a time.
func followMarketCatalog() domains.MarketCatalogDomain {
	return domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{
		vo.MarketCrypto: {},
		vo.MarketTaiwanStock: {
			TradingSession: vo.TradingSessionVo{
				Location:   time.FixedZone("Asia/Taipei", 8*60*60),
				DailyStart: 9 * time.Hour,
				DailyEnd:   13*time.Hour + 30*time.Minute,
				Weekdays: []time.Weekday{
					time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday,
				},
			},
			SimultaneousFollowCeiling: 2,
		},
	})
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

	// Every symbol these tests watch belongs to the round-the-clock market unless a
	// test says otherwise, which is what keeps the lifecycle rules readable without a
	// market in sight.
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, symbol string) (entities.TradingSymbol, bool, error) {
			return entities.TradingSymbol{
				Symbol: symbol, Market: string(vo.MarketCrypto), IsWatched: true,
			}, true, nil
		}).AnyTimes()

	testBed := &followTestBed{
		kCandleReposit:          kCandleRepository,
		tradingSymbolRepository: tradingSymbolRepository,
		feedsRequested:          make(chan string, 16),
	}

	liveMarketDataProxy.EXPECT().FollowKCandles(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, target vo.FollowTargetVo) (<-chan vo.LiveKCandleVo, error) {
			symbol := target.Symbol
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
		liveMarketDataProxy, kCandleRepository, tradingSymbolRepository, clockProxy,
		followMarketCatalog(), updateIntervalCeiling, quietTimeout, 10*time.Millisecond,
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

// taiwanFollowTestBed follows the same shape as the round-the-clock one, but its
// symbols belong to a market that closes and hands out only two live places.
type taiwanFollowTestBed struct {
	service                 *service.KCandleFollowService
	tradingSymbolRepository *mocks.MockITradingSymbolRepository
	clock                   *movingClock
	feedsRequested          chan string
}

func newTaiwanFollowTestBed(t *testing.T, currentTime time.Time) *taiwanFollowTestBed {
	t.Helper()

	mockController := gomock.NewController(t)
	liveMarketDataProxy := mocks.NewMockILiveMarketDataProxy(mockController)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	movingClock := &movingClock{currentTime: currentTime}
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().DoAndReturn(movingClock.now).AnyTimes()

	testBed := &taiwanFollowTestBed{
		tradingSymbolRepository: tradingSymbolRepository,
		clock:                   movingClock,
		feedsRequested:          make(chan string, 16),
	}

	liveMarketDataProxy.EXPECT().FollowKCandles(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, target vo.FollowTargetVo) (<-chan vo.LiveKCandleVo, error) {
			symbol := target.Symbol
			select {
			case testBed.feedsRequested <- symbol:
			default:
			}

			// Never delivers and never ends, so a follow that was started stays
			// started and can be counted.
			return make(chan vo.LiveKCandleVo), nil
		}).AnyTimes()

	testBed.service = service.NewKCandleFollowService(
		liveMarketDataProxy, kCandleRepository, tradingSymbolRepository, clockProxy,
		followMarketCatalog(), time.Nanosecond, time.Hour, time.Hour,
	)
	t.Cleanup(testBed.service.Stop)

	return testBed
}

func (testBed *taiwanFollowTestBed) watching(symbols ...string) {
	watchedSymbols := make([]entities.TradingSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		watchedSymbols = append(watchedSymbols, entities.TradingSymbol{
			Symbol: symbol, Market: string(vo.MarketTaiwanStock), IsWatched: true,
		})
	}

	testBed.tradingSymbolRepository.EXPECT().
		FindWatched(gomock.Any()).Return(watchedSymbols, nil).AnyTimes()
	testBed.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, symbol string) (entities.TradingSymbol, bool, error) {
			return entities.TradingSymbol{
				Symbol: symbol, Market: string(vo.MarketTaiwanStock), IsWatched: true,
			}, true, nil
		}).AnyTimes()
}

// taipeiFollowAt is a moment said in the clock the Taiwan session is written in.
func taipeiFollowAt(t *testing.T, moment string) time.Time {
	t.Helper()

	parsedTime, parseError := time.Parse(time.RFC3339, moment)
	require.NoError(t, parseError)

	return parsedTime
}

func TestALimitedMarketFollowsItsEarliestRegisteredSymbolsAndNoMore(t *testing.T) {
	// Two places, three symbols on the watchlist. Which two are live has to be a fact
	// somebody can state, not a race — so it is the two that were registered first.
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T10:00:00+08:00"))
	testBed.watching("2330", "2454", "2603")

	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Equal(t, 2, testBed.service.FollowedSymbolCount())
	assert.ElementsMatch(t, []string{"2330", "2454"}, drainFeeds(testBed.feedsRequested, 2))
}

func TestALimitedMarketFollowsFewerThanItsPlacesWhenThatIsAllThereIs(t *testing.T) {
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T10:00:00+08:00"))
	testBed.watching("2330")

	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Equal(t, 1, testBed.service.FollowedSymbolCount())
}

func TestALimitedMarketIsFollowedWithNobodyWatching(t *testing.T) {
	// Places are handed out by the watchlist, so nobody looking is simply nobody
	// looking — the follow was never theirs to start or to end.
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T10:00:00+08:00"))
	testBed.watching("2330")

	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Equal(t, 1, testBed.service.FollowedSymbolCount())
}

func TestAClosedLimitedMarketHoldsNoPlaces(t *testing.T) {
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T21:00:00+08:00"))
	testBed.watching("2330", "2454")

	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Equal(t, 0, testBed.service.FollowedSymbolCount())
}

func TestAViewerOfASymbolWithNoPlaceIsToldSoRatherThanShownAFrozenPicture(t *testing.T) {
	// The two places are taken. A third symbol must not quietly get a chart that
	// looks live and never moves — and must not be told the feed "stalled", which
	// would have them waiting for a recovery that is not coming.
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T10:00:00+08:00"))
	testBed.watching("2330", "2454", "2603")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	updates, watchError := testBed.service.WatchKCandles(t.Context(), "2603")

	require.NoError(t, watchError)
	update := firstUpdateFrom(t, updates)
	assert.Equal(t, dto.KCandleFollowStatusUnavailable, update.Status)
	assert.Equal(t, "2603", update.Symbol)
	// Still two: watching a symbol with no place must not take one.
	assert.Equal(t, 2, testBed.service.FollowedSymbolCount())
}

func TestAViewerOfASymbolWithAPlaceJoinsTheFollowAlreadyRunning(t *testing.T) {
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T10:00:00+08:00"))
	testBed.watching("2330", "2454")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	_, watchError := testBed.service.WatchKCandles(t.Context(), "2330")

	require.NoError(t, watchError)
	assert.Equal(t, 2, testBed.service.FollowedSymbolCount())
}

func TestASymbolPushedOutOfItsPlaceIsToldItsUpdatesAreGone(t *testing.T) {
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T10:00:00+08:00"))
	testBed.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, symbol string) (entities.TradingSymbol, bool, error) {
			return entities.TradingSymbol{
				Symbol: symbol, Market: string(vo.MarketTaiwanStock), IsWatched: true,
			}, true, nil
		}).AnyTimes()
	gomock.InOrder(
		testBed.tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(
			[]entities.TradingSymbol{
				{Symbol: "2330", Market: string(vo.MarketTaiwanStock), IsWatched: true},
			}, nil),
		testBed.tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(
			[]entities.TradingSymbol{}, nil),
	)
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	updates, watchError := testBed.service.WatchKCandles(t.Context(), "2330")
	require.NoError(t, watchError)

	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Equal(t, dto.KCandleFollowStatusUnavailable, lastStatusOf(t, updates))
	assert.Equal(t, 0, testBed.service.FollowedSymbolCount())
}

func TestAViewerArrivingOutOfHoursIsToldTheMarketIsShutRatherThanThatThereIsNoPlace(t *testing.T) {
	// Out of hours the two answers look identical from here — nothing is being
	// followed either way — and they ask opposite things of the viewer. "No place"
	// sends them looking for a fault; "shut" tells them tomorrow will fix it by
	// itself.
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T21:00:00+08:00"))
	testBed.watching("2330")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	updates, watchError := testBed.service.WatchKCandles(t.Context(), "2330")

	require.NoError(t, watchError)
	update := firstUpdateFrom(t, updates)
	assert.Equal(t, dto.KCandleFollowStatusMarketClosed, update.Status)
	assert.Equal(t, "2330", update.Symbol)
}

func TestAFollowEndedByTheCloseSaysTheMarketShutRatherThanThatItsPlaceIsGone(t *testing.T) {
	// The very last thing a viewer hears before the picture stops has to be the
	// reason it stopped. Hearing that its place is gone, in the second the market
	// shut, is being told a fault where there is only the end of the day.
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T13:00:00+08:00"))
	testBed.watching("2330")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	updates, watchError := testBed.service.WatchKCandles(t.Context(), "2330")
	require.NoError(t, watchError)

	testBed.clock.moveTo(taipeiFollowAt(t, "2026-09-08T14:00:00+08:00"))
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Equal(t, dto.KCandleFollowStatusMarketClosed, lastStatusOf(t, updates))
	assert.Equal(t, 0, testBed.service.FollowedSymbolCount())
}

func TestARoundTheClockMarketIsStillOnlyFollowedWhileSomebodyWatches(t *testing.T) {
	// The rule this feature started with is untouched: a market with no ceiling hands
	// no places out, so nothing follows it until a viewer asks.
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T10:00:00+08:00"))
	testBed.tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(
		[]entities.TradingSymbol{
			{Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true},
		}, nil).AnyTimes()

	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Equal(t, 0, testBed.service.FollowedSymbolCount())
}

func TestWatchingASymbolNobodyRegisteredIsRefused(t *testing.T) {
	// Without a registration there is no market, and without a market there is no
	// source. Guessing one from the name is the rule this system deliberately lacks.
	mockController := gomock.NewController(t)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2454").
		Return(entities.TradingSymbol{}, false, nil)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(followStartedAt).AnyTimes()
	followService := service.NewKCandleFollowService(
		mocks.NewMockILiveMarketDataProxy(mockController),
		mocks.NewMockIKCandleRepository(mockController),
		tradingSymbolRepository, clockProxy, followMarketCatalog(),
		time.Nanosecond, time.Hour, time.Hour,
	)
	t.Cleanup(followService.Stop)

	_, watchError := followService.WatchKCandles(t.Context(), "2454")

	assert.ErrorIs(t, watchError, domains.ErrTradingSymbolNotRegistered)
}

// firstUpdateFrom reads one update, failing rather than hanging when none arrives —
// a viewer told nothing is a failure to report, not a reason to stop the suite.
func firstUpdateFrom(
	t *testing.T, updates <-chan dto.KCandleFollowUpdateDto,
) dto.KCandleFollowUpdateDto {
	t.Helper()

	select {
	case update := <-updates:
		return update
	case <-time.After(2 * time.Second):
		t.Fatal("the viewer was told nothing in time")

		return dto.KCandleFollowUpdateDto{}
	}
}

// drainFeeds collects which symbols a feed was opened for.
func drainFeeds(feedsRequested chan string, count int) []string {
	symbols := make([]string, 0, count)
	for range count {
		select {
		case symbol := <-feedsRequested:
			symbols = append(symbols, symbol)
		case <-time.After(2 * time.Second):
			return symbols
		}
	}

	return symbols
}

// lastStatusOf reads updates until they stop arriving and reports the final one,
// which is what a viewer is left looking at.
func lastStatusOf(t *testing.T, updates <-chan dto.KCandleFollowUpdateDto) string {
	t.Helper()

	lastStatus := ""
	for {
		select {
		case update, isDelivering := <-updates:
			if !isDelivering {
				return lastStatus
			}
			lastStatus = update.Status
		case <-time.After(2 * time.Second):
			t.Fatal("the viewer was told nothing in time")

			return lastStatus
		}
	}
}

func TestAFollowHeldUpByARosterOutlivesItsLastViewer(t *testing.T) {
	// The place was handed out by the watchlist, not asked for by anybody. A viewer
	// leaving is therefore nobody watching — not a reason to give the place back,
	// which would leave it unfilled until somebody happened to look again.
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T10:00:00+08:00"))
	testBed.watching("2330")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	viewerLeft, viewerLeaves := context.WithCancel(t.Context())
	_, watchError := testBed.service.WatchKCandles(viewerLeft, "2330")
	require.NoError(t, watchError)

	viewerLeaves()

	// Never rather than eventually: the count starts at one, so waiting for it to
	// reach one would be satisfied before the viewer had even finished leaving. What
	// has to hold is that it never drops.
	assert.Never(t, func() bool {
		return testBed.service.FollowedSymbolCount() != 1
	}, 500*time.Millisecond, 10*time.Millisecond)
}

func TestPlacesAreGivenUpBeforeNewOnesAreTaken(t *testing.T) {
	// Going over what the source allows costs every place at once, not just the extra
	// one — so for the moment a roster is swapped wholesale, the count must never rise
	// above the ceiling. Starting first and stopping after would double it.
	mockController := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(taipeiFollowAt(t, "2026-09-08T10:00:00+08:00")).AnyTimes()
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	gomock.InOrder(
		tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(
			taiwanWatchlist("2330", "2454"), nil),
		tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(
			taiwanWatchlist("2603", "2609"), nil),
	)

	concurrentFeeds := &feedCounter{}
	liveMarketDataProxy := mocks.NewMockILiveMarketDataProxy(mockController)
	liveMarketDataProxy.EXPECT().FollowKCandles(gomock.Any(), gomock.Any()).DoAndReturn(
		func(followContext context.Context, _ vo.FollowTargetVo) (<-chan vo.LiveKCandleVo, error) {
			concurrentFeeds.opened()
			go func() {
				<-followContext.Done()
				concurrentFeeds.closed()
			}()

			return make(chan vo.LiveKCandleVo), nil
		}).AnyTimes()

	followService := service.NewKCandleFollowService(
		liveMarketDataProxy, mocks.NewMockIKCandleRepository(mockController),
		tradingSymbolRepository, clockProxy, followMarketCatalog(),
		time.Nanosecond, time.Hour, time.Hour,
	)
	t.Cleanup(followService.Stop)

	require.NoError(t, followService.RefreshFixedFollows(t.Context()))
	// Both places have to be genuinely open before the swap, or a high water mark of
	// two would only mean the first pair never got going.
	require.Eventually(t, func() bool { return concurrentFeeds.currentlyOpen() == 2 },
		2*time.Second, 10*time.Millisecond)

	require.NoError(t, followService.RefreshFixedFollows(t.Context()))

	require.Eventually(t, func() bool { return concurrentFeeds.currentlyOpen() == 2 },
		2*time.Second, 10*time.Millisecond)
	assert.Equal(t, 2, concurrentFeeds.highWaterMark())
	assert.Equal(t, 2, followService.FollowedSymbolCount())
}

func taiwanWatchlist(symbols ...string) []entities.TradingSymbol {
	watchedSymbols := make([]entities.TradingSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		watchedSymbols = append(watchedSymbols, entities.TradingSymbol{
			Symbol: symbol, Market: string(vo.MarketTaiwanStock), IsWatched: true,
		})
	}

	return watchedSymbols
}

// feedCounter records how many feeds were open at once, which is the only way to see
// an ordering that leaves no trace once it has finished.
type feedCounter struct {
	mutex   sync.Mutex
	open    int
	highest int
}

func (counter *feedCounter) opened() {
	counter.mutex.Lock()
	defer counter.mutex.Unlock()
	counter.open++
	if counter.open > counter.highest {
		counter.highest = counter.open
	}
}

func (counter *feedCounter) closed() {
	counter.mutex.Lock()
	defer counter.mutex.Unlock()
	counter.open--
}

func (counter *feedCounter) currentlyOpen() int {
	counter.mutex.Lock()
	defer counter.mutex.Unlock()

	return counter.open
}

func (counter *feedCounter) highWaterMark() int {
	counter.mutex.Lock()
	defer counter.mutex.Unlock()

	return counter.highest
}

func TestWatchingFailsWhenTheRegistrationCannotBeRead(t *testing.T) {
	// Which market a symbol belongs to decides where its feed comes from, so not
	// being able to find out is not something to carry on past.
	mockController := gomock.NewController(t)
	storageFailure := errors.New("storage unreachable")
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "2330").
		Return(entities.TradingSymbol{}, false, storageFailure)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(followStartedAt).AnyTimes()
	followService := service.NewKCandleFollowService(
		mocks.NewMockILiveMarketDataProxy(mockController),
		mocks.NewMockIKCandleRepository(mockController),
		tradingSymbolRepository, clockProxy, followMarketCatalog(),
		time.Nanosecond, time.Hour, time.Hour,
	)
	t.Cleanup(followService.Stop)

	_, watchError := followService.WatchKCandles(t.Context(), "2330")

	assert.ErrorIs(t, watchError, storageFailure)
}

func TestHandingOutPlacesFailsWhenTheWatchlistCannotBeRead(t *testing.T) {
	mockController := gomock.NewController(t)
	storageFailure := errors.New("storage unreachable")
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(nil, storageFailure)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(followStartedAt).AnyTimes()
	followService := service.NewKCandleFollowService(
		mocks.NewMockILiveMarketDataProxy(mockController),
		mocks.NewMockIKCandleRepository(mockController),
		tradingSymbolRepository, clockProxy, followMarketCatalog(),
		time.Nanosecond, time.Hour, time.Hour,
	)
	t.Cleanup(followService.Stop)

	refreshError := followService.RefreshFixedFollows(t.Context())

	assert.ErrorIs(t, refreshError, storageFailure)
}

func TestHandingOutPlacesAfterShutdownStartsNothing(t *testing.T) {
	// The roster pass and the shutdown can arrive at once. Starting follows into a
	// service that has already let go of every one of them would leave feeds open
	// with nothing left to close them.
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T10:00:00+08:00"))
	testBed.watching("2330", "2454")
	testBed.service.Stop()

	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Equal(t, 0, testBed.service.FollowedSymbolCount())
}

func TestHandingOutPlacesLeavesAViewerDrivenFollowAlone(t *testing.T) {
	// A market with no ceiling hands out no places, so a roster pass has no business
	// touching what somebody is watching there. Tearing it down would make every
	// crypto chart go dark once every five minutes.
	mockController := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(taipeiFollowAt(t, "2026-09-08T10:00:00+08:00")).AnyTimes()
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").Return(
		entities.TradingSymbol{
			Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true,
		}, true, nil).AnyTimes()
	tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(
		taiwanWatchlist("2330"), nil).AnyTimes()
	liveMarketDataProxy := mocks.NewMockILiveMarketDataProxy(mockController)
	liveMarketDataProxy.EXPECT().FollowKCandles(gomock.Any(), gomock.Any()).
		Return(make(chan vo.LiveKCandleVo), nil).AnyTimes()
	followService := service.NewKCandleFollowService(
		liveMarketDataProxy, mocks.NewMockIKCandleRepository(mockController),
		tradingSymbolRepository, clockProxy, followMarketCatalog(),
		time.Nanosecond, time.Hour, time.Hour,
	)
	t.Cleanup(followService.Stop)
	_, watchError := followService.WatchKCandles(t.Context(), "BTCUSDT")
	require.NoError(t, watchError)

	require.NoError(t, followService.RefreshFixedFollows(t.Context()))

	// The one somebody is watching, plus the one holding a Taiwan place.
	assert.Equal(t, 2, followService.FollowedSymbolCount())
}

func TestASecondPassWithTheSamePlacesChangesNothing(t *testing.T) {
	// Every five minutes this runs again. If it took the places back and gave them
	// out afresh each time, a chart would break for a moment on every pass.
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T10:00:00+08:00"))
	testBed.watching("2330", "2454")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Equal(t, 2, testBed.service.FollowedSymbolCount())
	// Two feeds opened in total, not four: the follows already running were left
	// running rather than replaced by new ones.
	assert.Len(t, drainFeeds(testBed.feedsRequested, 3), 2)
}

func TestALimitedMarketIsFollowedAgainOnceItOpens(t *testing.T) {
	// Closing gives every place back. Opening has to take them again by itself, or
	// the first pass of the morning would find nothing to do and the market would
	// stay unfollowed all day.
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T21:00:00+08:00"))
	testBed.watching("2330")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	require.Equal(t, 0, testBed.service.FollowedSymbolCount())

	testBed.clock.moveTo(taipeiFollowAt(t, "2026-09-09T09:00:00+08:00"))
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Equal(t, 1, testBed.service.FollowedSymbolCount())
}
