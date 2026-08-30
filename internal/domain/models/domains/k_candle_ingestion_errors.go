package domains

import "errors"

// ErrKCandleIngestionValidation marks a setting the ingestion rules cannot work
// with, as opposed to a K candle that failed its own rules.
var ErrKCandleIngestionValidation = errors.New("k candle ingestion validation failed")
