package service

import (
	"sync"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// viewerBufferSize is how many updates a viewer may fall behind by before one is
// dropped. Dropping is deliberate: a viewer that cannot keep up must not be able to
// stall the follow for everybody else, and an update it missed is superseded by the
// next one anyway. Only a forming candle is ever dropped this way — a closed one was
// already stored before it was ever sent.
const viewerBufferSize = 8

// kCandleFollowSymbol is one market being followed and everyone currently looking
// at it.
//
// It owns its viewers rather than letting the service reach in, which is what keeps
// the two locks apart: sending to a busy market's viewers must not hold up somebody
// opening a chart of a different one.
//
// latestUpdate is kept so that somebody arriving mid-candle sees the shape now
// rather than an empty chart until the market next moves.
type kCandleFollowSymbol struct {
	symbol string
	// market is which venue this symbol trades on, carried so that opening its feed
	// needs nothing looked up again — the follow already knows.
	market vo.MarketVo
	// isOnARoster marks a follow the system keeps up because a market's places were
	// handed to this symbol, not because anybody is looking at it. Such a follow
	// outlives its last viewer: the places are decided by the watchlist, and dropping
	// one the moment nobody happened to be watching would leave it unfilled until
	// somebody was.
	isOnARoster bool
	// throttle is how often this symbol's picture may be redrawn. It is per symbol
	// and not per channel: two symbols sharing a line are two pictures, and holding
	// one back because the other just moved would make a busy neighbour into a slow
	// chart.
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

// join adds one viewer and hands back how to reach them, along with what the market
// looks like right now.
//
// Catching them up happens here rather than at the caller, because the state it
// reads and the viewer it writes to both belong to this object — and because there
// must be no gap between the two in which an update could be published and missed.
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

	// Arriving during an outage must not look like arriving during a quiet market.
	// The last candle alone would look live.
	if followSymbol.isStalled {
		updates <- followSymbol.stalledUpdate()
	}

	return viewerId, updates
}

// leave removes one viewer, reporting whether that ends the follow — which is the
// one thing the caller needs to know.
//
// A follow held up by a market's roster is never ended by a viewer leaving. It was
// not started by one either, so nobody watching is simply nobody watching.
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

// publish hands one update to everyone watching, and remembers it for whoever
// arrives next.
//
// A viewer too far behind to take it loses this one rather than holding up the
// rest — see viewerBufferSize.
func (followSymbol *kCandleFollowSymbol) publish(update dto.KCandleFollowUpdateDto) {
	followSymbol.mutex.Lock()
	defer followSymbol.mutex.Unlock()

	// Stalled carries no candle, so it must not become the shape handed to whoever
	// arrives next — they would be drawn a candle of zeros. It is remembered as a
	// state instead, and told to them separately. Unavailable carries no candle
	// either, and it is the last thing a follow ever says, so there is no next
	// arrival for it to mislead.
	followSymbol.isStalled = update.Status == dto.KCandleFollowStatusStalled
	// Named the other way round — which states do carry one — so that a state added
	// later is silently treated as carrying no candle rather than silently treated as
	// carrying one it does not have.
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

// publishStalled tells every viewer that live updating has stopped. Whether the
// source refused, dropped, or fell silent, from the viewer's side it is one piece of
// news, so there is one way to say it.
func (followSymbol *kCandleFollowSymbol) publishStalled() {
	followSymbol.publish(followSymbol.stalledUpdate())
}

func (followSymbol *kCandleFollowSymbol) stalledUpdate() dto.KCandleFollowUpdateDto {
	return dto.KCandleFollowUpdateDto{
		Symbol: followSymbol.symbol,
		Status: dto.KCandleFollowStatusStalled,
	}
}

// publishUnavailable tells every viewer that this symbol has no live updating left
// to give: its place on the roster went to another symbol.
//
// It is said before the follow ends rather than left to the closing of their
// channels, because a channel that simply stops carries no reason, and the reason is
// the whole difference between waiting and not bothering to.
func (followSymbol *kCandleFollowSymbol) publishUnavailable() {
	followSymbol.publish(dto.KCandleFollowUpdateDto{
		Symbol: followSymbol.symbol,
		Status: dto.KCandleFollowStatusUnavailable,
	})
}

// publishMarketClosed tells every viewer that the market itself has shut, which is
// why nothing more is coming.
//
// The same silence follows as for unavailable, and that is exactly why it must be
// said differently: silence explained as "this will not come back" leaves somebody
// looking for a fault, when all that happened is that the day ended.
func (followSymbol *kCandleFollowSymbol) publishMarketClosed() {
	followSymbol.publish(dto.KCandleFollowUpdateDto{
		Symbol: followSymbol.symbol,
		Status: dto.KCandleFollowStatusMarketClosed,
	})
}

// end closes every viewer's updates. Stopping the work that publishes into them is
// the channel's job and has already happened by the time this is called — a symbol
// does not own the line it travels on.
func (followSymbol *kCandleFollowSymbol) end() {
	followSymbol.mutex.Lock()
	defer followSymbol.mutex.Unlock()

	for viewerId, updates := range followSymbol.viewers {
		delete(followSymbol.viewers, viewerId)
		close(updates)
	}
}
