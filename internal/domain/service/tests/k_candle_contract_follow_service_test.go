package service_test

import (
	"context"
	"errors"
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

// contractFollowTestBed wires a contract follow whose feed the test hands out, over a
// contract watchlist of ETHUSDT and BTCUSDT, with DOGEUSDT known but not followed.
//
// It holds no K candle store at all, and that is the point: a contract follow has no
// way to store a candle, so "a closed candle is not stored by the follow" is a fact
// about what it is built from rather than something a test has to catch it not doing.
type contractFollowTestBed struct {
	service        *service.KCandleContractFollowService
	feedsRequested chan vo.LiveFollowChannelVo
}

func newContractFollowTestBed(
	t *testing.T, updateIntervalCeiling time.Duration,
	feedFor func(attempt int) (<-chan vo.LiveKCandleVo, error),
) *contractFollowTestBed {
	t.Helper()

	mockController := gomock.NewController(t)
	liveMarketDataProxy := mocks.NewMockILiveMarketDataProxy(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)

	var readings atomic.Int64
	clockProxy.EXPECT().Now().DoAndReturn(func() time.Time {
		return followStartedAt.Add(time.Duration(readings.Add(1)) * time.Second)
	}).AnyTimes()

	contractTradingSymbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, symbol string) (entities.ContractTradingSymbol, bool, error) {
			switch symbol {
			case "BTCUSDT", "ETHUSDT":
				return entities.ContractTradingSymbol{Symbol: symbol, IsWatched: true}, true, nil
			case "DOGEUSDT":
				return entities.ContractTradingSymbol{Symbol: symbol}, true, nil
			}

			return entities.ContractTradingSymbol{}, false, nil
		}).AnyTimes()

	testBed := &contractFollowTestBed{feedsRequested: make(chan vo.LiveFollowChannelVo, 16)}

	var attempts atomic.Int64
	liveMarketDataProxy.EXPECT().FollowKCandles(gomock.Any(), gomock.Any()).DoAndReturn(
		func(_ context.Context, channel vo.LiveFollowChannelVo) (<-chan vo.LiveKCandleVo, error) {
			select {
			case testBed.feedsRequested <- channel:
			default:
			}

			return feedFor(int(attempts.Add(1)))
		}).AnyTimes()

	testBed.service = service.NewKCandleContractFollowService(
		liveMarketDataProxy, contractTradingSymbolRepository, clockProxy,
		updateIntervalCeiling, time.Hour, 10*time.Millisecond)
	t.Cleanup(testBed.service.Stop)

	return testBed
}

// oneContractFeed is a test bed whose every attempt is handed the same feed.
func oneContractFeed(t *testing.T) (*contractFollowTestBed, *liveFeed) {
	feed := newLiveFeed()

	return newContractFollowTestBed(t, time.Nanosecond, func(int) (<-chan vo.LiveKCandleVo, error) {
		return feed.kCandles, nil
	}), feed
}

func TestAContractIsFollowedOnceHoweverManyWatchIt(t *testing.T) {
	testBed, feed := oneContractFeed(t)

	firstViewer, firstViewerLeaves := context.WithCancel(context.Background())
	defer firstViewerLeaves()
	firstUpdates, firstError := testBed.service.WatchKCandleContracts(firstViewer, "BTCUSDT")
	require.NoError(t, firstError)
	assert.Equal(t, 1, testBed.service.FollowedSymbolCount())

	secondViewer, secondViewerLeaves := context.WithCancel(context.Background())
	defer secondViewerLeaves()
	secondUpdates, secondError := testBed.service.WatchKCandleContracts(secondViewer, "BTCUSDT")
	require.NoError(t, secondError)
	assert.Equal(t, 1, testBed.service.FollowedSymbolCount(), "第二個觀看者不該讓系統跟第二份")

	// One line for the contract, carrying exactly it — and no second one for the
	// second viewer.
	channel := <-testBed.feedsRequested
	assert.Equal(t, []string{"BTCUSDT"}, channel.Symbols)
	assert.Empty(t, testBed.feedsRequested, "第二個觀看者不該讓系統多開一條連線")

	feed.report(liveKCandleAt(followOpenTime, "64000.5", false))

	assert.Equal(t, "64000.5", firstUpdateFrom(t, firstUpdates).KCandle.Close.String())
	assert.Equal(t, "64000.5", firstUpdateFrom(t, secondUpdates).KCandle.Close.String())
}

func TestAContractFollowEndsOnlyWithItsLastViewer(t *testing.T) {
	testBed, feed := oneContractFeed(t)

	firstViewer, firstViewerLeaves := context.WithCancel(context.Background())
	secondViewer, secondViewerLeaves := context.WithCancel(context.Background())
	firstUpdates, _ := testBed.service.WatchKCandleContracts(firstViewer, "BTCUSDT")
	_, _ = testBed.service.WatchKCandleContracts(secondViewer, "BTCUSDT")

	secondViewerLeaves()
	time.Sleep(followSettleTime)
	assert.Equal(t, 1, testBed.service.FollowedSymbolCount(), "還有人在看就該繼續跟")

	feed.report(liveKCandleAt(followOpenTime, "64000.5", false))
	assert.Equal(t, dto.KCandleFollowStatusForming, firstUpdateFrom(t, firstUpdates).Status,
		"剩下那位觀看者應照常收到更新")

	firstViewerLeaves()
	assert.Eventually(t, func() bool { return testBed.service.FollowedSymbolCount() == 0 },
		time.Second, 10*time.Millisecond, "最後一個觀看者離開後就該停止跟盤")
}

// A viewer arriving mid-candle is handed the shape so far rather than an empty chart.
func TestAContractViewerArrivingMidCandleIsGivenTheShapeSoFar(t *testing.T) {
	testBed, feed := oneContractFeed(t)

	firstViewer, firstViewerLeaves := context.WithCancel(context.Background())
	defer firstViewerLeaves()
	firstUpdates, _ := testBed.service.WatchKCandleContracts(firstViewer, "BTCUSDT")
	feed.report(liveKCandleAt(followOpenTime, "64000.5", false))
	require.Equal(t, "64000.5", firstUpdateFrom(t, firstUpdates).KCandle.Close.String())

	lateViewer, lateViewerLeaves := context.WithCancel(context.Background())
	defer lateViewerLeaves()
	lateUpdates, _ := testBed.service.WatchKCandleContracts(lateViewer, "BTCUSDT")

	update := firstUpdateFrom(t, lateUpdates)
	assert.Equal(t, dto.KCandleFollowStatusForming, update.Status)
	assert.Equal(t, "64000.5", update.KCandle.Close.String())
}

// Watching a contract never touches the spot follow of the same code, and the reverse.
func TestAContractAndTheSpotMarketOfTheSameCodeAreFollowedApart(t *testing.T) {
	spotFeed := newLiveFeed()
	spotBed := newFollowTestBed(t, func(string) (<-chan vo.LiveKCandleVo, error) {
		return spotFeed.kCandles, nil
	})
	contractBed, contractFeed := oneContractFeed(t)

	spotViewer, spotViewerLeaves := context.WithCancel(context.Background())
	defer spotViewerLeaves()
	spotUpdates, _ := spotBed.service.WatchKCandles(spotViewer, "BTCUSDT")

	contractViewer, contractViewerLeaves := context.WithCancel(context.Background())
	defer contractViewerLeaves()
	contractUpdates, contractError := contractBed.service.WatchKCandleContracts(contractViewer, "BTCUSDT")
	require.NoError(t, contractError)

	assert.Equal(t, 1, spotBed.service.FollowedSymbolCount(), "現貨那一份不因合約有人在看而多或少")
	assert.Equal(t, 1, contractBed.service.FollowedSymbolCount())

	spotFeed.report(liveKCandleAt(followOpenTime, "64100", false))
	contractFeed.report(liveKCandleAt(followOpenTime, "64000.5", false))

	assert.Equal(t, "64100", firstUpdateFrom(t, spotUpdates).KCandle.Close.String())
	assert.Equal(t, "64000.5", firstUpdateFrom(t, contractUpdates).KCandle.Close.String(),
		"合約觀看者收到的是合約的價格")
}

func TestOnlyAContractOnTheContractWatchlistIsFollowed(t *testing.T) {
	testCases := []struct {
		name            string
		symbol          string
		expectedError   error
		expectedWording string
	}{
		{name: "a followed contract", symbol: "ETHUSDT"},
		{name: "a contract the system has never heard of", symbol: "NOPEUSDT",
			expectedError: domains.ErrTradingSymbolNotRegistered, expectedWording: "找不到這個合約標的 NOPEUSDT"},
		{name: "a known contract that is not followed", symbol: "DOGEUSDT",
			expectedError:   domains.ErrContractTradingSymbolNotWatched,
			expectedWording: "DOGEUSDT 不在合約追蹤名單上——請先把它加進合約追蹤名單"},
		{name: "no contract named at all", symbol: "  ",
			expectedError: domains.ErrKCandleContractValidation},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			testBed, _ := oneContractFeed(t)
			viewer, viewerLeaves := context.WithCancel(context.Background())
			defer viewerLeaves()

			_, watchError := testBed.service.WatchKCandleContracts(viewer, testCase.symbol)

			if testCase.expectedError == nil {
				require.NoError(t, watchError)
				assert.Equal(t, 1, testBed.service.FollowedSymbolCount())

				return
			}

			require.ErrorIs(t, watchError, testCase.expectedError)
			assert.ErrorContains(t, watchError, testCase.expectedWording)
			assert.Equal(t, 0, testBed.service.FollowedSymbolCount(), "被拒絕的不該開始跟")
		})
	}
}

func TestWatchingAContractFailsWhenItsEntryCannotBeRead(t *testing.T) {
	mockController := gomock.NewController(t)
	contractTradingSymbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.ContractTradingSymbol{}, false, errors.New("the database went away"))
	followService := service.NewKCandleContractFollowService(
		mocks.NewMockILiveMarketDataProxy(mockController), contractTradingSymbolRepository,
		mocks.NewMockIClockProxy(mockController), time.Nanosecond, time.Hour, time.Millisecond)

	_, watchError := followService.WatchKCandleContracts(context.Background(), "BTCUSDT")

	assert.EqualError(t, watchError, "the database went away")
}

// What arrives is the last-price candle and nothing else.
func TestAContractUpdateCarriesTheLastPriceCandle(t *testing.T) {
	testBed, feed := oneContractFeed(t)
	viewer, viewerLeaves := context.WithCancel(context.Background())
	defer viewerLeaves()
	updates, _ := testBed.service.WatchKCandleContracts(viewer, "BTCUSDT")

	reported := liveKCandleAt(followOpenTime, "64000.5", false)
	reported.QuoteVolume = decimal.NewNullDecimal(decimal.RequireFromString("64000.5"))
	reported.TakerBuyBaseVolume = decimal.NewNullDecimal(decimal.RequireFromString("0.4"))
	reported.TakerBuyQuoteVolume = decimal.NewNullDecimal(decimal.RequireFromString("25600.2"))
	feed.report(reported)

	update := firstUpdateFrom(t, updates)
	assert.Equal(t, "BTCUSDT", update.Symbol)
	assert.Equal(t, followOpenTime, update.KCandle.OpenTime)
	assert.Equal(t, "100", update.KCandle.Open.String())
	assert.Equal(t, "120", update.KCandle.High.String())
	assert.Equal(t, "90", update.KCandle.Low.String())
	assert.Equal(t, "64000.5", update.KCandle.Close.String())
	assert.Equal(t, "1", update.KCandle.Volume.String())
	assert.Equal(t, "64000.5", update.KCandle.QuoteVolume.Decimal.String())
	assert.Equal(t, "0.4", update.KCandle.TakerBuyBaseVolume.Decimal.String())
	assert.Equal(t, "25600.2", update.KCandle.TakerBuyQuoteVolume.Decimal.String())
}

// The line going down is said; coming back gives the shape now and replays nothing.
func TestAContractViewerIsToldOfAnOutageAndGivenTheShapeNowWhenItEnds(t *testing.T) {
	firstFeed, secondFeed := newLiveFeed(), newLiveFeed()
	testBed := newContractFollowTestBed(t, time.Nanosecond, func(attempt int) (<-chan vo.LiveKCandleVo, error) {
		if attempt == 1 {
			return firstFeed.kCandles, nil
		}

		return secondFeed.kCandles, nil
	})
	viewer, viewerLeaves := context.WithCancel(context.Background())
	defer viewerLeaves()
	updates, _ := testBed.service.WatchKCandleContracts(viewer, "BTCUSDT")

	firstFeed.end()
	assert.Equal(t, dto.KCandleFollowStatusStalled, firstUpdateFrom(t, updates).Status,
		"跟盤停了要告訴觀看者")

	secondFeed.report(liveKCandleAt(followOpenTime.Add(2*time.Minute), "64100", false))
	update := firstUpdateFrom(t, updates)
	assert.Equal(t, dto.KCandleFollowStatusForming, update.Status)
	assert.Equal(t, "64100", update.KCandle.Close.String(), "重新跟上給的是現在的樣子")
}

// A source that refuses outright is retried rather than given up on.
func TestAContractSourceThatRefusesIsRetried(t *testing.T) {
	feed := newLiveFeed()
	testBed := newContractFollowTestBed(t, time.Nanosecond, func(attempt int) (<-chan vo.LiveKCandleVo, error) {
		if attempt == 1 {
			return nil, errors.New("the venue said no")
		}

		return feed.kCandles, nil
	})
	viewer, viewerLeaves := context.WithCancel(context.Background())
	defer viewerLeaves()
	updates, _ := testBed.service.WatchKCandleContracts(viewer, "BTCUSDT")

	assert.Equal(t, dto.KCandleFollowStatusStalled, firstUpdateFrom(t, updates).Status)
	feed.report(liveKCandleAt(followOpenTime, "64000.5", false))
	assert.Equal(t, dto.KCandleFollowStatusForming, firstUpdateFrom(t, updates).Status)
}

// Contracts never close and have no places to hand out, so a viewer is never told the
// market is shut or that there is no room.
func TestAContractViewerIsNeverToldTheMarketIsShutOrFull(t *testing.T) {
	testBed, feed := oneContractFeed(t)
	viewer, viewerLeaves := context.WithCancel(context.Background())
	defer viewerLeaves()
	updates, _ := testBed.service.WatchKCandleContracts(viewer, "BTCUSDT")

	feed.report(liveKCandleAt(followOpenTime, "64000.5", true))

	status := firstUpdateFrom(t, updates).Status
	assert.NotEqual(t, dto.KCandleFollowStatusMarketClosed, status)
	assert.NotEqual(t, dto.KCandleFollowStatusUnavailable, status)
	assert.Equal(t, dto.KCandleFollowStatusClosed, status)
}

func TestStoppingEndsEveryContractFollowAndTurnsLateViewersAway(t *testing.T) {
	testBed, _ := oneContractFeed(t)
	viewer, viewerLeaves := context.WithCancel(context.Background())
	updates, _ := testBed.service.WatchKCandleContracts(viewer, "BTCUSDT")

	testBed.service.Stop()
	testBed.service.Stop()

	_, stillOpen := <-updates
	assert.False(t, stillOpen, "關機時每一位觀看者的更新都要關上")
	assert.Equal(t, 0, testBed.service.FollowedSymbolCount())

	_, lateError := testBed.service.WatchKCandleContracts(context.Background(), "BTCUSDT")
	assert.ErrorIs(t, lateError, service.ErrKCandleFollowStopped)

	// Leaving after the shutdown changes nothing and does not panic.
	viewerLeaves()
	time.Sleep(followSettleTime)
	assert.Equal(t, 0, testBed.service.FollowedSymbolCount())
}

// A viewer of an earlier follow leaving late must never close a later follow's viewer
// who happens to hold the same id.
func TestALateLeaverNeverClosesALaterContractFollowsViewer(t *testing.T) {
	testBed, _ := oneContractFeed(t)

	firstViewer, firstViewerLeaves := context.WithCancel(context.Background())
	_, _ = testBed.service.WatchKCandleContracts(firstViewer, "BTCUSDT")
	firstViewerLeaves()
	assert.Eventually(t, func() bool { return testBed.service.FollowedSymbolCount() == 0 },
		time.Second, 10*time.Millisecond)

	secondViewer, secondViewerLeaves := context.WithCancel(context.Background())
	defer secondViewerLeaves()
	_, _ = testBed.service.WatchKCandleContracts(secondViewer, "BTCUSDT")
	time.Sleep(followSettleTime)

	assert.Equal(t, 1, testBed.service.FollowedSymbolCount(), "後來那一份跟盤不該被先前的離開收掉")
}

// A candle naming a contract this line does not carry belongs to nobody here and is
// dropped, rather than drawn on somebody else's chart.
func TestACandleNamingAnotherContractIsDropped(t *testing.T) {
	testBed, feed := oneContractFeed(t)
	viewer, viewerLeaves := context.WithCancel(context.Background())
	defer viewerLeaves()
	updates, _ := testBed.service.WatchKCandleContracts(viewer, "BTCUSDT")

	stranger := liveKCandleAt(followOpenTime, "3000", false)
	stranger.Symbol = "ETHUSDT"
	feed.report(stranger)
	feed.report(liveKCandleAt(followOpenTime, "64000.5", false))

	update := firstUpdateFrom(t, updates)
	assert.Equal(t, "BTCUSDT", update.Symbol)
	assert.Equal(t, "64000.5", update.KCandle.Close.String(), "別的合約的那一根不該畫到這張圖上")
}

// Whether a contract is followed is asked when a viewer arrives and not again: one taken
// off the contract watchlist while somebody is looking keeps their picture until they
// leave, while a newcomer is refused.
func TestAContractTakenOffTheWatchlistMidWatchKeepsItsViewer(t *testing.T) {
	mockController := gomock.NewController(t)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	var readings atomic.Int64
	clockProxy.EXPECT().Now().DoAndReturn(func() time.Time {
		return followStartedAt.Add(time.Duration(readings.Add(1)) * time.Second)
	}).AnyTimes()

	feed := newLiveFeed()
	liveMarketDataProxy := mocks.NewMockILiveMarketDataProxy(mockController)
	liveMarketDataProxy.EXPECT().FollowKCandles(gomock.Any(), gomock.Any()).
		Return(feed.kCandles, nil).AnyTimes()

	var isWatched atomic.Bool
	isWatched.Store(true)
	contractTradingSymbolRepository := mocks.NewMockIContractTradingSymbolRepository(mockController)
	contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").DoAndReturn(
		func(context.Context, string) (entities.ContractTradingSymbol, bool, error) {
			return entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: isWatched.Load()}, true, nil
		}).AnyTimes()

	followService := service.NewKCandleContractFollowService(
		liveMarketDataProxy, contractTradingSymbolRepository, clockProxy,
		time.Nanosecond, time.Hour, 10*time.Millisecond)
	t.Cleanup(followService.Stop)

	viewer, viewerLeaves := context.WithCancel(context.Background())
	defer viewerLeaves()
	updates, watchError := followService.WatchKCandleContracts(viewer, "BTCUSDT")
	require.NoError(t, watchError)

	isWatched.Store(false)

	_, newcomerError := followService.WatchKCandleContracts(context.Background(), "BTCUSDT")
	assert.ErrorIs(t, newcomerError, domains.ErrContractTradingSymbolNotWatched, "新來的觀看者照規則被拒絕")

	feed.report(liveKCandleAt(followOpenTime, "64000.5", false))
	assert.Equal(t, "64000.5", firstUpdateFrom(t, updates).KCandle.Close.String(),
		"已經在看的人照常收到，直到他離開")
}

// With a real ceiling, the first forming candle of a new follow reaches its first viewer
// at once, and the next one inside the ceiling is held back.
func TestTheFirstFormingContractCandleArrivesAtOnceAndTheNextIsThrottled(t *testing.T) {
	feed := newLiveFeed()
	testBed := newContractFollowTestBed(t, 10*time.Second, func(int) (<-chan vo.LiveKCandleVo, error) {
		return feed.kCandles, nil
	})
	viewer, viewerLeaves := context.WithCancel(context.Background())
	defer viewerLeaves()
	updates, _ := testBed.service.WatchKCandleContracts(viewer, "BTCUSDT")

	feed.report(liveKCandleAt(followOpenTime, "64000", false))
	first := firstUpdateFrom(t, updates)
	assert.Equal(t, dto.KCandleFollowStatusForming, first.Status)
	assert.Equal(t, "64000", first.KCandle.Close.String(), "第一個觀看者要立刻收到進行中的那一根")

	feed.report(liveKCandleAt(followOpenTime, "64001", false))
	feed.report(liveKCandleAt(followOpenTime, "64002", true))
	closed := firstUpdateFrom(t, updates)
	assert.Equal(t, dto.KCandleFollowStatusClosed, closed.Status, "上限內的下一根進行中被擋下，走完的照送")
	assert.Equal(t, "64002", closed.KCandle.Close.String())
	assert.Empty(t, updates)
}
