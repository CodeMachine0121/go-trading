package service

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	_interface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// KCandleContractFollowService owns which perpetual contracts are being followed live:
// one follow per contract, started by the first viewer and ended by the last.
//
// It is the contract line's own registry, kept apart from the spot one for the reason
// the two lines are kept apart everywhere: BTCUSDT on spot and BTCUSDT as a perpetual
// are two markets, and a viewer of one must never be handed the other's price. Nothing
// here is shared with the spot follow but the mechanism — the viewers of one contract
// (kCandleFollowSymbol), the line they travel on (kCandleFollowChannel) and the keeping
// of that line against the source (kCandleFollowFeed).
//
// It is simpler than its spot twin in two ways, both deliberate. Contracts never close
// and the venue sets no ceiling on how many may be followed, so there is no roster.
// And a candle that closes is passed on but never stored: a contract K candle is four
// readings lined up, and the live line carries only the first, so the scheduled round —
// which reads all four — is what stores it.
type KCandleContractFollowService struct {
	contractTradingSymbolRepository _interface.IContractTradingSymbolRepository
	clockProxy                      _interface.IClockProxy
	updateIntervalCeiling           time.Duration
	feed                            *kCandleFollowFeed

	mutex sync.Mutex
	// follows is every contract somebody is watching, keyed by contract.
	follows map[string]*kCandleFollowSymbol
	// channels is every line currently open. A contract is followed one to a line, so
	// there is one per follow; it is keyed the way the spot registry keys its lines so
	// that the same channel value always names the same line.
	channels map[string]*kCandleFollowChannel
	stopped  bool
}

// NewKCandleContractFollowService takes the contract venue's live source and the three
// timing rules once; every contract it follows is judged by them.
func NewKCandleContractFollowService(
	liveMarketDataProxy _interface.ILiveMarketDataProxy,
	contractTradingSymbolRepository _interface.IContractTradingSymbolRepository,
	clockProxy _interface.IClockProxy,
	updateIntervalCeiling time.Duration,
	quietTimeout time.Duration,
	maximumRetryDelay time.Duration,
) *KCandleContractFollowService {
	kCandleContractFollowService := &KCandleContractFollowService{
		contractTradingSymbolRepository: contractTradingSymbolRepository,
		clockProxy:                      clockProxy,
		updateIntervalCeiling:           updateIntervalCeiling,
		follows:                         make(map[string]*kCandleFollowSymbol),
		channels:                        make(map[string]*kCandleFollowChannel),
	}
	kCandleContractFollowService.feed = newKCandleFollowFeed(
		liveMarketDataProxy, clockProxy, quietTimeout, maximumRetryDelay,
		kCandleContractFollowService.report)

	return kCandleContractFollowService
}

// WatchKCandleContracts joins this viewer to the live follow of one contract, starting
// that follow if nobody was watching it yet, and hands back the updates they will
// receive. Leaving is the viewer's own context ending.
//
// Only a contract on the contract watchlist is served. A contract the system has never
// heard of is not found; one it knows but does not follow is refused, because what
// closes on a live line is stored only by the round that follows the watchlist — a
// contract off it would show a candle closing and then leave nothing behind.
//
// Whether it is followed is asked when the viewer arrives and not again. A contract
// taken off the watchlist while somebody is looking keeps their picture until they
// leave, the same way the watchlist never reaches back into a follow already running.
//
// The returned channel is closed when the viewer leaves or the service stops.
func (kCandleContractFollowService *KCandleContractFollowService) WatchKCandleContracts(
	executionContext context.Context, symbol string,
) (<-chan dto.KCandleFollowUpdateDto, error) {
	contractSymbol, symbolError := domains.NewTradingSymbolDomain(strings.TrimSpace(symbol))
	if symbolError != nil {
		return nil, fmt.Errorf("%w: %w", domains.ErrKCandleContractValidation, symbolError)
	}

	contractTradingSymbol, isRegistered, findError := kCandleContractFollowService.
		contractTradingSymbolRepository.FindBySymbol(executionContext, contractSymbol.Value())
	if findError != nil {
		return nil, findError
	}

	if !isRegistered {
		return nil, fmt.Errorf("%w: %s", domains.ErrTradingSymbolNotRegistered, contractSymbol.Value())
	}

	if !contractTradingSymbol.IsWatched {
		return nil, domains.ContractTradingSymbolNotWatched(contractSymbol.Value())
	}

	kCandleContractFollowService.mutex.Lock()

	if kCandleContractFollowService.stopped {
		kCandleContractFollowService.mutex.Unlock()

		return nil, ErrKCandleFollowStopped
	}

	follow, isFollowing := kCandleContractFollowService.follows[contractSymbol.Value()]
	if !isFollowing {
		follow = newKCandleFollowSymbol(contractSymbol.Value(), vo.MarketCrypto, false,
			domains.NewViewerUpdateThrottleDomain(
				kCandleContractFollowService.updateIntervalCeiling,
				kCandleContractFollowService.clockProxy.Now()))
		kCandleContractFollowService.follows[contractSymbol.Value()] = follow

		// The line outlives the viewer who asked for it, so it must not inherit their
		// context — the next viewer would be following a contract on a cancelled one.
		channel := vo.NewLiveFollowChannelVo(vo.MarketCrypto, []string{contractSymbol.Value()})
		channelContext, cancel := context.WithCancel(context.WithoutCancel(executionContext))
		openChannel := newKCandleFollowChannel(
			channel, map[string]*kCandleFollowSymbol{contractSymbol.Value(): follow}, cancel)
		kCandleContractFollowService.channels[channel.Key] = openChannel

		go kCandleContractFollowService.feed.keep(channelContext, openChannel)
	}

	viewerId, updates := follow.join()
	kCandleContractFollowService.mutex.Unlock()

	go func() {
		<-executionContext.Done()

		if departingChannel, isOpen := kCandleContractFollowService.leave(
			contractSymbol.Value(), viewerId); isOpen {
			departingChannel.cancel()
		}
	}()

	return updates, nil
}

// Stop ends every contract follow and closes every viewer's updates. A viewer arriving
// after this is turned away rather than left waiting on a channel nothing will feed.
func (kCandleContractFollowService *KCandleContractFollowService) Stop() {
	kCandleContractFollowService.mutex.Lock()
	if kCandleContractFollowService.stopped {
		kCandleContractFollowService.mutex.Unlock()

		return
	}
	kCandleContractFollowService.stopped = true

	stoppedChannels := make([]*kCandleFollowChannel, 0, len(kCandleContractFollowService.channels))
	for key, openChannel := range kCandleContractFollowService.channels {
		stoppedChannels = append(stoppedChannels, openChannel)
		delete(kCandleContractFollowService.channels, key)
	}
	stoppedFollows := make([]*kCandleFollowSymbol, 0, len(kCandleContractFollowService.follows))
	for _, follow := range kCandleContractFollowService.follows {
		stoppedFollows = append(stoppedFollows, follow)
	}
	clear(kCandleContractFollowService.follows)
	kCandleContractFollowService.mutex.Unlock()

	for _, openChannel := range stoppedChannels {
		openChannel.end()
	}
	for _, follow := range stoppedFollows {
		follow.end()
	}
}

// FollowedSymbolCount reports how many contracts are being followed right now — the
// only observable trace of "one follow per contract, ending with the last viewer".
func (kCandleContractFollowService *KCandleContractFollowService) FollowedSymbolCount() int {
	kCandleContractFollowService.mutex.Lock()
	defer kCandleContractFollowService.mutex.Unlock()

	return len(kCandleContractFollowService.follows)
}

// leave removes one viewer and, when they were the last, takes the contract's follow
// and its line out of the registry, handing the line back to be cancelled.
//
// It is a method of its own because it draws the stretch the lock is held for: the
// line is cancelled by the caller after the lock is let go, so that a line winding
// down never holds up somebody opening another contract's chart.
//
// A contract's follow leaves the registry only when its last viewer does, or when the
// service stops and empties it, so the follow found here is always the one the viewer
// joined — unlike the spot registry, whose roster can replace a follow under a viewer
// still attached to it.
func (kCandleContractFollowService *KCandleContractFollowService) leave(
	symbol string, viewerId int,
) (*kCandleFollowChannel, bool) {
	kCandleContractFollowService.mutex.Lock()
	defer kCandleContractFollowService.mutex.Unlock()

	follow, isFollowing := kCandleContractFollowService.follows[symbol]
	if !isFollowing || !follow.leave(viewerId) {
		return nil, false
	}
	delete(kCandleContractFollowService.follows, symbol)

	channelKey := vo.NewLiveFollowChannelVo(follow.market, []string{symbol}).Key
	departingChannel, isOpen := kCandleContractFollowService.channels[channelKey]
	if isOpen {
		delete(kCandleContractFollowService.channels, channelKey)
	}

	return departingChannel, isOpen
}

// report decides what one reported contract candle amounts to: whether it is worth
// passing on now. A closed one always is — it is that candle's last word — and it is
// never stored here; the scheduled round stores it with all four readings.
func (kCandleContractFollowService *KCandleContractFollowService) report(
	_ context.Context, follow *kCandleFollowSymbol, liveKCandle vo.LiveKCandleVo,
) {
	follow.pass(liveKCandle, kCandleContractFollowService.clockProxy.Now())
}
