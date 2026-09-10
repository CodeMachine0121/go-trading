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

// ErrKCandleFollowStopped is reported to a viewer arriving after the service has
// been asked to stop. It is a sentinel so that a caller can tell "we are shutting
// down" apart from "this market cannot be followed".
var ErrKCandleFollowStopped = errors.New("k candle follow stopped")

// KCandleFollowService owns which markets are being followed: one follow per
// trading symbol, started by the first viewer and ended by the last.
//
// Ten people watching BTCUSDT are one follow, because the market has only one
// answer and asking for it ten times would open ten connections to hear the same
// thing. Following a market nobody is looking at buys nothing the scheduled round
// would not deliver anyway, which is why the last viewer leaving ends it.
//
// This is the first domain service in the project that holds state and outlives a
// request. It is here rather than a layer out because what it holds is a rule, not
// a mechanism. Everything that is a mechanism is delegated: the rules that carry a
// number go to LiveChannelHealthDomain and ViewerUpdateThrottleDomain, the line to
// kCandleFollowChannel, and the viewers of one market to kCandleFollowSymbol —
// leaving this file with the registry and the round trip to the source.
type KCandleFollowService struct {
	liveMarketDataProxy     _interface.ILiveMarketDataProxy
	kCandleRepository       _interface.IKCandleRepository
	tradingSymbolRepository _interface.ITradingSymbolRepository
	clockProxy              _interface.IClockProxy
	marketCatalogDomain     domains.MarketCatalogDomain
	updateIntervalCeiling   time.Duration
	quietTimeout            time.Duration
	maximumRetryDelay       time.Duration

	mutex sync.Mutex
	// follows is every symbol with a viewer registry, whether or not anybody is
	// watching it. It is keyed by symbol because that is how a viewer asks.
	follows map[string]*kCandleFollowSymbol
	// channels is every line currently open, keyed by the set of symbols travelling
	// on it. Keying by the set is what makes rebuilding-on-change free: a roster that
	// produced the same set produces the same key and matches what is already there.
	channels map[string]*kCandleFollowChannel
	stopped  bool
}

// NewKCandleFollowService takes the three timing rules once; every follow it starts
// is judged by them.
func NewKCandleFollowService(
	liveMarketDataProxy _interface.ILiveMarketDataProxy,
	kCandleRepository _interface.IKCandleRepository,
	tradingSymbolRepository _interface.ITradingSymbolRepository,
	clockProxy _interface.IClockProxy,
	marketCatalogDomain domains.MarketCatalogDomain,
	updateIntervalCeiling time.Duration,
	quietTimeout time.Duration,
	maximumRetryDelay time.Duration,
) *KCandleFollowService {
	return &KCandleFollowService{
		liveMarketDataProxy:     liveMarketDataProxy,
		kCandleRepository:       kCandleRepository,
		tradingSymbolRepository: tradingSymbolRepository,
		clockProxy:              clockProxy,
		marketCatalogDomain:     marketCatalogDomain,
		updateIntervalCeiling:   updateIntervalCeiling,
		quietTimeout:            quietTimeout,
		maximumRetryDelay:       maximumRetryDelay,
		follows:                 make(map[string]*kCandleFollowSymbol),
		channels:                make(map[string]*kCandleFollowChannel),
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
	registeredSymbol, isRegistered, findError := kCandleFollowService.tradingSymbolRepository.
		FindBySymbol(executionContext, symbol)
	if findError != nil {
		return nil, findError
	}

	// Without a registration there is no market, and without a market there is no
	// source to follow. Guessing one from the shape of the name is the rule this
	// system deliberately does not have.
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
		if marketDomain.HasFollowCeiling() {
			// This market hands its live places out from a roster. A viewer arriving
			// for a symbol that holds none is told so rather than left watching a
			// picture that looks live and is not — and rather than being given a place
			// that would put the market over what its source allows.
			kCandleFollowService.mutex.Unlock()

			return kCandleFollowService.noLivePlaceUpdates(
				executionContext, symbol, marketDomain), nil
		}

		// A market with no ceiling puts one symbol on a channel, so the channel a
		// viewer starts carries exactly what they came for.
		follow = newKCandleFollowSymbol(symbol, marketDomain.Value(), false,
			domains.NewViewerUpdateThrottleDomain(
				kCandleFollowService.updateIntervalCeiling,
				kCandleFollowService.clockProxy.Now()))
		kCandleFollowService.follows[symbol] = follow
		kCandleFollowService.openChannel(
			executionContext,
			vo.NewLiveFollowChannelVo(marketDomain.Value(), []string{symbol}),
			map[string]*kCandleFollowSymbol{symbol: follow})
	}

	viewerId, updates := follow.join()
	kCandleFollowService.mutex.Unlock()

	// The follow this viewer actually joined is carried, not just its name. Viewer ids
	// start again at zero for every follow, and a rostered follow can be retired and a
	// replacement started while a viewer of the old one is still writing to a wedged
	// client — so by the time this fires, that id may belong to somebody else's stream.
	joinedFollow := follow
	go func() {
		<-executionContext.Done()
		kCandleFollowService.leave(symbol, joinedFollow, viewerId)
	}()

	return updates, nil
}

// RefreshFixedFollows works out which symbols each market with a live-place ceiling
// should be following right now, and makes that true.
//
// Markets whose source limits how many symbols may be followed at once do not hand
// their places out to whoever asks first. Places go to the earliest-registered
// symbols on the watchlist, so which ones are live is a fact somebody can state and
// change — rather than a race between viewers, where the losers get a chart that
// looks live and is not.
//
// Places are given up before any are taken. Going over the source's limit for even a
// moment costs every place at once, so the order here is not an optimisation but the
// difference between a market being followed and none of it being.
func (kCandleFollowService *KCandleFollowService) RefreshFixedFollows(
	executionContext context.Context,
) error {
	watchedSymbols, findError := kCandleFollowService.tradingSymbolRepository.FindWatched(
		executionContext)
	if findError != nil {
		return findError
	}

	// One reading of the clock for the whole round. Asking twice would let the roster
	// be decided in one trading day and the reason a follow ended in the next.
	currentTime := kCandleFollowService.clockProxy.Now()

	// The very same roster the console reads when it says which symbols can be
	// followed, so what it promises and what this does cannot drift apart.
	rosterDomain := domains.NewLiveFollowRosterDomain(
		watchedSymbols, kCandleFollowService.marketCatalogDomain, currentTime)

	wantedChannels := rosterDomain.Channels()

	// A line and the viewers travelling on it are retired separately, because a
	// roster change usually retires the line without retiring anybody: gaining or
	// losing one symbol makes a different line, and the symbols that were on the old
	// one and are on the new one never left the roster at all. Telling them their
	// symbol has no place — and closing their stream — for a line that is coming
	// back in the same breath would be a lie about the one thing they asked.
	departing, retired := kCandleFollowService.takeDepartedChannels(wantedChannels)

	retiredSymbols := make(map[string]bool, len(retired))
	for _, retiredFollow := range retired {
		retiredSymbols[retiredFollow.symbol] = true
	}

	for _, departingChannel := range departing {
		departingChannel.end()
		departingChannel.publishStalled(retiredSymbols)
	}

	// Why a follow is ending is decided here, where both answers are still in hand.
	// Once it has ended, all that is left is a symbol that is no longer on a roster —
	// and that looks identical whether the day is over or somebody else took its
	// place.
	for _, retiredFollow := range retired {
		if kCandleFollowService.marketCatalogDomain.MarketOf(string(retiredFollow.market)).
			IsOpen(currentTime) {
			retiredFollow.publishUnavailable()
		} else {
			retiredFollow.publishMarketClosed()
		}

		retiredFollow.end()
	}

	kCandleFollowService.startMissingChannels(executionContext, wantedChannels)

	return nil
}

// takeDepartedChannels removes every rostered channel the roster no longer asks for
// and hands them back to be ended outside the lock.
//
// "No longer asks for" is decided by key alone, so a channel that lost a symbol and
// a channel that lost its whole market are the same case. Nothing here compares two
// sets of symbols, because the key already is the set.
// It is a method of its own rather than part of the round for one reason: it draws
// the stretch the lock is held for. Ending a line waits on a goroutine that may be
// sleeping out a thirty-second retry, and holding this lock for that long would stop
// anybody opening a chart. Written inline, the release would have to be remembered
// by hand at every way out of it.
func (kCandleFollowService *KCandleFollowService) takeDepartedChannels(
	wantedChannels []vo.LiveFollowChannelVo,
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
		if isWantedChannel[key] || !openChannel.isRostered() {
			continue
		}

		departing = append(departing, openChannel)
		delete(kCandleFollowService.channels, key)

		// A symbol the new roster still wants keeps its viewers and its place in the
		// registry — the line under it is being replaced, which is not something its
		// viewers asked about and not something they should have to notice beyond a
		// moment's pause. Only a symbol nobody asked for again is really retired.
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

// startMissingChannels opens every channel the roster asks for that is not open
// already.
//
// Its own method for the same reason as its counterpart above: it draws the second
// stretch the lock is held for, after the lines that departed have been let go of
// outside it.
func (kCandleFollowService *KCandleFollowService) startMissingChannels(
	executionContext context.Context, wantedChannels []vo.LiveFollowChannelVo,
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
			// A symbol whose registry survived a line being replaced keeps it, so the
			// people watching it keep their stream and their place in the queue. A new
			// one is only built for a symbol nobody was following a moment ago.
			follow, isAlreadyRegistered := kCandleFollowService.follows[symbol]
			if !isAlreadyRegistered {
				follow = newKCandleFollowSymbol(symbol, wantedChannel.Market, true,
					domains.NewViewerUpdateThrottleDomain(
						kCandleFollowService.updateIntervalCeiling, now))
				kCandleFollowService.follows[symbol] = follow
			}
			follows[symbol] = follow
		}

		kCandleFollowService.openChannel(executionContext, wantedChannel, follows)
	}
}

// openChannel starts one channel running. The caller holds the lock; the channel
// outlives the viewer or the round that asked for it, so it must not inherit their
// context — the next viewer would be following a market on a cancelled one.
func (kCandleFollowService *KCandleFollowService) openChannel(
	executionContext context.Context,
	channel vo.LiveFollowChannelVo,
	follows map[string]*kCandleFollowSymbol,
) {
	channelContext, cancel := context.WithCancel(context.WithoutCancel(executionContext))
	openChannel := newKCandleFollowChannel(channel, follows, cancel)
	kCandleFollowService.channels[channel.Key] = openChannel

	go kCandleFollowService.run(channelContext, openChannel)
}

// noLivePlaceUpdates is what a viewer of a market with no place for their symbol
// receives: the news, once, and then nothing until they leave.
//
// The news says which of the two reasons it is, because a viewer arriving out of
// hours is not looking at a system that has run out of places — they are looking at a
// market that is shut, and it will let them in tomorrow without their doing a thing.
//
// The channel stays open rather than closing straight away because a closed channel
// reads as "the feed ended" everywhere else in this feature, and a viewer who never
// had a feed must not be told one ended.
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

// Stop ends every follow and closes every viewer's updates. A viewer arriving after
// this is turned away rather than left waiting on a channel nothing will feed.
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

	// Shutting down is the one ending that really does finish with everybody, so the
	// lines are stopped and then every viewer's updates are closed.
	for _, openChannel := range stoppedChannels {
		openChannel.end()
	}
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
func (kCandleFollowService *KCandleFollowService) leave(
	symbol string, joinedFollow *kCandleFollowSymbol, viewerId int,
) {
	kCandleFollowService.mutex.Lock()

	follow, isFollowing := kCandleFollowService.follows[symbol]
	// Not merely "is anything following this symbol", but "is it still the one this
	// viewer joined". A replacement follow hands out the same ids from zero, so
	// leaving by name alone would close a stream belonging to whoever now holds this
	// id — with no status update and no reason.
	if !isFollowing || follow != joinedFollow {
		kCandleFollowService.mutex.Unlock()

		return
	}

	if !follow.leave(viewerId) {
		kCandleFollowService.mutex.Unlock()

		return
	}
	delete(kCandleFollowService.follows, symbol)

	// A viewer-started follow is alone on its channel, so the last viewer leaving
	// takes the line with it. Looking the channel up by what this symbol travels on
	// rather than remembering it here keeps one answer to "which line is this on".
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

// run keeps one market followed for as long as anyone is watching it. Every time the
// feed ends — refused, dropped, or gone silent — the viewers are told, the wait
// grows, and it tries again. It never gives up; only the last viewer leaving ends it.
func (kCandleFollowService *KCandleFollowService) run(
	executionContext context.Context, openChannel *kCandleFollowChannel,
) {
	defer close(openChannel.finished)

	healthDomain := domains.NewLiveChannelHealthDomain(
		kCandleFollowService.quietTimeout,
		kCandleFollowService.maximumRetryDelay,
		kCandleFollowService.clockProxy.Now(),
	)

	for {
		// Each attempt gets a context of its own so that abandoning it really closes
		// the line behind it. A feed that fell silent is still open — its reader is
		// sitting on a socket nobody is listening to any more — and dialling the next
		// attempt without letting go of it would leave two lines where the plan
		// allows one, which is the very thing being followed a channel at a time was
		// meant to prevent.
		attemptContext, abandonAttempt := context.WithCancel(executionContext)

		liveKCandles, followError := kCandleFollowService.liveMarketDataProxy.
			FollowKCandles(attemptContext, openChannel.channel)
		if followError == nil {
			healthDomain.MarkConnected(kCandleFollowService.clockProxy.Now())
			kCandleFollowService.consume(attemptContext, openChannel, healthDomain, liveKCandles)
		} else {
			log.Printf("live k candle follow: %s could not be followed: %v",
				openChannel.channel.Key, followError)
		}

		abandonAttempt()

		// A channel whose context is already done was ended on purpose — the system is
		// shutting down, or these symbols lost their places. Saying "stalled" then
		// would leave a viewer waiting for a recovery nobody intends, and would
		// overwrite the reason they were just given.
		if executionContext.Err() != nil {
			return
		}

		// One line went down, so every symbol on it hears the same news — nobody is
		// being retired here, the line is simply being tried again.
		openChannel.publishStalled(nil)

		// Said out loud because the gap is the only evidence the growing back-off is
		// working: a source that keeps accepting connections and dropping them is
		// otherwise indistinguishable, in the log, from one being retried every second.
		retryDelay := healthDomain.NextRetryDelay()
		log.Printf("live k candle follow: %s is not delivering; trying again in %s",
			openChannel.channel.Key, retryDelay)

		if !waitOrDone(executionContext, retryDelay) {
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
	openChannel *kCandleFollowChannel,
	healthDomain *domains.LiveChannelHealthDomain,
	liveKCandles <-chan vo.LiveKCandleVo,
) {
	quietCheck := time.NewTicker(healthDomain.QuietCheckInterval())
	defer quietCheck.Stop()

	for {
		select {
		case <-executionContext.Done():
			return

		case liveKCandle, isDelivering := <-liveKCandles:
			if !isDelivering {
				return
			}

			// Anything arriving proves the line is alive, whichever symbol it was
			// about — the silence and the retry gap belong to the line, not to a
			// symbol that happened to trade.
			healthDomain.MarkReceived(kCandleFollowService.clockProxy.Now())

			// The candle names the symbol it belongs to, and that is the only thing
			// that decides whose it is. Two symbols sharing a line are two pictures.
			follow, isCarried := openChannel.followOf(liveKCandle.Symbol)
			if !isCarried {
				// Said out loud because from every other angle this looks healthy:
				// the line is alive, candles are arriving, and not one of them ever
				// reaches a viewer. Silence here would make a name that does not
				// match — a case difference, a suffix — indistinguishable from a
				// market that simply has nothing to report.
				log.Printf("live k candle follow: %s carries no %s, dropping its candle",
					openChannel.channel.Key, liveKCandle.Symbol)

				continue
			}
			kCandleFollowService.report(executionContext, follow, liveKCandle)

		case <-quietCheck.C:
			if healthDomain.HasGoneQuiet(kCandleFollowService.clockProxy.Now()) {
				return
			}
		}
	}
}

// report decides what one reported candle amounts to: whether it is worth passing
// on, and whether it is that candle's last word and therefore worth storing.
func (kCandleFollowService *KCandleFollowService) report(
	executionContext context.Context,
	follow *kCandleFollowSymbol,
	liveKCandle vo.LiveKCandleVo,
) {
	now := kCandleFollowService.clockProxy.Now()
	if !follow.throttle.Admit(liveKCandle, now) {
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
// refuses, is one the scheduled round will deal with, and neither reason is worth
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
