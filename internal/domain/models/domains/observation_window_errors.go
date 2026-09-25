package domains

import "errors"

// ErrObservationWindowValidation marks a window with no start or with start not before end; callers wrap it in their own sentinels.
var ErrObservationWindowValidation = errors.New("observation window validation failed")
