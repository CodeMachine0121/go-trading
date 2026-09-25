package script

import (
	"context"
	"fmt"
	"slices"
	"sync"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
)

// IndicatorScriptCompartmentSlots caps how many compartments run at once across the whole service; strategy bot rounds wait ahead of on-demand calculations so a flood of requests cannot starve them.
type IndicatorScriptCompartmentSlots struct {
	mutex sync.Mutex
	// freeCount is only ever above zero while nobody waits, since a freed slot goes to a waiter first.
	freeCount                  int
	strategyBotRoundWaiters    []chan struct{}
	onDemandCalculationWaiters []chan struct{}
}

func NewIndicatorScriptCompartmentSlots(capacity int) *IndicatorScriptCompartmentSlots {
	return &IndicatorScriptCompartmentSlots{freeCount: capacity}
}

// take waits for a slot no longer than the caller's own context allows.
func (slots *IndicatorScriptCompartmentSlots) take(
	executionContext context.Context, servesStrategyBotRounds bool,
) error {
	slots.mutex.Lock()
	if slots.freeCount > 0 {
		slots.freeCount--
		slots.mutex.Unlock()
		return nil
	}

	granted := make(chan struct{})
	waiters := &slots.onDemandCalculationWaiters
	if servesStrategyBotRounds {
		waiters = &slots.strategyBotRoundWaiters
	}
	*waiters = append(*waiters, granted)
	slots.mutex.Unlock()

	select {
	case <-granted:
		return nil
	case <-executionContext.Done():
	}

	slots.mutex.Lock()
	waitingIndex := slices.Index(*waiters, granted)
	if waitingIndex >= 0 {
		*waiters = slices.Delete(*waiters, waitingIndex, waitingIndex+1)
	}
	slots.mutex.Unlock()
	// Granted in the same instant the caller left, so the slot is passed on rather than lost.
	if waitingIndex < 0 {
		slots.release()
	}

	return fmt.Errorf("%w: 算式隔間目前全數忙碌中，請稍後再試", domains.ErrIndicatorScriptCompartmentsBusy)
}

func (slots *IndicatorScriptCompartmentSlots) release() {
	slots.mutex.Lock()
	defer slots.mutex.Unlock()

	for _, waiters := range []*[]chan struct{}{&slots.strategyBotRoundWaiters, &slots.onDemandCalculationWaiters} {
		if len(*waiters) > 0 {
			close((*waiters)[0])
			*waiters = (*waiters)[1:]
			return
		}
	}

	slots.freeCount++
}
