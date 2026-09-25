package domains

import "errors"

// ErrKCandleIngestionValidation marks an unusable ingestion setting, as opposed to a K candle failing its own rules.
var ErrKCandleIngestionValidation = errors.New("k candle ingestion validation failed")
