package service

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ErrKCandleFollowStopped is reported to a viewer arriving after the service has
// been asked to stop. It is a sentinel so that a caller can tell "we are shutting
// down" apart from "this market cannot be followed".
var ErrKCandleFollowStopped = errors.New("k candle follow stopped")

// viewerBufferSize is how many updates a viewer may fall behind by before one is
// dropped. Dropping is deliberate: a viewer that cannot keep up must not be able to
// stall the follow for everybody else, and an update it missed is superseded by the
// next one anyway. Only a forming candle is ever dropped this way — see publish.
const viewerBufferSize = 8

// KCandleFollowService owns the life of every live follow: who is watching what,
// how many follows exist, and when one ends.
//
// The unit of following is the trading symbol, not the viewer. Ten people watching
// BTCUSDT are one follow, because the market only has one answer and asking for it
// ten times would cost ten connections to say the same thing. The last viewer
// leaving is what ends it — following a market nobody is looking at buys nothing
// the five-minute round would not deliver anyway.
//
// This is the first domain service in the project that holds state and outlives a
// request. It is here rather than a layer out because what it holds is a rule
// ("one follow per symbol, ending with the last viewer"), not a mechanism. Every
// rule that carries a number is delegated to KCandleFollowDomain, so this file is
// left with lifecycle alone.
type KCandleFollowService struct {
	liveMarketDataProxy   _interface.ILiveMarketDataProxy
	kCandleRepository     _interface.IKCandleRepository
	clockProxy            _interface.IClockProxy
	updateIntervalCeiling time.Duration
	quietTimeout          time.Duration
	maximumRetryDelay     time.Duration

	mutex   sync.Mutex
	follows map[string]*symbolFollow
	stopped bool
}

// symbolFollow is one market being followed, plus everyone currently looking at it.
// latestUpdate is kept so that somebody arriving mid-candle sees the shape now
// rather than an empty chart until the market next moves.
type symbolFollow struct {
	cancel       context.CancelFunc
	finished     chan struct{}
	viewers      map[int]chan dto.KCandleFollowUpdateDto
	nextViewerId int
	latestUpdate dto.KCandleFollowUpdateDto
	hasLatest    bool
	isStalled    bool
}

// NewKCandleFollowService takes the three timing rules once; every follow it starts
// is judged by them.
func NewKCandleFollowService(
	liveMarketDataProxy _interface.ILiveMarketDataProxy,
	kCandleRepository _interface.IKCandleRepository,
	clockProxy _interface.IClockProxy,
	updateIntervalCeiling time.Duration,
	quietTimeout time.Duration,
	maximumRetryDelay time.Duration,
) *KCandleFollowService {
	return &KCandleFollowService{
		liveMarketDataProxy:   liveMarketDataProxy,
		kCandleRepository:     kCandleRepository,
		clockProxy:            clockProxy,
		updateIntervalCeiling: updateIntervalCeiling,
		quietTimeout:          quietTimeout,
		maximumRetryDelay:     maximumRetryDelay,
		follows:               make(map[string]*symbolFollow),
	}
}

// WatchKCandles joins this viewer to the follow of one trading symbol, starting that follow
// if nobody was watching it yet, and hands back the updates they will receive.
//
// It is the only way in, and it answers four questions at once — is anyone
// following this, start if not, add this viewer, and how does this viewer leave —
// so that no caller has to sequence them. Leaving is the viewer's own context
// ending, which is why a dropped connection needs no separate rule: it is the same
// event.
//
// The returned channel is closed when the viewer leaves or the service stops.
func (kCandleFollowService *KCandleFollowService) WatchKCandles(
	executionContext context.Context, symbol string,
) (<-chan dto.KCandleFollowUpdateDto, error) {
	kCandleFollowService.mutex.Lock()

	if kCandleFollowService.stopped {
		kCandleFollowService.mutex.Unlock()

		return nil, ErrKCandleFollowStopped
	}

	follow, isFollowing := kCandleFollowService.follows[symbol]
	if !isFollowing {
		followContext, cancel := context.WithCancel(context.WithoutCancel(executionContext))
		follow = &symbolFollow{
			cancel:   cancel,
			finished: make(chan struct{}),
			viewers:  make(map[int]chan dto.KCandleFollowUpdateDto),
		}
		kCandleFollowService.follows[symbol] = follow

		go kCandleFollowService.run(followContext, symbol, follow)
	}

	viewerId := follow.nextViewerId
	follow.nextViewerId++
	updates := make(chan dto.KCandleFollowUpdateDto, viewerBufferSize)
	follow.viewers[viewerId] = updates

	// Somebody arriving mid-candle gets what the market looks like right now, so the
	// chart has something to continue from instead of waiting for the next move.
	if follow.hasLatest {
		updates <- follow.latestUpdate
	}

	// Arriving during an outage must not look like arriving during a quiet market.
	// Without this they would be handed the last candle and nothing else, and a
	// picture that stopped updating would look exactly like one that is live.
	if follow.isStalled {
		updates <- dto.KCandleFollowUpdateDto{
			Symbol: symbol,
			Status: dto.KCandleFollowStatusStalled,
		}
	}

	kCandleFollowService.mutex.Unlock()

	go func() {
		<-executionContext.Done()
		kCandleFollowService.leave(symbol, viewerId)
	}()

	return updates, nil
}

// Stop ends every follow and closes every viewer's updates. A viewer arriving after
// this is turned away rather than left waiting on a channel nothing will feed.
func (kCandleFollowService *KCandleFollowService) Stop() {
	kCandleFollowService.mutex.Lock()
	if kCandleFollowService.stopped {
		kCandleFollowService.mutex.Unlock()

		return
	}
	kCandleFollowService.stopped = true

	stoppedFollows := make([]*symbolFollow, 0, len(kCandleFollowService.follows))
	for symbol, follow := range kCandleFollowService.follows {
		stoppedFollows = append(stoppedFollows, follow)
		delete(kCandleFollowService.follows, symbol)
	}
	kCandleFollowService.mutex.Unlock()

	for _, follow := range stoppedFollows {
		follow.cancel()
		<-follow.finished
		kCandleFollowService.closeViewers(follow)
	}
}

// FollowedSymbolCount reports how many markets are being followed right now. It
// exists because "one follow per symbol, ending with the last viewer" is a rule
// with no other observable effect — without it the rule could only be checked by
// counting connections to an exchange.
func (kCandleFollowService *KCandleFollowService) FollowedSymbolCount() int {
	kCandleFollowService.mutex.Lock()
	defer kCandleFollowService.mutex.Unlock()

	return len(kCandleFollowService.follows)
}

// leave removes one viewer and, when they were the last, ends the follow itself.
func (kCandleFollowService *KCandleFollowService) leave(symbol string, viewerId int) {
	kCandleFollowService.mutex.Lock()

	follow, isFollowing := kCandleFollowService.follows[symbol]
	if !isFollowing {
		kCandleFollowService.mutex.Unlock()

		return
	}

	updates, isViewing := follow.viewers[viewerId]
	if !isViewing {
		kCandleFollowService.mutex.Unlock()

		return
	}
	delete(follow.viewers, viewerId)
	close(updates)

	if len(follow.viewers) > 0 {
		kCandleFollowService.mutex.Unlock()

		return
	}
	delete(kCandleFollowService.follows, symbol)
	kCandleFollowService.mutex.Unlock()

	follow.cancel()
}

// run keeps one market followed for as long as anyone is watching it. Every time
// the feed ends — refused, dropped, or gone silent — the viewers are told, the
// wait grows, and it tries again. It never gives up; only the last viewer leaving
// ends it.
func (kCandleFollowService *KCandleFollowService) run(
	executionContext context.Context, symbol string, follow *symbolFollow,
) {
	defer close(follow.finished)

	followDomain := domains.NewKCandleFollowDomain(
		kCandleFollowService.updateIntervalCeiling,
		kCandleFollowService.quietTimeout,
		kCandleFollowService.maximumRetryDelay,
		kCandleFollowService.clockProxy.Now(),
	)

	for {
		liveKCandles, followError := kCandleFollowService.liveMarketDataProxy.
			FollowKCandles(executionContext, symbol)
		if followError == nil {
			followDomain.MarkFollowing(kCandleFollowService.clockProxy.Now())
			kCandleFollowService.consume(executionContext, follow, followDomain, liveKCandles)
		} else {
			log.Printf("live k candle follow: %s could not be followed: %v", symbol, followError)
		}

		// Whether it never connected or stopped delivering, from the viewer's side it
		// is the same news: the picture is no longer live.
		kCandleFollowService.publish(follow, dto.KCandleFollowUpdateDto{
			Symbol: symbol,
			Status: dto.KCandleFollowStatusStalled,
		})

		if !waitOrDone(executionContext, followDomain.NextRetryDelay()) {
			return
		}
	}
}

// consume passes on what the feed reports for as long as it keeps reporting. It
// returns when the feed ends or falls silent for too long — a connection that looks
// open but has stopped delivering is how this kind of channel usually dies, and a
// viewer must not be left watching a frozen picture that claims to be live.
func (kCandleFollowService *KCandleFollowService) consume(
	executionContext context.Context,
	follow *symbolFollow,
	followDomain *domains.KCandleFollowDomain,
	liveKCandles <-chan vo.LiveKCandleVo,
) {
	quietCheck := time.NewTicker(followDomain.QuietCheckInterval())
	defer quietCheck.Stop()

	for {
		select {
		case <-executionContext.Done():
			return

		case liveKCandle, isDelivering := <-liveKCandles:
			if !isDelivering {
				return
			}
			kCandleFollowService.report(executionContext, follow, followDomain, liveKCandle)

		case <-quietCheck.C:
			if followDomain.HasGoneQuiet(kCandleFollowService.clockProxy.Now()) {
				return
			}
		}
	}
}

// report carries one reported candle to the viewers and, when it is that candle's
// last word, into storage. Storing goes through the ordinary K candle rules — the
// same road a fetched candle takes — so a candle that breaks one is skipped with a
// record and the follow carries on.
func (kCandleFollowService *KCandleFollowService) report(
	executionContext context.Context,
	follow *symbolFollow,
	followDomain *domains.KCandleFollowDomain,
	liveKCandle vo.LiveKCandleVo,
) {
	now := kCandleFollowService.clockProxy.Now()
	if !followDomain.Admit(liveKCandle, now) {
		return
	}

	status := dto.KCandleFollowStatusForming
	if liveKCandle.Closed {
		status = dto.KCandleFollowStatusClosed
	}

	kCandleFollowService.publish(follow, dto.KCandleFollowUpdateDto{
		Symbol:  liveKCandle.Symbol,
		Status:  status,
		KCandle: liveKCandle.ToDto(),
	})

	if !liveKCandle.Closed {
		return
	}

	kCandleDomain, validationError := domains.NewKCandleDomain(liveKCandle.ToWriteDto(), now)
	if validationError != nil {
		log.Printf("live k candle follow: %s at %s skipped: %v",
			liveKCandle.Symbol, liveKCandle.OpenTime.UTC(), validationError)

		return
	}

	if _, saveError := kCandleFollowService.kCandleRepository.
		Save(executionContext, kCandleDomain.ToEntity()); saveError != nil {
		log.Printf("live k candle follow: %s at %s not stored: %v",
			liveKCandle.Symbol, liveKCandle.OpenTime.UTC(), saveError)
	}
}

// publish hands one update to everyone watching, and remembers it for whoever
// arrives next.
//
// A viewer too far behind to take it loses this one rather than holding up the
// rest. That is safe for a forming candle, which the next update supersedes; a
// closed candle is this candle's last word, but losing it from a screen does not
// lose it from the system — it was already stored.
func (kCandleFollowService *KCandleFollowService) publish(
	follow *symbolFollow, update dto.KCandleFollowUpdateDto,
) {
	kCandleFollowService.mutex.Lock()
	defer kCandleFollowService.mutex.Unlock()

	// Stalled carries no candle, so it must not become the shape handed to whoever
	// arrives next — they would be drawn a candle of zeros. It is remembered as a
	// state instead, and told to them separately.
	follow.isStalled = update.Status == dto.KCandleFollowStatusStalled
	if !follow.isStalled {
		follow.latestUpdate = update
		follow.hasLatest = true
	}

	for _, updates := range follow.viewers {
		select {
		case updates <- update:
		default:
		}
	}
}

// closeViewers ends every viewer's updates for a follow that is over.
func (kCandleFollowService *KCandleFollowService) closeViewers(follow *symbolFollow) {
	kCandleFollowService.mutex.Lock()
	defer kCandleFollowService.mutex.Unlock()

	for viewerId, updates := range follow.viewers {
		delete(follow.viewers, viewerId)
		close(updates)
	}
}

// waitOrDone waits out the retry gap, reporting false if the follow ended first so
// that a follow nobody is watching stops immediately rather than after the wait.
func waitOrDone(executionContext context.Context, delay time.Duration) bool {
	timer := time.NewTimer(delay)
	defer timer.Stop()

	select {
	case <-executionContext.Done():
		return false
	case <-timer.C:
		return true
	}
}
