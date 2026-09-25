package domains_test

import (
	"strconv"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
)

var liveStreamCapacity = vo.LiveStreamCapacityVo{PerRequester: 20, Total: 1000}

func TestLiveStreamOccupancyRefusesPastEitherCap(t *testing.T) {
	testCases := []struct {
		name          string
		openedBefore  map[string]int
		vacatedBefore map[string]int
		requesterKey  string
		expectedError error
	}{
		{
			name:         "the twentieth stream of one requester opens",
			openedBefore: map[string]int{"user:7": 19},
			requesterKey: "user:7",
		},
		{
			name:          "the twenty-first stream of one requester is refused",
			openedBefore:  map[string]int{"user:7": 20},
			requesterKey:  "user:7",
			expectedError: domains.ErrLiveStreamCapacityReached,
		},
		{
			name:          "closing one frees its place at once",
			openedBefore:  map[string]int{"user:7": 20},
			vacatedBefore: map[string]int{"user:7": 1},
			requesterKey:  "user:7",
		},
		{
			name:          "a refused attempt takes no place, so one close makes room for exactly one",
			openedBefore:  map[string]int{"user:7": 21},
			vacatedBefore: map[string]int{"user:7": 1},
			requesterKey:  "user:7",
		},
		{
			name:          "the service-wide cap applies to a requester holding nothing",
			openedBefore:  spreadOver(50, 20),
			requesterKey:  "user:9999",
			expectedError: domains.ErrLiveStreamCapacityReached,
		},
		{
			name:          "a release by someone holding nothing frees nobody else's place",
			openedBefore:  spreadOver(50, 20),
			vacatedBefore: map[string]int{"user:9999": 1},
			requesterKey:  "user:9998",
			expectedError: domains.ErrLiveStreamCapacityReached,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			occupancy := domains.NewLiveStreamOccupancyDomain(liveStreamCapacity)
			for requesterKey, count := range testCase.openedBefore {
				for range count {
					_ = occupancy.Occupy(requesterKey)
				}
			}
			for requesterKey, count := range testCase.vacatedBefore {
				for range count {
					occupancy.Vacate(requesterKey)
				}
			}

			occupyError := occupancy.Occupy(testCase.requesterKey)

			assert.Equal(t, testCase.expectedError, occupyError)
			if testCase.expectedError != nil {
				assert.Equal(t, "同時開著的即時跟盤已達上限，請先關掉一些再開", occupyError.Error())
			}
		})
	}
}

func spreadOver(requesterCount int, streamsEach int) map[string]int {
	opened := make(map[string]int, requesterCount)
	for index := range requesterCount {
		opened["user:"+strconv.Itoa(index+1)] = streamsEach
	}

	return opened
}
