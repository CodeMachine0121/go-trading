package service

import (
	"sync"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// viewerBufferSize is how far a viewer may fall behind before updates are dropped, so a slow viewer cannot stall the others; only forming candles are lost, as closed ones are already stored.
const viewerBufferSize = 8

// kCandleFollowSymbol is one followed market and its viewers, with its own lock so publishing to one market never blocks opening another; latestUpdate lets late arrivals see the current candle at once.
type kCandleFollowSymbol struct {
	symbol string
	market vo.MarketVo
	// isOnARoster marks a follow kept for a roster place rather than a viewer, so it outlives its last viewer.
	isOnARoster bool
	// throttle is per symbol, not per channel, so a busy neighbour on the same line cannot slow this chart.
	throttle *domains.ViewerUpdateThrottleDomain

	mutex        sync.Mutex
	viewers      map[int]chan dto.KCandleFollowUpdateDto
	nextViewerId int
	latestUpdate dto.KCandleFollowUpdateDto
	hasLatest    bool
	isStalled    bool
}

func newKCandleFollowSymbol(
	symbol string,
	market vo.MarketVo,
	isOnARoster bool,
	throttle *domains.ViewerUpdateThrottleDomain,
) *kCandleFollowSymbol {
	return &kCandleFollowSymbol{
		symbol:      symbol,
		market:      market,
		isOnARoster: isOnARoster,
		throttle:    throttle,
		viewers:     make(map[int]chan dto.KCandleFollowUpdateDto),
	}
}

// join adds a viewer and, under the same lock, catches them up so no update can be missed in between.
func (followSymbol *kCandleFollowSymbol) join() (int, chan dto.KCandleFollowUpdateDto) {
	followSymbol.mutex.Lock()
	defer followSymbol.mutex.Unlock()

	viewerId := followSymbol.nextViewerId
	followSymbol.nextViewerId++
	updates := make(chan dto.KCandleFollowUpdateDto, viewerBufferSize)
	followSymbol.viewers[viewerId] = updates

	if followSymbol.hasLatest {
		updates <- followSymbol.latestUpdate
	}

	// Arriving during an outage must not look like a quiet market showing the last candle.
	if followSymbol.isStalled {
		updates <- followSymbol.stalledUpdate()
	}

	return viewerId, updates
}

// leave removes a viewer and reports whether that ends the follow; rostered follows never end this way.
func (followSymbol *kCandleFollowSymbol) leave(viewerId int) bool {
	followSymbol.mutex.Lock()
	defer followSymbol.mutex.Unlock()

	updates, isViewing := followSymbol.viewers[viewerId]
	if !isViewing {
		return false
	}
	delete(followSymbol.viewers, viewerId)
	close(updates)

	return len(followSymbol.viewers) == 0 && !followSymbol.isOnARoster
}

// publish sends the update to every viewer, dropping it for ones whose buffer is full, and remembers candle-carrying updates for the next arrival.
func (followSymbol *kCandleFollowSymbol) publish(update dto.KCandleFollowUpdateDto) {
	followSymbol.mutex.Lock()
	defer followSymbol.mutex.Unlock()

	// Stalled carries no candle, so it is remembered as a state rather than as the latest shape.
	followSymbol.isStalled = update.Status == dto.KCandleFollowStatusStalled
	// Listing the states that do carry a candle makes any future state default to carrying none.
	carriesACandle := update.Status == dto.KCandleFollowStatusForming ||
		update.Status == dto.KCandleFollowStatusClosed
	if carriesACandle {
		followSymbol.latestUpdate = update
		followSymbol.hasLatest = true
	}

	for _, updates := range followSymbol.viewers {
		select {
		case updates <- update:
		default:
		}
	}
}

// pass publishes the candle if the throttle admits it (closed candles always are) and reports whether it did.
func (followSymbol *kCandleFollowSymbol) pass(liveKCandle vo.LiveKCandleVo, now time.Time) bool {
	if !followSymbol.throttle.Admit(liveKCandle, now) {
		return false
	}

	status := dto.KCandleFollowStatusForming
	if liveKCandle.Closed {
		status = dto.KCandleFollowStatusClosed
	}

	followSymbol.publish(dto.KCandleFollowUpdateDto{
		Symbol:  liveKCandle.Symbol,
		Status:  status,
		KCandle: liveKCandle.ToDto(),
	})

	return true
}

// publishStalled tells viewers live updating stopped, whatever the cause.
func (followSymbol *kCandleFollowSymbol) publishStalled() {
	followSymbol.publish(followSymbol.stalledUpdate())
}

func (followSymbol *kCandleFollowSymbol) stalledUpdate() dto.KCandleFollowUpdateDto {
	return dto.KCandleFollowUpdateDto{
		Symbol: followSymbol.symbol,
		Status: dto.KCandleFollowStatusStalled,
	}
}

// publishUnavailable tells viewers the symbol's roster place went to another symbol, before the follow ends and closes their channels without a reason.
func (followSymbol *kCandleFollowSymbol) publishUnavailable() {
	followSymbol.publish(dto.KCandleFollowUpdateDto{
		Symbol: followSymbol.symbol,
		Status: dto.KCandleFollowStatusUnavailable,
	})
}

// publishMarketClosed tells viewers the market has shut, so the silence that follows is not mistaken for a fault.
func (followSymbol *kCandleFollowSymbol) publishMarketClosed() {
	followSymbol.publish(dto.KCandleFollowUpdateDto{
		Symbol: followSymbol.symbol,
		Status: dto.KCandleFollowStatusMarketClosed,
	})
}

// end closes every viewer's updates; the channel has already stopped publishing into them.
func (followSymbol *kCandleFollowSymbol) end() {
	followSymbol.mutex.Lock()
	defer followSymbol.mutex.Unlock()

	for viewerId, updates := range followSymbol.viewers {
		delete(followSymbol.viewers, viewerId)
		close(updates)
	}
}
