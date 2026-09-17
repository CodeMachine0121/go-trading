package domains

import (
	"errors"
	"fmt"
	"slices"
	"strings"
)

// SharedAggregationIntervalDomain is a set of signal sources' coarsenesses, and the
// one question asked of them: is there a single one they all read?
//
// It exists because two gates ask it — saving a trading strategy and replaying one —
// and they have to refuse in the same words. The sentence naming the coarsenesses
// found is written here once; two copies of it would eventually disagree, and the
// person reading them would be told two different things about one fact.
//
// It holds the coarsenesses rather than the sources because that is all the question
// is about. Handed the sources, it would be a model of something it does not judge.
type SharedAggregationIntervalDomain struct {
	aggregationIntervals []string
}

// NewSharedAggregationIntervalDomain reads the coarsenesses in the order the sources
// declared them. Order is kept because the refusal lists them, and a list somebody is
// meant to go and reconcile reads best in the order they see on screen.
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

// Shared is the one coarseness they all read, or a refusal naming the ones found.
//
// One answer rather than two — "are they the same" beside "and here is why not" — is
// what stops a caller pairing them wrongly. There is no way from here to produce a
// disagreement with nothing said about it.
//
// The refusal names the coarsenesses. Told only that they differ, somebody has to
// open every source to see how, and the thing they then have to do is make them the
// same.
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
