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

// KCandleContractFollowService keeps one live follow per perpetual contract, apart from the spot registry because spot and perpetual BTCUSDT are different markets.
// It has no roster (contracts never close and have no follow ceiling) and never stores closed candles, since only the scheduled round reads all four readings.
type KCandleContractFollowService struct {
	contractTradingSymbolRepository _interface.IContractTradingSymbolRepository
	clockProxy                      _interface.IClockProxy
	updateIntervalCeiling           time.Duration
	feed                            *kCandleFollowFeed

	mutex   sync.Mutex
	follows map[string]*kCandleFollowSymbol
	// channels holds one line per follow, keyed like the spot registry so a channel value always names the same line.
	channels map[string]*kCandleFollowChannel
	stopped  bool
}

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

// WatchKCandleContracts joins the viewer to the contract's live follow; only watchlisted contracts are served because closed candles are stored only by the watchlist round.
// Watch status is checked once on arrival, and the returned channel closes when the viewer's context ends or the service stops.
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
		return nil, fmt.Errorf("%w: 找不到這個合約標的 %s", domains.ErrTradingSymbolNotRegistered, contractSymbol.Value())
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
		// A zero last-sent time lets the first forming candle go out immediately instead of after a whole ceiling.
		follow = newKCandleFollowSymbol(contractSymbol.Value(), vo.MarketCrypto, false,
			domains.NewViewerUpdateThrottleDomain(
				kCandleContractFollowService.updateIntervalCeiling, time.Time{}))
		kCandleContractFollowService.follows[contractSymbol.Value()] = follow

		// The line outlives the requesting viewer, so it must not inherit their cancellable context.
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

// Stop ends every follow and closes all viewers' updates; viewers arriving afterwards are turned away.
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

func (kCandleContractFollowService *KCandleContractFollowService) FollowedSymbolCount() int {
	kCandleContractFollowService.mutex.Lock()
	defer kCandleContractFollowService.mutex.Unlock()

	return len(kCandleContractFollowService.follows)
}

// leave removes a viewer and, if they were the last, removes the follow and returns its line for the caller to cancel outside the lock.
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

// report throttles each candle; closed ones are not stored here because the scheduled round stores them with all four readings.
func (kCandleContractFollowService *KCandleContractFollowService) report(
	_ context.Context, follow *kCandleFollowSymbol, liveKCandle vo.LiveKCandleVo,
) {
	follow.pass(liveKCandle, kCandleContractFollowService.clockProxy.Now())
}
