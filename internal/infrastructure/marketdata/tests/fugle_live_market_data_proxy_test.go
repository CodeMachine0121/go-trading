package marketdata_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/marketdata"
	"github.com/coder/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// fugleStreamUnderTest stands in for the live feed: it records what it was told and
// pushes back whatever the test hands it.
type fugleStreamUnderTest struct {
	server       *httptest.Server
	instructions chan map[string]any
	pushes       chan string
}

func newFugleStreamUnderTest(t *testing.T) *fugleStreamUnderTest {
	t.Helper()

	stream := &fugleStreamUnderTest{
		instructions: make(chan map[string]any, 8),
		pushes:       make(chan string, 16),
	}

	stream.server = httptest.NewServer(http.HandlerFunc(func(
		responseWriter http.ResponseWriter, request *http.Request,
	) {
		connection, acceptError := websocket.Accept(responseWriter, request, nil)
		if acceptError != nil {
			return
		}
		defer func() { _ = connection.CloseNow() }()

		go func() {
			for {
				_, body, readError := connection.Read(request.Context())
				if readError != nil {
					return
				}

				var instruction map[string]any
				if json.Unmarshal(body, &instruction) == nil {
					stream.instructions <- instruction
				}
			}
		}()

		for {
			select {
			case push := <-stream.pushes:
				if connection.Write(request.Context(), websocket.MessageText, []byte(push)) != nil {
					return
				}
			case <-request.Context().Done():
				return
			}
		}
	}))
	t.Cleanup(stream.server.Close)

	return stream
}

func (stream *fugleStreamUnderTest) push(body string) {
	stream.pushes <- body
}

func (stream *fugleStreamUnderTest) nextInstruction(t *testing.T) map[string]any {
	t.Helper()

	select {
	case instruction := <-stream.instructions:
		return instruction
	case <-time.After(2 * time.Second):
		t.Fatal("the source was told nothing in time")

		return nil
	}
}

func (stream *fugleStreamUnderTest) follow(t *testing.T) <-chan vo.LiveKCandleVo {
	t.Helper()

	followContext, stopFollowing := context.WithCancel(t.Context())
	t.Cleanup(stopFollowing)

	liveKCandles, followError := marketdata.NewFugleLiveMarketDataProxy(
		"ws"+stream.server.URL[len("http"):], "a-key").
		FollowKCandles(followContext, vo.FollowTargetVo{
			Symbol: "2330", Market: vo.MarketTaiwanStock,
		})
	require.NoError(t, followError)

	return liveKCandles
}

// fugleCandlePush spells one pushed candle the way the source does.
func fugleCandlePush(localTime string, close string, volume string) string {
	return fugleCandlePushOpening(localTime, "574", close, volume)
}

// fugleCandlePushOpening is the same, with the open spelled out — for the cases that
// are about which contribution's open the slot ends up with.
func fugleCandlePushOpening(localTime string, open string, close string, volume string) string {
	return `{"event":"data","channel":"candles","data":{"symbol":"2330","date":"` + localTime +
		`","open":` + open + `,"high":576,"low":572,"close":` + close + `,"volume":` + volume + `}}`
}

func nextLiveKCandle(t *testing.T, liveKCandles <-chan vo.LiveKCandleVo) vo.LiveKCandleVo {
	t.Helper()

	select {
	case liveKCandle, isDelivering := <-liveKCandles:
		require.True(t, isDelivering, "the feed ended without reporting anything")

		return liveKCandle
	case <-time.After(2 * time.Second):
		t.Fatal("the feed reported nothing in time")

		return vo.LiveKCandleVo{}
	}
}

func TestFollowingSaysWhoIsAskingAndWhatIsWanted(t *testing.T) {
	// Both have to succeed before there is a feed at all — a caller handed a channel
	// is entitled to believe a feed was opened.
	stream := newFugleStreamUnderTest(t)

	stream.follow(t)

	authentication := stream.nextInstruction(t)
	assert.Equal(t, "auth", authentication["event"])
	assert.Equal(t, "a-key", authentication["data"].(map[string]any)["apikey"])

	subscription := stream.nextInstruction(t)
	assert.Equal(t, "subscribe", subscription["event"])
	assert.Equal(t, "candles", subscription["data"].(map[string]any)["channel"])
	assert.Equal(t, "2330", subscription["data"].(map[string]any)["symbol"])
}

func TestAPushedCandleIsReportedInTheShapeTheDomainKnows(t *testing.T) {
	stream := newFugleStreamUnderTest(t)
	liveKCandles := stream.follow(t)

	stream.push(fugleCandlePush("2026-09-08T10:00:00.000+08:00", "575", "8450"))

	liveKCandle := nextLiveKCandle(t, liveKCandles)
	assert.Equal(t, "2330", liveKCandle.Symbol)
	assert.Equal(t, taipeiAt(t, "2026-09-08T10:00:00+08:00").UTC(), liveKCandle.OpenTime)
	assert.Equal(t, "575", liveKCandle.Close.String())
	assert.Equal(t, "8450", liveKCandle.Volume.String())
	// Nothing has arrived to prove this candle finished, so it is not claimed to have.
	assert.False(t, liveKCandle.Closed)
}

func TestACandleIsReportedFinishedOnlyOnceALaterOneArrives(t *testing.T) {
	// This source never says a candle is final. A push landing in a later slot is the
	// only proof available that the earlier slot finished.
	stream := newFugleStreamUnderTest(t)
	liveKCandles := stream.follow(t)

	stream.push(fugleCandlePush("2026-09-08T10:00:00.000+08:00", "575", "8450"))
	require.False(t, nextLiveKCandle(t, liveKCandles).Closed)

	stream.push(fugleCandlePush("2026-09-08T10:05:00.000+08:00", "580", "1000"))

	finished := nextLiveKCandle(t, liveKCandles)
	assert.True(t, finished.Closed)
	assert.Equal(t, taipeiAt(t, "2026-09-08T10:00:00+08:00").UTC(), finished.OpenTime)
	assert.Equal(t, "575", finished.Close.String())

	// The finished one is reported first on purpose: it is the one that gets stored,
	// and a viewer shown the new slot first would watch the chart go backwards.
	forming := nextLiveKCandle(t, liveKCandles)
	assert.False(t, forming.Closed)
	assert.Equal(t, taipeiAt(t, "2026-09-08T10:05:00+08:00").UTC(), forming.OpenTime)
}

func TestPushesInsideOneSlotAreFoldedIntoOneCandle(t *testing.T) {
	// The source does not say how long a pushed candle covers, and its subscription
	// takes no length. Folding by the slot a push falls in is right whatever rate it
	// pushes at — at one bar a minute each slot has a single contributor and folding
	// is the identity, and a source that ever pushed faster is folded rather than
	// believed.
	stream := newFugleStreamUnderTest(t)
	liveKCandles := stream.follow(t)

	stream.push(fugleCandlePushOpening("2026-09-08T10:00:00.000+08:00", "574", "575", "100"))
	require.Equal(t, "100", nextLiveKCandle(t, liveKCandles).Volume.String())

	// A different open on the later part, so that "opens where its earliest part
	// opened" is a claim this test could actually catch being broken.
	stream.push(fugleCandlePushOpening("2026-09-08T10:00:30.000+08:00", "581", "590", "50"))

	folded := nextLiveKCandle(t, liveKCandles)
	assert.Equal(t, taipeiAt(t, "2026-09-08T10:00:00+08:00").UTC(), folded.OpenTime)
	assert.Equal(t, "150", folded.Volume.String(), "the slot traded everything its parts traded")
	assert.Equal(t, "590", folded.Close.String(), "the slot closes where its latest part closed")
	assert.Equal(t, "574", folded.Open.String(), "the slot opens where its earliest part opened")
	assert.False(t, folded.Closed)
}

func TestASlotIsOpenedAndClosedByTimeRatherThanByArrivalOrder(t *testing.T) {
	// A source is free to hand two pushes of the same slot over in either order.
	// Reading them in arrival order gives the slot the open of whichever turned up
	// first and the close of whichever turned up last — and the slot is then stored
	// with both wrong, while its high, low and volume stay right and hide it.
	stream := newFugleStreamUnderTest(t)
	liveKCandles := stream.follow(t)

	stream.push(fugleCandlePushOpening("2026-09-08T10:00:30.000+08:00", "581", "590", "50"))
	require.Equal(t, "50", nextLiveKCandle(t, liveKCandles).Volume.String())

	stream.push(fugleCandlePushOpening("2026-09-08T10:00:00.000+08:00", "574", "575", "100"))

	folded := nextLiveKCandle(t, liveKCandles)
	assert.Equal(t, "574", folded.Open.String(), "the slot opens where its earliest part opened")
	assert.Equal(t, "590", folded.Close.String(), "the slot closes where its latest part closed")
	assert.Equal(t, "150", folded.Volume.String())
}

func TestARepeatOfTheSamePushDoesNotCountTwice(t *testing.T) {
	// A source restating a bar it already sent must not double the slot's volume.
	stream := newFugleStreamUnderTest(t)
	liveKCandles := stream.follow(t)

	stream.push(fugleCandlePush("2026-09-08T10:00:00.000+08:00", "575", "100"))
	require.Equal(t, "100", nextLiveKCandle(t, liveKCandles).Volume.String())

	stream.push(fugleCandlePush("2026-09-08T10:00:00.000+08:00", "578", "120"))

	restated := nextLiveKCandle(t, liveKCandles)
	assert.Equal(t, "120", restated.Volume.String())
	assert.Equal(t, "578", restated.Close.String())
}

func TestAPushOlderThanTheSlotBeingBuiltIsIgnored(t *testing.T) {
	// Messages arriving out of order say nothing new about a slot already superseded,
	// and letting one through would rewrite a candle that has already been stored.
	stream := newFugleStreamUnderTest(t)
	liveKCandles := stream.follow(t)

	stream.push(fugleCandlePush("2026-09-08T10:05:00.000+08:00", "580", "100"))
	require.Equal(t, taipeiAt(t, "2026-09-08T10:05:00+08:00").UTC(),
		nextLiveKCandle(t, liveKCandles).OpenTime)

	stream.push(fugleCandlePush("2026-09-08T10:00:00.000+08:00", "575", "999"))
	stream.push(fugleCandlePush("2026-09-08T10:05:30.000+08:00", "585", "50"))

	// The stale push produced nothing, so the next thing reported is the later slot
	// folded — still 10:05, and without the 999 that arrived out of order.
	stillForming := nextLiveKCandle(t, liveKCandles)
	assert.Equal(t, taipeiAt(t, "2026-09-08T10:05:00+08:00").UTC(), stillForming.OpenTime)
	assert.Equal(t, "150", stillForming.Volume.String())
}

func TestMessagesThatAreNotCandlesAreIgnored(t *testing.T) {
	// The greeting, the acknowledgements and the heartbeats are the protocol talking
	// about itself and say nothing about the market.
	stream := newFugleStreamUnderTest(t)
	liveKCandles := stream.follow(t)

	stream.push(`{"event":"authenticated","data":{"message":"Authenticated successfully"}}`)
	stream.push(`{"event":"heartbeat","data":{"time":1234567890}}`)
	stream.push(`{"event":"data","channel":"trades","data":{"symbol":"2330","price":575}}`)
	stream.push(fugleCandlePush("2026-09-08T10:00:00.000+08:00", "575", "8450"))

	liveKCandle := nextLiveKCandle(t, liveKCandles)
	assert.Equal(t, taipeiAt(t, "2026-09-08T10:00:00+08:00").UTC(), liveKCandle.OpenTime)
}

func TestAnUnreadableMessageDoesNotEndTheFeed(t *testing.T) {
	// One bad message is one bad message. Ending the feed over it would cost the
	// viewer everything that came after.
	stream := newFugleStreamUnderTest(t)
	liveKCandles := stream.follow(t)

	stream.push(`{"event":"data","channel":"candles","data":{"date":`)
	stream.push(fugleCandlePush("2026-09-08T10:00:00.000+08:00", "575", "8450"))

	assert.Equal(t, taipeiAt(t, "2026-09-08T10:00:00+08:00").UTC(),
		nextLiveKCandle(t, liveKCandles).OpenTime)
}

func TestAFeedThatCannotBeOpenedIsReportedRatherThanHandedBack(t *testing.T) {
	_, followError := marketdata.NewFugleLiveMarketDataProxy(
		"ws://127.0.0.1:1/streaming", "a-key").
		FollowKCandles(t.Context(), vo.FollowTargetVo{
			Symbol: "2330", Market: vo.MarketTaiwanStock,
		})

	require.Error(t, followError)
}

func TestTheChannelClosesWhenTheFollowEnds(t *testing.T) {
	// Closing the channel is the only way this proxy reports an ending, so a caller
	// never has to ask about state — and letting go of the follow is an ending like
	// any other.
	stream := newFugleStreamUnderTest(t)
	followContext, stopFollowing := context.WithCancel(t.Context())
	liveKCandles, followError := marketdata.NewFugleLiveMarketDataProxy(
		"ws"+stream.server.URL[len("http"):], "a-key").
		FollowKCandles(followContext, vo.FollowTargetVo{
			Symbol: "2330", Market: vo.MarketTaiwanStock,
		})
	require.NoError(t, followError)

	stopFollowing()

	assert.Eventually(t, func() bool {
		select {
		case _, isDelivering := <-liveKCandles:
			return !isDelivering
		default:
			return false
		}
	}, 2*time.Second, 10*time.Millisecond)
}

func TestAFoldedSlotReachesAsHighAndAsLowAsAnyOfItsParts(t *testing.T) {
	stream := newFugleStreamUnderTest(t)
	liveKCandles := stream.follow(t)

	stream.push(`{"event":"data","channel":"candles","data":{"symbol":"2330",` +
		`"date":"2026-09-08T10:00:00.000+08:00","open":574,"high":576,"low":572,"close":575,"volume":100}}`)
	require.Equal(t, "576", nextLiveKCandle(t, liveKCandles).High.String())

	stream.push(`{"event":"data","channel":"candles","data":{"symbol":"2330",` +
		`"date":"2026-09-08T10:00:30.000+08:00","open":575,"high":590,"low":560,"close":585,"volume":50}}`)

	folded := nextLiveKCandle(t, liveKCandles)
	assert.Equal(t, "590", folded.High.String())
	assert.Equal(t, "560", folded.Low.String())
}

func TestAPushedCandleWhoseTimeCannotBeReadIsSkipped(t *testing.T) {
	// One candle nobody can place in time is one candle. Ending the feed over it would
	// cost the viewer everything that came after.
	stream := newFugleStreamUnderTest(t)
	liveKCandles := stream.follow(t)

	stream.push(fugleCandlePush("yesterday", "575", "100"))
	stream.push(fugleCandlePush("2026-09-08T10:00:00.000+08:00", "575", "8450"))

	assert.Equal(t, taipeiAt(t, "2026-09-08T10:00:00+08:00").UTC(),
		nextLiveKCandle(t, liveKCandles).OpenTime)
}
