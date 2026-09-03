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

// KCandleFollowService owns which markets are being followed: one follow per
// trading symbol, started by the first viewer and ended by the last.
//
// Ten people watching BTCUSDT are one follow, because the market has only one
// answer and asking for it ten times would open ten connections to hear the same
// thing. Following a market nobody is looking at buys nothing the five-minute round
// would not deliver anyway, which is why the last viewer leaving ends it.
//
// This is the first domain service in the project that holds state and outlives a
// request. It is here rather than a layer out because what it holds is a rule, not
// a mechanism. Everything that is a mechanism is delegated: the rules that carry a
// number go to KCandleFollowDomain, and the viewers of one market go to
// symbolFollow — leaving this file with the registry and the round trip to the
// source.
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

// WatchKCandles joins this viewer to the follow of one trading symbol, starting that
// follow if nobody was watching it yet, and hands back the updates they will
// receive.
//
// It is the only way in, and it answers four questions at once — is anyone following
// this, start if not, add this viewer, and how does this viewer leave — so that no
// caller has to sequence them. Leaving is the viewer's own context ending, which is
// why a dropped connection needs no separate rule: it is the same event.
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
		// The follow outlives the viewer who started it, so it must not inherit their
		// context — the next viewer would be following a market on a cancelled one.
		followContext, cancel := context.WithCancel(context.WithoutCancel(executionContext))
		follow = newSymbolFollow(symbol, cancel)
		kCandleFollowService.follows[symbol] = follow

		go kCandleFollowService.run(followContext, follow)
	}

	viewerId, updates := follow.join()
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
		follow.end()
	}
}

// FollowedSymbolCount reports how many markets are being followed right now. It
// exists because "one follow per symbol, ending with the last viewer" is a rule with
// no other observable effect — without it the rule could only be checked by counting
// connections to an exchange.
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

	if !follow.leave(viewerId) {
		kCandleFollowService.mutex.Unlock()

		return
	}
	delete(kCandleFollowService.follows, symbol)
	kCandleFollowService.mutex.Unlock()

	follow.cancel()
}

// run keeps one market followed for as long as anyone is watching it. Every time the
// feed ends — refused, dropped, or gone silent — the viewers are told, the wait
// grows, and it tries again. It never gives up; only the last viewer leaving ends it.
func (kCandleFollowService *KCandleFollowService) run(
	executionContext context.Context, follow *symbolFollow,
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
			FollowKCandles(executionContext, follow.symbol)
		if followError == nil {
			followDomain.MarkFollowing(kCandleFollowService.clockProxy.Now())
			kCandleFollowService.consume(executionContext, follow, followDomain, liveKCandles)
		} else {
			log.Printf("live k candle follow: %s could not be followed: %v",
				follow.symbol, followError)
		}

		follow.publishStalled()

		if !waitOrDone(executionContext, followDomain.NextRetryDelay()) {
			return
		}
	}
}

// consume decides when this follow has something to do: a candle arrived, the feed
// ended, or it has been silent long enough to count as dead.
//
// A connection that looks open but has stopped delivering is how this kind of feed
// usually fails, and a viewer must not be left watching a frozen picture that claims
// to be live.
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

// report decides what one reported candle amounts to: whether it is worth passing
// on, and whether it is that candle's last word and therefore worth storing.
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

	follow.publish(dto.KCandleFollowUpdateDto{
		Symbol:  liveKCandle.Symbol,
		Status:  status,
		KCandle: liveKCandle.ToDto(),
	})

	if !liveKCandle.Closed {
		return
	}

	kCandleFollowService.store(executionContext, liveKCandle, now)
}

// store puts a candle that has closed where the rest of the system can see it,
// through the ordinary K candle rules — the same road a fetched candle takes.
//
// It does not report failure upwards: a candle that breaks a rule, or that storage
// refuses, is one the five-minute round will deal with, and neither reason is worth
// taking the picture away from whoever is watching.
func (kCandleFollowService *KCandleFollowService) store(
	executionContext context.Context, liveKCandle vo.LiveKCandleVo, now time.Time,
) {
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
