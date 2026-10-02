package domains

import "errors"

// ErrKCandleIngestionValidation marks an unusable ingestion setting, as opposed to a K candle failing its own rules.
var ErrKCandleIngestionValidation = errors.New("k candle ingestion validation failed")

// ErrMarketDataNotHeld is the source plainly saying it holds no K candles for that symbol on that day, which a history sync presumes is a closed day; any other failure stays a refusal, because mistaking a refusal for a holiday means hammering a refusing source.
var ErrMarketDataNotHeld = errors.New("market data not held by source")
