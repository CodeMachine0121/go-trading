package application

import "sync"

// StrategyBotRoundGuard keeps one bot from running two rounds at once.
//
// It is not a domain model and it holds no business rule. Strip away the set and
// the lock and nothing about strategy bots is left — it is a piece of running
// machinery, so it lives beside the thing that owns the rounds rather than in with
// the models.
//
// It keeps its set in memory on purpose. A claim written to the store would survive
// a crash, and a bot whose claim outlived the round that took it would never be
// picked up again — quietly, and for good. One that lives in the process clears
// itself the moment the process comes back, which is the failure worth having.
//
// The cost of that choice is an assumption: one process. A second instance would
// keep its own set and the two would run the same bot in the same minute. The day
// that is on the table, this becomes a stored claim with an expiry — and nothing
// else here changes.
type StrategyBotRoundGuard struct {
	mutex      sync.Mutex
	inProgress map[uint]struct{}
}

func NewStrategyBotRoundGuard() *StrategyBotRoundGuard {
	return &StrategyBotRoundGuard{inProgress: map[uint]struct{}{}}
}

// TryEnter claims a bot for one round, and says whether the claim was free to take.
// A bot already mid-round is left alone rather than queued: the round waiting behind
// it would read the same candles and reach the same answer.
func (strategyBotRoundGuard *StrategyBotRoundGuard) TryEnter(strategyBotID uint) bool {
	strategyBotRoundGuard.mutex.Lock()
	defer strategyBotRoundGuard.mutex.Unlock()

	if _, alreadyRunning := strategyBotRoundGuard.inProgress[strategyBotID]; alreadyRunning {
		return false
	}

	strategyBotRoundGuard.inProgress[strategyBotID] = struct{}{}

	return true
}

// Leave releases a claim. It is always paired with a successful TryEnter through a
// defer, so a round that panics still gives the bot back.
func (strategyBotRoundGuard *StrategyBotRoundGuard) Leave(strategyBotID uint) {
	strategyBotRoundGuard.mutex.Lock()
	defer strategyBotRoundGuard.mutex.Unlock()

	delete(strategyBotRoundGuard.inProgress, strategyBotID)
}
