package vo

// AggregationIntervalVo is how long one aggregated K candle covers. A query
// declares exactly one of these; every candle it gets back covers that much time.
// Immutable, no behavior — how an interval is read, defaulted, aligned and counted
// lives in AggregationIntervalDomain.
type AggregationIntervalVo string

const (
	// AggregationIntervalFiveMinutes is one candle per five minutes, the interval
	// assumed when a caller declares nothing. It matches the length one stored K
	// candle already covers, so aggregating at it changes nothing.
	AggregationIntervalFiveMinutes AggregationIntervalVo = "5m"
	// AggregationIntervalFifteenMinutes is one candle per quarter of an hour.
	AggregationIntervalFifteenMinutes AggregationIntervalVo = "15m"
	// AggregationIntervalOneHour is one candle per hour.
	AggregationIntervalOneHour AggregationIntervalVo = "1h"
	// AggregationIntervalFourHours is one candle per four hours.
	AggregationIntervalFourHours AggregationIntervalVo = "4h"
	// AggregationIntervalOneDay is one candle per day.
	AggregationIntervalOneDay AggregationIntervalVo = "1d"
)
