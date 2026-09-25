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

// followSettleTime gives watch and leave, which hand work to goroutines, time to take effect.
const followSettleTime = 200 * time.Millisecond

// liveFeed is a market feed whose reports and ending the test controls.
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

// followTestBed wires a follow service with timings small enough to outrun and a feed the test hands out.
type followTestBed struct {
	service                 *service.KCandleFollowService
	kCandleReposit          *mocks.MockIKCandleRepository
	tradingSymbolRepository *mocks.MockITradingSymbolRepository
	feedsRequested          chan string
}

// followMarketCatalog holds the viewer-driven round-the-clock market and a closing Taiwan session with a limited number of live places.
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
			// One channel carrying two symbols: a ceiling of two.
			FollowsFixedRoster:         true,
			SimultaneousChannelCeiling: 1,
			SymbolsPerLiveChannel:      2,
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

	// The clock advances on every read; a frozen one would make the ceiling throttle every forming candle, which is all these tests would then measure.
	var readings atomic.Int64
	clockProxy.EXPECT().Now().DoAndReturn(func() time.Time {
		return followStartedAt.Add(time.Duration(readings.Add(1)) * time.Second)
	}).AnyTimes()

	// Symbols default to the round-the-clock market unless a test says otherwise.
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
		func(_ context.Context, channel vo.LiveFollowChannelVo) (<-chan vo.LiveKCandleVo, error) {
			// Every symbol on the channel is recorded, since tests check followed symbols rather than lines.
			for _, symbol := range channel.Symbols {
				// Dropped rather than blocked, so a retry loop outrunning the test cannot wedge the follow.
				select {
				case testBed.feedsRequested <- symbol:
				default:
				}
			}

			return feedFor(channel.Symbols[0])
		}).AnyTimes()

	// A near-zero ceiling avoids throttling in lifecycle tests, and an hour-long quiet threshold means silence is never taken for death except where a test sets its own.
	testBed.service = service.NewKCandleFollowService(
		liveMarketDataProxy, kCandleRepository, tradingSymbolRepository, clockProxy,
		followMarketCatalog(), updateIntervalCeiling, quietTimeout, 10*time.Millisecond,
	)
	t.Cleanup(testBed.service.Stop)

	return testBed
}

var followStartedAt = time.Date(2026, 9, 3, 9, 7, 0, 0, time.UTC)

// followOpenTime is the candle still running at 09:07, on a five-minute mark and not in the future.
var followOpenTime = time.Date(2026, 9, 3, 9, 5, 0, 0, time.UTC)

// Ten viewers on one symbol are one follow, because the market has only one answer.
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

// The last viewer leaving ends the follow, since the five-minute round delivers the data anyway.
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

// Following is independent of the watchlist, so an unwatched symbol is still followable.
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

// Changing symbol leaves the old market, whose updates must stop.
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

// Joining before anything was reported hands over nothing and still succeeds.
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

// Only a closed candle is stored; a forming one is shown but never stored.
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

// Live candles follow the ordinary K candle rules, and a rule-breaking one ends only itself.
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

// A picture that silently stopped updating is more dangerous than one that says so.
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

// Arriving during an outage hands over the last candle plus the stalled status, so it does not look live.
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

// A refusing source is reported like a dropped feed and retried.
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

// An open but silent connection is this feed's usual failure mode, and the viewer must be told.
func TestAFeedThatGoesSilentIsTreatedAsStopped(t *testing.T) {
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

// A candle storage refuses does not end the follow, since the round will store it.
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

// Shutdown and a viewer leaving can race, and neither may be left holding a follow the other ended.
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

// An unset quiet threshold must not mean a zero, constantly-firing check interval.
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

// The follow must actually consult the throttle rather than forward everything.
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

// A slow viewer must not stall the others; the update it misses is superseded anyway.
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

// Reconnecting shows the market as it is now and replays nothing missed.
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

// Shutdown must close every viewer's channel rather than leave it unfed.
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

// taiwanFollowTestBed is shaped like the round-the-clock one, but for a closing market with two live places.
type taiwanFollowTestBed struct {
	service                 *service.KCandleFollowService
	tradingSymbolRepository *mocks.MockITradingSymbolRepository
	clock                   *movingClock
	feedsRequested          chan string
}

func newTaiwanFollowTestBed(t *testing.T, currentTime time.Time) *taiwanFollowTestBed {
	t.Helper()

	return newTaiwanFollowTestBedWithCatalog(t, currentTime, followMarketCatalog())
}

// uncappedFollowMarketCatalog is the Taiwan session followed from a roster without a cap, the only catalogue where "rostered" and "capped" differ, so a follow asking the wrong question fails.
func uncappedFollowMarketCatalog() domains.MarketCatalogDomain {
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
			FollowsFixedRoster:    true,
			SymbolsPerLiveChannel: 25,
		},
	})
}

func newUncappedTaiwanFollowTestBed(
	t *testing.T, currentTime time.Time,
) *taiwanFollowTestBed {
	t.Helper()

	return newTaiwanFollowTestBedWithCatalog(t, currentTime, uncappedFollowMarketCatalog())
}

func newTaiwanFollowTestBedWithCatalog(
	t *testing.T, currentTime time.Time, marketCatalogDomain domains.MarketCatalogDomain,
) *taiwanFollowTestBed {
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
		func(_ context.Context, channel vo.LiveFollowChannelVo) (<-chan vo.LiveKCandleVo, error) {
			for _, symbol := range channel.Symbols {
				select {
				case testBed.feedsRequested <- symbol:
				default:
				}
			}

			// Never delivers or ends, so a started follow stays countable.
			return make(chan vo.LiveKCandleVo), nil
		}).AnyTimes()

	testBed.service = service.NewKCandleFollowService(
		liveMarketDataProxy, kCandleRepository, tradingSymbolRepository, clockProxy,
		marketCatalogDomain, time.Nanosecond, time.Hour, time.Hour,
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

// taipeiFollowAt parses a moment in the Taiwan session's timezone.
func taipeiFollowAt(t *testing.T, moment string) time.Time {
	t.Helper()

	parsedTime, parseError := time.Parse(time.RFC3339, moment)
	require.NoError(t, parseError)

	return parsedTime
}

func TestALimitedMarketFollowsItsEarliestRegisteredSymbolsAndNoMore(t *testing.T) {
	// Two places, three watched symbols: the two registered first are live, deterministically.
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
	// Places come from the watchlist, so the follow runs with no viewers.
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
	// With both places taken, a third symbol must be told it has no place, not shown a frozen chart or told "stalled".
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
	// Out of hours both cases look identical here, but "shut" tells the viewer tomorrow will fix it while "no place" suggests a fault.
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
	// The last word before the picture stops must be the real reason: market shut, not place gone.
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

func TestALeavingViewerNeverClosesAReplacementFollowsStream(t *testing.T) {
	// Viewer ids restart at zero per follow and a rostered follow can be replaced mid-watch, so leaving by symbol alone would close the new id-zero viewer.
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T10:00:00+08:00"))
	testBed.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, symbol string) (entities.TradingSymbol, bool, error) {
			return entities.TradingSymbol{
				Symbol: symbol, Market: string(vo.MarketTaiwanStock), IsWatched: true,
			}, true, nil
		}).AnyTimes()
	watched := []entities.TradingSymbol{
		{Symbol: "2330", Market: string(vo.MarketTaiwanStock), IsWatched: true},
	}
	gomock.InOrder(
		testBed.tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(watched, nil),
		testBed.tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).
			Return([]entities.TradingSymbol{}, nil),
		testBed.tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(watched, nil),
	)

	// The first follow is retired by a roster refresh without the viewer's context ending.
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	departingViewer, cancelDepartingViewer := context.WithCancel(context.Background())
	_, watchError := testBed.service.WatchKCandles(departingViewer, "2330")
	require.NoError(t, watchError)
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	// A replacement follow, and a second viewer who is handed the same id.
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	stayingViewer, cancelStayingViewer := context.WithCancel(context.Background())
	defer cancelStayingViewer()
	updates, watchError := testBed.service.WatchKCandles(stayingViewer, "2330")
	require.NoError(t, watchError)

	cancelDepartingViewer()

	assert.Never(t, func() bool {
		select {
		case _, isDelivering := <-updates:
			return !isDelivering
		default:
			return false
		}
	}, 200*time.Millisecond, 10*time.Millisecond,
		"還在看的那個人的通道被上一個人的離開收掉了")
	assert.Equal(t, 1, testBed.service.FollowedSymbolCount())
}

func TestARoundTheClockMarketIsStillOnlyFollowedWhileSomebodyWatches(t *testing.T) {
	// A market with no ceiling hands out no places, so nothing follows it until a viewer asks.
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T10:00:00+08:00"))
	testBed.tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).Return(
		[]entities.TradingSymbol{
			{Symbol: "BTCUSDT", Market: string(vo.MarketCrypto), IsWatched: true},
		}, nil).AnyTimes()

	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Equal(t, 0, testBed.service.FollowedSymbolCount())
}

func TestWatchingASymbolNobodyRegisteredIsRefused(t *testing.T) {
	// The market comes only from registration; it is deliberately never guessed from the name.
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

// firstUpdateFrom reads one update, failing rather than hanging when none arrives.
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

// lastStatusOf reads until updates stop and returns the final status, what the viewer is left seeing.
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
	// A roster place is not the viewer's, so their leaving must not give it back.
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T10:00:00+08:00"))
	testBed.watching("2330")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	viewerLeft, viewerLeaves := context.WithCancel(t.Context())
	_, watchError := testBed.service.WatchKCandles(viewerLeft, "2330")
	require.NoError(t, watchError)

	viewerLeaves()

	// Never, not Eventually: the count already starts at one, so what must hold is that it never drops.
	assert.Never(t, func() bool {
		return testBed.service.FollowedSymbolCount() != 1
	}, 500*time.Millisecond, 10*time.Millisecond)
}

func TestChannelsAreGivenUpBeforeNewOnesAreTaken(t *testing.T) {
	// Exceeding the source's line limit even briefly costs every place, so during a wholesale swap the one-line plan must never see two open lines.
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
		func(followContext context.Context, _ vo.LiveFollowChannelVo) (<-chan vo.LiveKCandleVo, error) {
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
	// The line must really be open before the swap, or a high-water mark of one proves nothing.
	require.Eventually(t, func() bool { return concurrentFeeds.currentlyOpen() == 1 },
		2*time.Second, 10*time.Millisecond)

	require.NoError(t, followService.RefreshFixedFollows(t.Context()))

	require.Eventually(t, func() bool { return concurrentFeeds.currentlyOpen() == 1 },
		2*time.Second, 10*time.Millisecond)
	assert.Equal(t, 1, concurrentFeeds.highWaterMark(),
		"方案只准一條，換名單的那一瞬間也不能有第二條")
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

// feedCounter records the peak number of simultaneously open feeds, the only trace an ordering bug leaves.
type feedCounter struct {
	mutex   sync.Mutex
	open    int
	highest int
	total   int
}

func (counter *feedCounter) opened() {
	counter.mutex.Lock()
	defer counter.mutex.Unlock()
	counter.open++
	counter.total++
	if counter.open > counter.highest {
		counter.highest = counter.open
	}
}

// totalOpened tells a high-water mark of one apart from nothing having been retried.
func (counter *feedCounter) totalOpened() int {
	counter.mutex.Lock()
	defer counter.mutex.Unlock()

	return counter.total
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
	// The registration decides where the feed comes from, so a failed read cannot be carried past.
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
	// A roster pass racing shutdown must not start feeds nothing will close.
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T10:00:00+08:00"))
	testBed.watching("2330", "2454")
	testBed.service.Stop()

	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Equal(t, 0, testBed.service.FollowedSymbolCount())
}

func TestHandingOutPlacesLeavesAViewerDrivenFollowAlone(t *testing.T) {
	// A roster pass must not touch viewer-driven follows, or every crypto chart would go dark each round.
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

	// The watched one, plus the one holding a Taiwan place.
	assert.Equal(t, 2, followService.FollowedSymbolCount())
}

func TestASecondPassWithTheSamePlacesChangesNothing(t *testing.T) {
	// The pass runs every five minutes; reissuing places each time would break charts on every pass.
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T10:00:00+08:00"))
	testBed.watching("2330", "2454")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Equal(t, 2, testBed.service.FollowedSymbolCount())
	// Two feeds in total, not four: running follows were left alone.
	assert.Len(t, drainFeeds(testBed.feedsRequested, 3), 2)
}

func TestALimitedMarketIsFollowedAgainOnceItOpens(t *testing.T) {
	// Closing returns every place, so opening must take them again or the market stays unfollowed all day.
	testBed := newTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T21:00:00+08:00"))
	testBed.watching("2330")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	require.Equal(t, 0, testBed.service.FollowedSymbolCount())

	testBed.clock.moveTo(taipeiFollowAt(t, "2026-09-09T09:00:00+08:00"))
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Equal(t, 1, testBed.service.FollowedSymbolCount())
}

// sharedChannelTestBed follows a limited market with two symbols on one line and hands the test that channel's feed.
type sharedChannelTestBed struct {
	service                 *service.KCandleFollowService
	tradingSymbolRepository *mocks.MockITradingSymbolRepository
	// One queue so the opened channel and its feed cannot drift out of step.
	channelsOpened chan openedChannel
	// saved records, in order, the symbol of every candle stored.
	saved chan string
}

type openedChannel struct {
	channel vo.LiveFollowChannelVo
	feed    chan vo.LiveKCandleVo
}

func newSharedChannelTestBed(t *testing.T) *sharedChannelTestBed {
	t.Helper()

	mockController := gomock.NewController(t)
	liveMarketDataProxy := mocks.NewMockILiveMarketDataProxy(mockController)
	kCandleRepository := mocks.NewMockIKCandleRepository(mockController)
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	// The clock advances on every read; a frozen one would make the ceiling throttle every forming candle.
	clockProxy := mocks.NewMockIClockProxy(mockController)
	var readings atomic.Int64
	clockProxy.EXPECT().Now().DoAndReturn(func() time.Time {
		return taipeiFollowAt(t, "2026-09-08T10:00:00+08:00").
			Add(time.Duration(readings.Add(1)) * time.Second)
	}).AnyTimes()

	testBed := &sharedChannelTestBed{
		tradingSymbolRepository: tradingSymbolRepository,
		channelsOpened:          make(chan openedChannel, 16),
		saved:                   make(chan string, 16),
	}

	// Recording the stored symbol is the only outside view that a shared-line candle was filed under its own symbol.
	kCandleRepository.EXPECT().Save(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, kCandle entities.KCandle) (entities.KCandle, error) {
			select {
			case testBed.saved <- kCandle.Symbol:
			default:
			}

			return kCandle, nil
		}).AnyTimes()

	liveMarketDataProxy.EXPECT().FollowKCandles(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, channel vo.LiveFollowChannelVo) (<-chan vo.LiveKCandleVo, error) {
			feed := make(chan vo.LiveKCandleVo, 8)
			// Dropped rather than blocked, so a retry loop cannot wedge the goroutine the test waits to shut down.
			select {
			case testBed.channelsOpened <- openedChannel{channel: channel, feed: feed}:
			default:
			}

			return feed, nil
		}).AnyTimes()

	// A minimal update ceiling avoids throttling, and a small retry ceiling lets reopens happen within the test.
	testBed.service = service.NewKCandleFollowService(
		liveMarketDataProxy, kCandleRepository, tradingSymbolRepository, clockProxy,
		followMarketCatalog(), time.Nanosecond, time.Hour, 10*time.Millisecond,
	)
	t.Cleanup(testBed.service.Stop)

	return testBed
}

// watching sets the watchlist the next refresh reads, replacing the previous answer.
func (testBed *sharedChannelTestBed) watching(symbols ...string) {
	watchedSymbols := make([]entities.TradingSymbol, 0, len(symbols))
	for _, symbol := range symbols {
		watchedSymbols = append(watchedSymbols, entities.TradingSymbol{
			Symbol: symbol, Market: string(vo.MarketTaiwanStock), IsWatched: true,
		})
	}

	testBed.tradingSymbolRepository.EXPECT().
		FindWatched(gomock.Any()).Return(watchedSymbols, nil).Times(1)
	testBed.tradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, symbol string) (entities.TradingSymbol, bool, error) {
			return entities.TradingSymbol{
				Symbol: symbol, Market: string(vo.MarketTaiwanStock), IsWatched: true,
			}, true, nil
		}).AnyTimes()
}

func (testBed *sharedChannelTestBed) nextChannel(t *testing.T) (vo.LiveFollowChannelVo, chan vo.LiveKCandleVo) {
	t.Helper()

	select {
	case opened := <-testBed.channelsOpened:
		return opened.channel, opened.feed
	case <-time.After(2 * time.Second):
		t.Fatal("沒有任何通道被開起來")

		return vo.LiveFollowChannelVo{}, nil
	}
}

func sharedChannelKCandle(t *testing.T, symbol string, closed bool) vo.LiveKCandleVo {
	t.Helper()

	return vo.LiveKCandleVo{
		Symbol:   symbol,
		OpenTime: taipeiFollowAt(t, "2026-09-08T09:59:00+08:00").UTC(),
		Open:     decimal.RequireFromString("100"),
		High:     decimal.RequireFromString("120"),
		Low:      decimal.RequireFromString("90"),
		Close:    decimal.RequireFromString("110"),
		Volume:   decimal.RequireFromString("11"),
		Closed:   closed,
	}
}

// A limited market's whole roster travels on one line, as its plan allows.
func TestALimitedMarketsRosterTravelsDownOneChannel(t *testing.T) {
	testBed := newSharedChannelTestBed(t)
	testBed.watching("2330", "2454")

	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	channel, _ := testBed.nextChannel(t)
	assert.Equal(t, []string{"2330", "2454"}, channel.Symbols)
	assert.Empty(t, testBed.channelsOpened, "整份名單只該開一條線")
}

// Sharing a line must not mean sharing a picture.
func TestACandleReachesOnlyTheViewersOfItsOwnSymbol(t *testing.T) {
	testBed := newSharedChannelTestBed(t)
	testBed.watching("2330", "2454")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	_, feed := testBed.nextChannel(t)

	viewer, cancelViewer := context.WithCancel(context.Background())
	defer cancelViewer()
	updates, watchError := testBed.service.WatchKCandles(viewer, "2330")
	require.NoError(t, watchError)

	feed <- sharedChannelKCandle(t, "2454", false)
	feed <- sharedChannelKCandle(t, "2330", false)

	update := <-updates
	assert.Equal(t, "2330", update.Symbol,
		"看 2330 的人不該收到 2454 的那一根——同一條線不等於同一份行情")
	assert.Equal(t, dto.KCandleFollowStatusForming, update.Status)
}

// Only the symbol a closed candle names is stored, or the wrong market would be written undetectably.
func TestOnlyTheSymbolACandleNamesIsStored(t *testing.T) {
	testBed := newSharedChannelTestBed(t)
	testBed.watching("2330", "2454")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	_, feed := testBed.nextChannel(t)

	// Deliberately the later symbol, so filing under the line's first symbol would fail.
	feed <- sharedChannelKCandle(t, "2454", true)

	select {
	case saved := <-testBed.saved:
		assert.Equal(t, "2454", saved)
	case <-time.After(2 * time.Second):
		t.Fatal("走完的那一根沒有被存起來")
	}
	assert.Empty(t, testBed.saved, "同一條線上的另一檔不該有任何東西被存入")
}

// One line going down is news for every symbol on it.
func TestAChannelEndingTellsEverySymbolOnIt(t *testing.T) {
	testBed := newSharedChannelTestBed(t)
	testBed.watching("2330", "2454")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	_, feed := testBed.nextChannel(t)

	viewer, cancelViewer := context.WithCancel(context.Background())
	defer cancelViewer()
	firstUpdates, firstError := testBed.service.WatchKCandles(viewer, "2330")
	require.NoError(t, firstError)
	secondUpdates, secondError := testBed.service.WatchKCandles(viewer, "2454")
	require.NoError(t, secondError)

	close(feed)

	assert.Equal(t, dto.KCandleFollowStatusStalled, (<-firstUpdates).Status)
	assert.Equal(t, dto.KCandleFollowStatusStalled, (<-secondUpdates).Status)
}

// Reopening brings the whole roster back, not whichever symbol happened to be first.
func TestAChannelThatComesBackCarriesTheWholeRosterAgain(t *testing.T) {
	testBed := newSharedChannelTestBed(t)
	testBed.watching("2330", "2454")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	_, feed := testBed.nextChannel(t)

	close(feed)

	reopened, _ := testBed.nextChannel(t)
	assert.Equal(t, []string{"2330", "2454"}, reopened.Symbols)
}

// A changed roster is a new channel, and the old line ends first because the plan limits concurrent lines.
func TestARosterThatGainsASymbolRebuildsTheChannel(t *testing.T) {
	testBed := newSharedChannelTestBed(t)
	testBed.watching("2330")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	require.NotNil(t, mustChannelSymbols(t, testBed, []string{"2330"}))

	testBed.watching("2330", "2454")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	rebuilt, _ := testBed.nextChannel(t)
	assert.Equal(t, []string{"2330", "2454"}, rebuilt.Symbols)
}

// A symbol winning no place leaves the line's set, and so the line, untouched.
func TestASymbolThatWinsNoPlaceLeavesTheChannelAlone(t *testing.T) {
	testBed := newSharedChannelTestBed(t)
	testBed.watching("2330", "2454")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	require.NotNil(t, mustChannelSymbols(t, testBed, []string{"2330", "2454"}))

	testBed.watching("2330", "2454", "2603")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Empty(t, testBed.channelsOpened,
		"名額已滿，第三檔進不來，線上的名單沒變就不該重建")
}

func TestARosterThatLosesASymbolRebuildsTheChannel(t *testing.T) {
	testBed := newSharedChannelTestBed(t)
	testBed.watching("2330", "2454")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	require.NotNil(t, mustChannelSymbols(t, testBed, []string{"2330", "2454"}))

	testBed.watching("2330")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	rebuilt, _ := testBed.nextChannel(t)
	assert.Equal(t, []string{"2330"}, rebuilt.Symbols)
}

// An unchanged roster interrupts nobody.
func TestARosterThatDidNotChangeLeavesTheChannelAlone(t *testing.T) {
	testBed := newSharedChannelTestBed(t)
	testBed.watching("2330", "2454")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	require.NotNil(t, mustChannelSymbols(t, testBed, []string{"2330", "2454"}))

	testBed.watching("2330", "2454")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Empty(t, testBed.channelsOpened, "名單沒變就不該有第二條線被開起來")
}

// A silent line is still open, so it must be released before the next dial or two lines would exceed the plan.
func TestAChannelThatFellSilentIsLetGoOfBeforeTheNextAttempt(t *testing.T) {
	mockController := gomock.NewController(t)
	// The clock advances on every read, so the line is found silent as soon as it is checked.
	clockProxy := mocks.NewMockIClockProxy(mockController)
	var readings atomic.Int64
	clockProxy.EXPECT().Now().DoAndReturn(func() time.Time {
		return taipeiFollowAt(t, "2026-09-08T10:00:00+08:00").
			Add(time.Duration(readings.Add(1)) * time.Second)
	}).AnyTimes()
	tradingSymbolRepository := mocks.NewMockITradingSymbolRepository(mockController)
	tradingSymbolRepository.EXPECT().FindWatched(gomock.Any()).
		Return(taiwanWatchlist("2330"), nil).AnyTimes()

	concurrentFeeds := &feedCounter{}
	liveMarketDataProxy := mocks.NewMockILiveMarketDataProxy(mockController)
	liveMarketDataProxy.EXPECT().FollowKCandles(gomock.Any(), gomock.Any()).DoAndReturn(
		func(followContext context.Context, _ vo.LiveFollowChannelVo) (<-chan vo.LiveKCandleVo, error) {
			concurrentFeeds.opened()
			// Never delivers or ends on its own, like a silent line; it closes only when released.
			go func() {
				<-followContext.Done()
				concurrentFeeds.closed()
			}()

			return make(chan vo.LiveKCandleVo), nil
		}).AnyTimes()

	// A zero silence threshold makes every check find the line quiet, forcing repeated retries.
	followService := service.NewKCandleFollowService(
		liveMarketDataProxy, mocks.NewMockIKCandleRepository(mockController),
		tradingSymbolRepository, clockProxy, followMarketCatalog(),
		time.Nanosecond, 2*time.Millisecond, time.Millisecond,
	)
	t.Cleanup(followService.Stop)

	require.NoError(t, followService.RefreshFixedFollows(t.Context()))

	// Several attempts are needed, or a high-water mark of one only means nothing was retried.
	require.Eventually(t, func() bool { return concurrentFeeds.totalOpened() >= 3 },
		2*time.Second, 10*time.Millisecond)
	assert.Equal(t, 1, concurrentFeeds.highWaterMark(),
		"安靜的那條線必須先放掉，才能撥下一條——方案只准一條")
}

// A symbol surviving a line rebuild keeps its viewers and must not be told its place is gone.
func TestASymbolThatSurvivesARebuildKeepsItsViewers(t *testing.T) {
	testBed := newSharedChannelTestBed(t)
	testBed.watching("2330", "2454")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	require.NotNil(t, mustChannelSymbols(t, testBed, []string{"2330", "2454"}))

	viewer, cancelViewer := context.WithCancel(context.Background())
	defer cancelViewer()
	updates, watchError := testBed.service.WatchKCandles(viewer, "2330")
	require.NoError(t, watchError)

	testBed.watching("2330")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	// Stalled, not unavailable: the line is replaced, not the symbol.
	assert.Equal(t, dto.KCandleFollowStatusStalled, (<-updates).Status)

	// The candle on the new line reaches the existing viewer without reconnecting.
	_, rebuiltFeed := testBed.nextChannel(t)
	rebuiltFeed <- sharedChannelKCandle(t, "2330", false)

	select {
	case update := <-updates:
		assert.Equal(t, "2330", update.Symbol)
		assert.Equal(t, dto.KCandleFollowStatusForming, update.Status)
	case <-time.After(2 * time.Second):
		t.Fatal("換線之後，原本的觀看者沒有再收到任何更新")
	}
}

// A symbol that really lost its place hears that, not the promise it will be back.
func TestASymbolDroppedFromARebuildIsToldItsPlaceIsGone(t *testing.T) {
	testBed := newSharedChannelTestBed(t)
	testBed.watching("2330", "2454")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	require.NotNil(t, mustChannelSymbols(t, testBed, []string{"2330", "2454"}))

	viewer, cancelViewer := context.WithCancel(context.Background())
	defer cancelViewer()
	updates, watchError := testBed.service.WatchKCandles(viewer, "2454")
	require.NoError(t, watchError)

	testBed.watching("2330")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Equal(t, dto.KCandleFollowStatusUnavailable, (<-updates).Status,
		"被擠掉的那一檔不該先聽到「等一下就回來」")
}

// An emptied roster must actually stop the open line, or the plan is spent on nothing.
func TestARosterThatEmptiesEndsTheOpenChannel(t *testing.T) {
	testBed := newSharedChannelTestBed(t)
	testBed.watching("2330")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))
	require.NotNil(t, mustChannelSymbols(t, testBed, []string{"2330"}))

	viewer, cancelViewer := context.WithCancel(context.Background())
	defer cancelViewer()
	updates, watchError := testBed.service.WatchKCandles(viewer, "2330")
	require.NoError(t, watchError)

	testBed.watching()
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	// Being told and then closed distinguishes a real end from a quiet line.
	assert.Equal(t, dto.KCandleFollowStatusUnavailable, (<-updates).Status)
	// Polled without blocking, so a line that never closes fails rather than hangs.
	assert.Eventually(t, func() bool {
		select {
		case _, isDelivering := <-updates:
			return !isDelivering
		default:
			return false
		}
	}, 2*time.Second, 10*time.Millisecond, "名單空了，開著的那條線必須真的收掉")
	assert.Empty(t, testBed.channelsOpened, "收掉之後不該再開新的")
}

// mustChannelSymbols waits for the next channel and asserts its symbols.
func mustChannelSymbols(
	t *testing.T, testBed *sharedChannelTestBed, expectedSymbols []string,
) chan vo.LiveKCandleVo {
	t.Helper()

	channel, feed := testBed.nextChannel(t)
	require.Equal(t, expectedSymbols, channel.Symbols)

	return feed
}

// With no cap, every watched stock is followed.
func TestAnUncappedRosteredMarketFollowsEveryWatchedSymbol(t *testing.T) {
	testBed := newUncappedTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T10:00:00+08:00"))
	testBed.watching("1101", "1301", "2317", "2330", "2454", "2603", "2609", "2881")

	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	assert.Equal(t, 8, testBed.service.FollowedSymbolCount())
}

// An unwatched stock gets no live updates even with no cap, a case the capped bed cannot distinguish from a cap check.
func TestAViewerOfAnUnwatchedSymbolIsToldSoEvenWhenNothingCapsTheMarket(t *testing.T) {
	testBed := newUncappedTaiwanFollowTestBed(t, taipeiFollowAt(t, "2026-09-08T10:00:00+08:00"))
	testBed.watching("2330", "2454")
	require.NoError(t, testBed.service.RefreshFixedFollows(t.Context()))

	updates, watchError := testBed.service.WatchKCandles(t.Context(), "2603")

	require.NoError(t, watchError)
	update := firstUpdateFrom(t, updates)
	assert.Equal(t, dto.KCandleFollowStatusUnavailable, update.Status)
	assert.Equal(t, "2603", update.Symbol)
	// Still two: watching an unwatched symbol must not start following it.
	assert.Equal(t, 2, testBed.service.FollowedSymbolCount())
}
