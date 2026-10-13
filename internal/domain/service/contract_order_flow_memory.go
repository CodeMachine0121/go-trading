package service

import (
	"sync"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
)

// ContractOrderFlowMemory keeps recently gathered order flow days in this process only, least recently used dropped first once the budget is spent; it also lets one gatherer at a time work on a given day so the same archive is never downloaded twice at once.
// It holds no business rule: what a day holds and how it grows is ContractOrderFlowDayDomain's.
type ContractOrderFlowMemory struct {
	lock        sync.Mutex
	budgetBytes int64
	usedBytes   int64
	// useCount stamps each recall and keep, so the entry with the lowest stamp is the least recently used.
	useCount uint64
	entries  map[contractOrderFlowMemoryKey]*contractOrderFlowMemoryEntry
	// gatherers counts who holds or waits for each day's gathering lock, so idle locks are dropped.
	gatherers map[contractOrderFlowMemoryKey]*contractOrderFlowGatherer
}

// contractOrderFlowMemoryKey is the unit remembered: one symbol's day at one price step.
type contractOrderFlowMemoryKey struct {
	symbol    string
	dayStart  int64
	priceStep string
}

type contractOrderFlowMemoryEntry struct {
	day      domains.ContractOrderFlowDayDomain
	bytes    int64
	lastUsed uint64
}

type contractOrderFlowGatherer struct {
	lock    sync.Mutex
	holders int
}

func NewContractOrderFlowMemory(budgetBytes int64) *ContractOrderFlowMemory {
	return &ContractOrderFlowMemory{
		budgetBytes: budgetBytes,
		entries:     map[contractOrderFlowMemoryKey]*contractOrderFlowMemoryEntry{},
		gatherers:   map[contractOrderFlowMemoryKey]*contractOrderFlowGatherer{},
	}
}

// Recall hands back what is remembered of that day, marking it recently used.
func (memory *ContractOrderFlowMemory) Recall(
	symbol string, day time.Time, priceStep domains.OrderFlowPriceStepDomain,
) (domains.ContractOrderFlowDayDomain, bool) {
	memory.lock.Lock()
	defer memory.lock.Unlock()

	entry, isRemembered := memory.entries[contractOrderFlowMemoryKey{
		symbol: symbol, dayStart: day.UTC().Unix(), priceStep: priceStep.Key()}]
	if !isRemembered {
		return domains.ContractOrderFlowDayDomain{}, false
	}
	memory.useCount++
	entry.lastUsed = memory.useCount

	return entry.day, true
}

// Keep remembers the day in place of any earlier version of it, dropping the least recently used days until it fits; a day larger than the whole budget is not kept.
func (memory *ContractOrderFlowMemory) Keep(symbol string, day domains.ContractOrderFlowDayDomain) {
	key := contractOrderFlowMemoryKey{
		symbol: symbol, dayStart: day.DayStart().Unix(), priceStep: day.PriceStep().Key()}
	dayBytes := day.EstimatedBytes()

	memory.lock.Lock()
	defer memory.lock.Unlock()

	if entry, isRemembered := memory.entries[key]; isRemembered {
		memory.usedBytes -= entry.bytes
		delete(memory.entries, key)
	}
	if dayBytes > memory.budgetBytes {
		return
	}

	// Only a few hundred days are ever remembered, so finding the least recently used by a scan is cheap.
	for memory.usedBytes+dayBytes > memory.budgetBytes {
		oldestKey := contractOrderFlowMemoryKey{}
		oldestUse := ^uint64(0)
		for entryKey, entry := range memory.entries {
			if entry.lastUsed < oldestUse {
				oldestKey, oldestUse = entryKey, entry.lastUsed
			}
		}
		memory.usedBytes -= memory.entries[oldestKey].bytes
		delete(memory.entries, oldestKey)
	}

	memory.useCount++
	memory.entries[key] = &contractOrderFlowMemoryEntry{day: day, bytes: dayBytes, lastUsed: memory.useCount}
	memory.usedBytes += dayBytes
}

// Gathering waits until no one else gathers that day and returns the release; hold it from the recall that found the day missing to the keep that fills it.
func (memory *ContractOrderFlowMemory) Gathering(
	symbol string, day time.Time, priceStep domains.OrderFlowPriceStepDomain,
) func() {
	key := contractOrderFlowMemoryKey{symbol: symbol, dayStart: day.UTC().Unix(), priceStep: priceStep.Key()}

	memory.lock.Lock()
	gatherer, isGathered := memory.gatherers[key]
	if !isGathered {
		gatherer = &contractOrderFlowGatherer{}
		memory.gatherers[key] = gatherer
	}
	gatherer.holders++
	memory.lock.Unlock()

	gatherer.lock.Lock()

	return func() {
		gatherer.lock.Unlock()

		memory.lock.Lock()
		defer memory.lock.Unlock()
		gatherer.holders--
		if gatherer.holders == 0 {
			delete(memory.gatherers, key)
		}
	}
}
