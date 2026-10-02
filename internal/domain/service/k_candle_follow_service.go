package service

import (
	"context"
	"errors"
	"fmt"
	"log"
	"sync"
	"time"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// liveKCandleSnapshotRetention keeps snapshots long enough for a relay that reconnected after a short outage, and no longer.
const liveKCandleSnapshotRetention = 10 * time.Minute

// ErrKCandleFollowStopped lets callers tell "shutting down" apart from "this market cannot be followed".
var ErrKCandleFollowStopped = errors.New("k candle follow stopped")

// KCandleFollowService keeps one live follow per spot trading symbol, started by the first viewer and ended by the last, so many viewers share one source connection.
type KCandleFollowService struct {
	kCandleRepository       _interface.IKCandleRepository
	tradingSymbolRepository _interface.ITradingSymbolRepository
	clockProxy              _interface.IClockProxy
	marketCatalogDomain     domains.MarketCatalogDomain
	updateIntervalCeiling   time.Duration
	feed                    *kCandleFollowFeed
	// relayFeed follows a roster from what the replica on duty saw, for a replica off duty.
	relayFeed                     *kCandleFollowFeed
	liveKCandleSnapshotRepository _interface.ILiveKCandleSnapshotRepository

	mutex   sync.Mutex
	follows map[string]*kCandleFollowSymbol
	// channels is keyed by the symbol set on each line, so a roster producing the same set matches the line already open.
	channels map[string]*kCandleFollowChannel
	stopped  bool
}

func NewKCandleFollowService(
	liveMarketDataProxy _interface.ILiveMarketDataProxy,
	kCandleRepository _interface.IKCandleRepository,
	tradingSymbolRepository _interface.ITradingSymbolRepository,
	clockProxy _interface.IClockProxy,
	marketCatalogDomain domains.MarketCatalogDomain,
	updateIntervalCeiling time.Duration,
	quietTimeout time.Duration,
	maximumRetryDelay time.Duration,
	liveKCandleSnapshotRepository _interface.ILiveKCandleSnapshotRepository,
	relayInterval time.Duration,
) *KCandleFollowService {
	kCandleFollowService := &KCandleFollowService{
		liveKCandleSnapshotRepository: liveKCandleSnapshotRepository,
		kCandleRepository:             kCandleRepository,
		tradingSymbolRepository:       tradingSymbolRepository,
		clockProxy:                    clockProxy,
		marketCatalogDomain:           marketCatalogDomain,
		updateIntervalCeiling:         updateIntervalCeiling,
		follows:                       make(map[string]*kCandleFollowSymbol),
		channels:                      make(map[string]*kCandleFollowChannel),
	}
	kCandleFollowService.feed = newKCandleFollowFeed(
		liveMarketDataProxy, clockProxy, quietTimeout, maximumRetryDelay, kCandleFollowService.report)
	kCandleFollowService.relayFeed = newKCandleFollowFeed(
		newKCandleSnapshotRelay(liveKCandleSnapshotRepository, clockProxy, relayInterval, quietTimeout),
		clockProxy, quietTimeout, maximumRetryDelay, kCandleFollowService.reportRelayed)

	return kCandleFollowService
}

// WatchKCandles joins the viewer to the symbol's follow, starting it if needed; the viewer leaves when their context ends, and the returned channel closes then or when the service stops.
func (kCandleFollowService *KCandleFollowService) WatchKCandles(
	executionContext context.Context, symbol string,
) (<-chan dto.KCandleFollowUpdateDto, error) {
	registeredSymbol, isRegistered, findError := kCandleFollowService.tradingSymbolRepository.
		FindBySymbol(executionContext, symbol)
	if findError != nil {
		return nil, findError
	}

	// The market comes only from registration; it is deliberately never guessed from the symbol name.
	if !isRegistered {
		return nil, fmt.Errorf("%w: %s", domains.ErrTradingSymbolNotRegistered, symbol)
	}

	marketDomain := kCandleFollowService.marketCatalogDomain.MarketOf(registeredSymbol.Market)

	kCandleFollowService.mutex.Lock()

	if kCandleFollowService.stopped {
		kCandleFollowService.mutex.Unlock()

		return nil, ErrKCandleFollowStopped
	}

	follow, isFollowing := kCandleFollowService.follows[symbol]
	if !isFollowing {
		if marketDomain.FollowsFixedRoster() {
			// A symbol off a fixed-roster market's roster is told so rather than given a live-looking follow the roster never asked for.
			kCandleFollowService.mutex.Unlock()

			return kCandleFollowService.noLivePlaceUpdates(
				executionContext, symbol, marketDomain), nil
		}

		follow = newKCandleFollowSymbol(symbol, marketDomain.Value(), false,
			domains.NewViewerUpdateThrottleDomain(
				kCandleFollowService.updateIntervalCeiling,
				kCandleFollowService.clockProxy.Now()))
		kCandleFollowService.follows[symbol] = follow
		kCandleFollowService.openChannel(
			executionContext,
			vo.NewLiveFollowChannelVo(marketDomain.Value(), []string{symbol}),
			map[string]*kCandleFollowSymbol{symbol: follow}, false)
	}

	viewerId, updates := follow.join()
	kCandleFollowService.mutex.Unlock()

	// Capture the joined follow itself: viewer ids restart at zero per follow, and a rostered follow may be replaced before this viewer leaves.
	joinedFollow := follow
	go func() {
		<-executionContext.Done()
		kCandleFollowService.leave(symbol, joinedFollow, viewerId)
	}()

	return updates, nil
}

// RefreshFixedFollows gives the live places of ceiling-limited markets to the earliest-registered watched symbols, followed from the source; only the replica on duty does this.
// Places are released before new ones are taken, because exceeding the source's limit even briefly drops every place.
func (kCandleFollowService *KCandleFollowService) RefreshFixedFollows(
	executionContext context.Context,
) error {
	return kCandleFollowService.followRoster(executionContext, false)
}

// RefreshRelayedFollows follows the same roster from what the replica on duty saw, for a replica off duty, so its viewers see live updates without spending a place.
func (kCandleFollowService *KCandleFollowService) RefreshRelayedFollows(
	executionContext context.Context,
) error {
	return kCandleFollowService.followRoster(executionContext, true)
}

// followRoster, shared by both ways of following a roster, swaps any line of the other way for one of this way.
func (kCandleFollowService *KCandleFollowService) followRoster(
	executionContext context.Context, relayed bool,
) error {
	watchedSymbols, findError := kCandleFollowService.tradingSymbolRepository.FindWatched(
		executionContext)
	if findError != nil {
		return findError
	}

	// One clock reading for the whole round, so the roster and the end reasons use the same trading day.
	currentTime := kCandleFollowService.clockProxy.Now()

	// Same roster the console reads, so what it promises and what this does cannot drift.
	rosterDomain := domains.NewLiveFollowRosterDomain(
		watchedSymbols, kCandleFollowService.marketCatalogDomain, currentTime)

	wantedChannels := rosterDomain.Channels()

	kCandleFollowService.retireUnwantedChannels(wantedChannels, relayed, currentTime)

	kCandleFollowService.startMissingChannels(executionContext, wantedChannels, relayed)

	return nil
}

// retireUnwantedChannels ends rostered lines the wanted roster no longer names and tells their viewers why.
func (kCandleFollowService *KCandleFollowService) retireUnwantedChannels(
	wantedChannels []vo.LiveFollowChannelVo, relayed bool, currentTime time.Time,
) {
	// Lines and symbols are retired separately: a roster change replaces a line, but symbols on both old and new lines must not be told they lost their place.
	departing, retired := kCandleFollowService.takeDepartedChannels(wantedChannels, relayed)

	retiredSymbols := make(map[string]bool, len(retired))
	for _, retiredFollow := range retired {
		retiredSymbols[retiredFollow.symbol] = true
	}

	for _, departingChannel := range departing {
		departingChannel.end()
		departingChannel.publishStalled(retiredSymbols)
	}

	// The end reason must be decided now; afterwards "market closed" and "place taken" look identical.
	for _, retiredFollow := range retired {
		if kCandleFollowService.marketCatalogDomain.MarketOf(string(retiredFollow.market)).
			IsOpen(currentTime) {
			retiredFollow.publishUnavailable()
		} else {
			retiredFollow.publishMarketClosed()
		}

		retiredFollow.end()
	}
}

// takeDepartedChannels removes rostered channels whose key the roster no longer wants and returns them to be ended outside the lock.
// It exists to scope the lock with defer, since ending a line can wait out a thirty-second retry.
func (kCandleFollowService *KCandleFollowService) takeDepartedChannels(
	wantedChannels []vo.LiveFollowChannelVo, relayed bool,
) ([]*kCandleFollowChannel, []*kCandleFollowSymbol) {
	kCandleFollowService.mutex.Lock()
	defer kCandleFollowService.mutex.Unlock()

	if kCandleFollowService.stopped {
		return nil, nil
	}

	isWantedChannel := make(map[string]bool, len(wantedChannels))
	isWantedSymbol := make(map[string]bool)
	for _, wantedChannel := range wantedChannels {
		isWantedChannel[wantedChannel.Key] = true
		for _, symbol := range wantedChannel.Symbols {
			isWantedSymbol[symbol] = true
		}
	}

	departing := make([]*kCandleFollowChannel, 0)
	retired := make([]*kCandleFollowSymbol, 0)
	for key, openChannel := range kCandleFollowService.channels {
		// A wanted line followed the other way is replaced too, so a replica never follows the source off duty or relays on duty.
		if (isWantedChannel[key] && openChannel.relayed == relayed) || !openChannel.isRostered() {
			continue
		}

		departing = append(departing, openChannel)
		delete(kCandleFollowService.channels, key)

		// A symbol the new roster still wants keeps its viewers across the line replacement; only unwanted ones are retired.
		for symbol, follow := range openChannel.follows {
			if isWantedSymbol[symbol] {
				continue
			}

			retired = append(retired, follow)
			delete(kCandleFollowService.follows, symbol)
		}
	}

	return departing, retired
}

// startMissingChannels opens every wanted channel not already open, as a separate method to scope the second lock hold.
func (kCandleFollowService *KCandleFollowService) startMissingChannels(
	executionContext context.Context, wantedChannels []vo.LiveFollowChannelVo, relayed bool,
) {
	kCandleFollowService.mutex.Lock()
	defer kCandleFollowService.mutex.Unlock()

	if kCandleFollowService.stopped {
		return
	}

	now := kCandleFollowService.clockProxy.Now()
	for _, wantedChannel := range wantedChannels {
		if _, isOpen := kCandleFollowService.channels[wantedChannel.Key]; isOpen {
			continue
		}

		follows := make(map[string]*kCandleFollowSymbol, len(wantedChannel.Symbols))
		for _, symbol := range wantedChannel.Symbols {
			// Reuse a registry that survived a line replacement so its viewers keep their stream.
			follow, isAlreadyRegistered := kCandleFollowService.follows[symbol]
			if !isAlreadyRegistered {
				follow = newKCandleFollowSymbol(symbol, wantedChannel.Market, true,
					domains.NewViewerUpdateThrottleDomain(
						kCandleFollowService.updateIntervalCeiling, now))
				kCandleFollowService.follows[symbol] = follow
			}
			follows[symbol] = follow
		}

		kCandleFollowService.openChannel(executionContext, wantedChannel, follows, relayed)
	}
}

// openChannel starts a channel with the lock held; the channel outlives its requester, so it must not inherit their cancellable context.
func (kCandleFollowService *KCandleFollowService) openChannel(
	executionContext context.Context,
	channel vo.LiveFollowChannelVo,
	follows map[string]*kCandleFollowSymbol,
	relayed bool,
) {
	channelContext, cancel := context.WithCancel(context.WithoutCancel(executionContext))
	openChannel := newKCandleFollowChannel(channel, follows, cancel, relayed)
	kCandleFollowService.channels[channel.Key] = openChannel

	if relayed {
		go kCandleFollowService.relayFeed.keep(channelContext, openChannel)

		return
	}
	go kCandleFollowService.feed.keep(channelContext, openChannel)
}

// noLivePlaceUpdates sends a viewer with no live place one market-closed or unavailable status, then keeps the channel open until they leave, since a closed channel means "feed ended".
func (kCandleFollowService *KCandleFollowService) noLivePlaceUpdates(
	executionContext context.Context, symbol string, marketDomain domains.MarketDomain,
) <-chan dto.KCandleFollowUpdateDto {
	status := dto.KCandleFollowStatusMarketClosed
	if marketDomain.IsOpen(kCandleFollowService.clockProxy.Now()) {
		status = dto.KCandleFollowStatusUnavailable
	}

	updates := make(chan dto.KCandleFollowUpdateDto, 1)
	updates <- dto.KCandleFollowUpdateDto{Symbol: symbol, Status: status}

	go func() {
		<-executionContext.Done()
		close(updates)
	}()

	return updates
}

// Stop ends every follow and closes all viewers' updates; viewers arriving afterwards are turned away.
func (kCandleFollowService *KCandleFollowService) Stop() {
	kCandleFollowService.mutex.Lock()
	if kCandleFollowService.stopped {
		kCandleFollowService.mutex.Unlock()

		return
	}
	kCandleFollowService.stopped = true

	stoppedChannels := make([]*kCandleFollowChannel, 0, len(kCandleFollowService.channels))
	for key, openChannel := range kCandleFollowService.channels {
		stoppedChannels = append(stoppedChannels, openChannel)
		delete(kCandleFollowService.channels, key)
	}
	stoppedFollows := make([]*kCandleFollowSymbol, 0, len(kCandleFollowService.follows))
	for _, follow := range kCandleFollowService.follows {
		stoppedFollows = append(stoppedFollows, follow)
	}
	clear(kCandleFollowService.follows)
	kCandleFollowService.mutex.Unlock()

	for _, openChannel := range stoppedChannels {
		openChannel.end()
	}
	for _, follow := range stoppedFollows {
		follow.end()
	}
}

// FollowedSymbolCount is the only observable effect of the one-follow-per-symbol rule.
func (kCandleFollowService *KCandleFollowService) FollowedSymbolCount() int {
	kCandleFollowService.mutex.Lock()
	defer kCandleFollowService.mutex.Unlock()

	return len(kCandleFollowService.follows)
}

func (kCandleFollowService *KCandleFollowService) leave(
	symbol string, joinedFollow *kCandleFollowSymbol, viewerId int,
) {
	kCandleFollowService.mutex.Lock()

	follow, isFollowing := kCandleFollowService.follows[symbol]
	// Check it is still the follow this viewer joined; a replacement reuses ids from zero and would otherwise lose another viewer's stream.
	if !isFollowing || follow != joinedFollow {
		kCandleFollowService.mutex.Unlock()

		return
	}

	if !follow.leave(viewerId) {
		kCandleFollowService.mutex.Unlock()

		return
	}
	delete(kCandleFollowService.follows, symbol)

	// A viewer-started follow is alone on its channel, so the last viewer takes the line with it.
	channelKey := vo.NewLiveFollowChannelVo(follow.market, []string{symbol}).Key
	departingChannel, isOpen := kCandleFollowService.channels[channelKey]
	if isOpen {
		delete(kCandleFollowService.channels, channelKey)
	}
	kCandleFollowService.mutex.Unlock()

	if isOpen {
		departingChannel.cancel()
	}
}

// report passes the candle on through the throttle and stores it once closed; a rostered candle is also handed to the replicas off duty.
func (kCandleFollowService *KCandleFollowService) report(
	executionContext context.Context,
	follow *kCandleFollowSymbol,
	liveKCandle vo.LiveKCandleVo,
) {
	now := kCandleFollowService.clockProxy.Now()

	if follow.isOnARoster {
		if saveError := kCandleFollowService.liveKCandleSnapshotRepository.Save(executionContext,
			domains.NewLiveKCandleSnapshotDomain(liveKCandle, now).ToEntity()); saveError != nil {
			log.Printf("live k candle follow: %s could not be passed on to the other replicas: %v",
				liveKCandle.Symbol, saveError)
		}
		// Trimmed as each minute closes, so the table holds only what a relay could still be reading.
		if liveKCandle.Closed {
			if trimError := kCandleFollowService.liveKCandleSnapshotRepository.DeleteObservedBefore(
				executionContext, now.Add(-liveKCandleSnapshotRetention)); trimError != nil {
				log.Printf("live k candle follow: old snapshots could not be dropped: %v", trimError)
			}
		}
	}

	if !follow.pass(liveKCandle, now) || !liveKCandle.Closed {
		return
	}

	kCandleFollowService.store(executionContext, liveKCandle, now)
}

// reportRelayed only passes the candle on: the replica on duty already stores the closed one.
func (kCandleFollowService *KCandleFollowService) reportRelayed(
	_ context.Context,
	follow *kCandleFollowSymbol,
	liveKCandle vo.LiveKCandleVo,
) {
	follow.pass(liveKCandle, kCandleFollowService.clockProxy.Now())
}

// store saves a closed candle through the ordinary K candle rules, only logging failures because the scheduled round will cover them.
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
