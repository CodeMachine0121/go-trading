package vo

// AggregationIntervalVo is how long one aggregated K candle covers; parsing and alignment live in AggregationIntervalDomain.
type AggregationIntervalVo string

const (
	// AggregationIntervalOneMinute is the default and matches a stored K candle, so aggregating at it changes nothing.
	AggregationIntervalOneMinute      AggregationIntervalVo = "1m"
	AggregationIntervalFiveMinutes    AggregationIntervalVo = "5m"
	AggregationIntervalFifteenMinutes AggregationIntervalVo = "15m"
	AggregationIntervalOneHour        AggregationIntervalVo = "1h"
	AggregationIntervalFourHours      AggregationIntervalVo = "4h"
	AggregationIntervalOneDay         AggregationIntervalVo = "1d"
)
