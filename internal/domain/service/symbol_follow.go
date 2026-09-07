package service

import (
	"context"
	"sync"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// viewerBufferSize is how many updates a viewer may fall behind by before one is
// dropped. Dropping is deliberate: a viewer that cannot keep up must not be able to
// stall the follow for everybody else, and an update it missed is superseded by the
// next one anyway. Only a forming candle is ever dropped this way — a closed one was
// already stored before it was ever sent.
const viewerBufferSize = 8

// symbolFollow is one market being followed and everyone currently looking at it.
//
// It owns its viewers rather than letting the service reach in, which is what keeps
// the two locks apart: sending to a busy market's viewers must not hold up somebody
// opening a chart of a different one.
//
// latestUpdate is kept so that somebody arriving mid-candle sees the shape now
// rather than an empty chart until the market next moves.
type symbolFollow struct {
	symbol string
	// market is which venue this symbol trades on, carried so that opening its feed
	// needs nothing looked up again — the follow already knows.
	market vo.MarketVo
	cancel context.CancelFunc
	// isOnARoster marks a follow the system keeps up because a market's places were
	// handed to this symbol, not because anybody is looking at it. Such a follow
	// outlives its last viewer: the places are decided by the watchlist, and dropping
	// one the moment nobody happened to be watching would leave it unfilled until
	// somebody was.
	isOnARoster bool
	finished    chan struct{}

	mutex        sync.Mutex
	viewers      map[int]chan dto.KCandleFollowUpdateDto
	nextViewerId int
	latestUpdate dto.KCandleFollowUpdateDto
	hasLatest    bool
	isStalled    bool
}

func newSymbolFollow(
	symbol string, market vo.MarketVo, isOnARoster bool, cancel context.CancelFunc,
) *symbolFollow {
	return &symbolFollow{
		symbol:      symbol,
		market:      market,
		isOnARoster: isOnARoster,
		cancel:      cancel,
		finished:    make(chan struct{}),
		viewers:     make(map[int]chan dto.KCandleFollowUpdateDto),
	}
}

// join adds one viewer and hands back how to reach them, along with what the market
// looks like right now.
//
// Catching them up happens here rather than at the caller, because the state it
// reads and the viewer it writes to both belong to this object — and because there
// must be no gap between the two in which an update could be published and missed.
func (symbolFollow *symbolFollow) join() (int, chan dto.KCandleFollowUpdateDto) {
	symbolFollow.mutex.Lock()
	defer symbolFollow.mutex.Unlock()

	viewerId := symbolFollow.nextViewerId
	symbolFollow.nextViewerId++
	updates := make(chan dto.KCandleFollowUpdateDto, viewerBufferSize)
	symbolFollow.viewers[viewerId] = updates

	if symbolFollow.hasLatest {
		updates <- symbolFollow.latestUpdate
	}

	// Arriving during an outage must not look like arriving during a quiet market.
	// The last candle alone would look live.
	if symbolFollow.isStalled {
		updates <- symbolFollow.stalledUpdate()
	}

	return viewerId, updates
}

// leave removes one viewer, reporting whether that ends the follow — which is the
// one thing the caller needs to know.
//
// A follow held up by a market's roster is never ended by a viewer leaving. It was
// not started by one either, so nobody watching is simply nobody watching.
func (symbolFollow *symbolFollow) leave(viewerId int) bool {
	symbolFollow.mutex.Lock()
	defer symbolFollow.mutex.Unlock()

	updates, isViewing := symbolFollow.viewers[viewerId]
	if !isViewing {
		return false
	}
	delete(symbolFollow.viewers, viewerId)
	close(updates)

	return len(symbolFollow.viewers) == 0 && !symbolFollow.isOnARoster
}

// publish hands one update to everyone watching, and remembers it for whoever
// arrives next.
//
// A viewer too far behind to take it loses this one rather than holding up the
// rest — see viewerBufferSize.
func (symbolFollow *symbolFollow) publish(update dto.KCandleFollowUpdateDto) {
	symbolFollow.mutex.Lock()
	defer symbolFollow.mutex.Unlock()

	// Stalled carries no candle, so it must not become the shape handed to whoever
	// arrives next — they would be drawn a candle of zeros. It is remembered as a
	// state instead, and told to them separately. Unavailable carries no candle
	// either, and it is the last thing a follow ever says, so there is no next
	// arrival for it to mislead.
	symbolFollow.isStalled = update.Status == dto.KCandleFollowStatusStalled
	carriesACandle := update.Status != dto.KCandleFollowStatusStalled &&
		update.Status != dto.KCandleFollowStatusUnavailable
	if carriesACandle {
		symbolFollow.latestUpdate = update
		symbolFollow.hasLatest = true
	}

	for _, updates := range symbolFollow.viewers {
		select {
		case updates <- update:
		default:
		}
	}
}

// publishStalled tells every viewer that live updating has stopped. Whether the
// source refused, dropped, or fell silent, from the viewer's side it is one piece of
// news, so there is one way to say it.
func (symbolFollow *symbolFollow) publishStalled() {
	symbolFollow.publish(symbolFollow.stalledUpdate())
}

func (symbolFollow *symbolFollow) stalledUpdate() dto.KCandleFollowUpdateDto {
	return dto.KCandleFollowUpdateDto{
		Symbol: symbolFollow.symbol,
		Status: dto.KCandleFollowStatusStalled,
	}
}

// publishUnavailable tells every viewer that this market has no live updating left to
// give — its place on the roster went to another symbol, or its market shut.
//
// It is said before the follow ends rather than left to the closing of their
// channels, because a channel that simply stops carries no reason, and the reason is
// the whole difference between waiting and not bothering to.
func (symbolFollow *symbolFollow) publishUnavailable() {
	symbolFollow.publish(dto.KCandleFollowUpdateDto{
		Symbol: symbolFollow.symbol,
		Status: dto.KCandleFollowStatusUnavailable,
	})
}

// end stops this follow and closes every viewer's updates, waiting for the work to
// finish first so that nothing is still publishing into a channel about to close.
func (symbolFollow *symbolFollow) end() {
	symbolFollow.cancel()
	<-symbolFollow.finished

	symbolFollow.mutex.Lock()
	defer symbolFollow.mutex.Unlock()

	for viewerId, updates := range symbolFollow.viewers {
		delete(symbolFollow.viewers, viewerId)
		close(updates)
	}
}
