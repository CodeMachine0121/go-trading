package application

import "sync"

// StrategyBotRoundGuard keeps one bot from running two rounds at once.
// It is in-memory so a crash can't leave a stale claim, which assumes a single process; multiple instances would need a stored claim with expiry.
type StrategyBotRoundGuard struct {
	mutex      sync.Mutex
	inProgress map[uint]struct{}
}

func NewStrategyBotRoundGuard() *StrategyBotRoundGuard {
	return &StrategyBotRoundGuard{inProgress: map[uint]struct{}{}}
}

// TryEnter claims a bot for one round; a bot already mid-round is skipped rather than queued.
func (strategyBotRoundGuard *StrategyBotRoundGuard) TryEnter(strategyBotID uint) bool {
	strategyBotRoundGuard.mutex.Lock()
	defer strategyBotRoundGuard.mutex.Unlock()

	if _, alreadyRunning := strategyBotRoundGuard.inProgress[strategyBotID]; alreadyRunning {
		return false
	}

	strategyBotRoundGuard.inProgress[strategyBotID] = struct{}{}

	return true
}

// Leave releases a claim; pair it with TryEnter via defer so a panicking round still releases it.
func (strategyBotRoundGuard *StrategyBotRoundGuard) Leave(strategyBotID uint) {
	strategyBotRoundGuard.mutex.Lock()
	defer strategyBotRoundGuard.mutex.Unlock()

	delete(strategyBotRoundGuard.inProgress, strategyBotID)
}
