package domains

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// SharedAggregationIntervalDomain decides whether signal sources share one interval, so strategy save and replay refuse in the same words.
type SharedAggregationIntervalDomain struct {
	aggregationIntervals []string
}

// NewSharedAggregationIntervalDomain deduplicates while keeping declaration order, which is how the refusal lists them.
func NewSharedAggregationIntervalDomain(
	aggregationIntervals []string,
) SharedAggregationIntervalDomain {
	distinctIntervals := make([]string, 0, len(aggregationIntervals))

	for _, aggregationInterval := range aggregationIntervals {
		interval := strings.TrimSpace(aggregationInterval)
		if !slices.Contains(distinctIntervals, interval) {
			distinctIntervals = append(distinctIntervals, interval)
		}
	}

	return SharedAggregationIntervalDomain{aggregationIntervals: distinctIntervals}
}

// Shared returns the single common interval, or a refusal naming every interval found.
func (sharedAggregationIntervalDomain SharedAggregationIntervalDomain) Shared() (string, error) {
	if len(sharedAggregationIntervalDomain.aggregationIntervals) == 0 {
		return "", errors.New("這一份交易策略沒有任何信號來源")
	}

	if len(sharedAggregationIntervalDomain.aggregationIntervals) > 1 {
		return "", fmt.Errorf(
			"這一份交易策略的信號來源目前用了 %s 這幾種彙總刻度。"+
				"一份交易策略只看一種粗細的 K 線——"+
				"「一小時的那一棒」與「五分鐘的那一棒」不是同一根，"+
				"請先把每個來源調成同一種",
			strings.Join(sharedAggregationIntervalDomain.aggregationIntervals, "、"))
	}

	return sharedAggregationIntervalDomain.aggregationIntervals[0], nil
}
