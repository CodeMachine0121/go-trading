package domains

import (
	"sync"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// LiveStreamOccupancyDomain counts open live streams per requester and in total.
type LiveStreamOccupancyDomain struct {
	mutex               sync.Mutex
	capacity            vo.LiveStreamCapacityVo
	occupiedByRequester map[string]int
	occupiedTotal       int
}

// NewLiveStreamOccupancyDomain raises a zero or negative capacity to one.
func NewLiveStreamOccupancyDomain(capacity vo.LiveStreamCapacityVo) *LiveStreamOccupancyDomain {
	return &LiveStreamOccupancyDomain{
		capacity: vo.LiveStreamCapacityVo{
			PerRequester: max(1, capacity.PerRequester),
			Total:        max(1, capacity.Total),
		},
		occupiedByRequester: make(map[string]int),
	}
}

// Occupy takes a place, or refuses without taking one when either cap is already reached.
func (occupancy *LiveStreamOccupancyDomain) Occupy(requesterKey string) error {
	occupancy.mutex.Lock()
	defer occupancy.mutex.Unlock()

	if occupancy.occupiedTotal >= occupancy.capacity.Total ||
		occupancy.occupiedByRequester[requesterKey] >= occupancy.capacity.PerRequester {
		return ErrLiveStreamCapacityReached
	}

	occupancy.occupiedByRequester[requesterKey]++
	occupancy.occupiedTotal++

	return nil
}

// Vacate ignores a requester holding nothing, so a doubled release cannot free someone else's place.
func (occupancy *LiveStreamOccupancyDomain) Vacate(requesterKey string) {
	occupancy.mutex.Lock()
	defer occupancy.mutex.Unlock()

	if occupancy.occupiedByRequester[requesterKey] <= 0 {
		return
	}

	occupancy.occupiedByRequester[requesterKey]--
	if occupancy.occupiedByRequester[requesterKey] == 0 {
		delete(occupancy.occupiedByRequester, requesterKey)
	}
	occupancy.occupiedTotal--
}
