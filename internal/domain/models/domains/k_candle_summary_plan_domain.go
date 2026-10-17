package domains

import "time"

// KCandleSummaryDayLength is the unit summaries are filled, invalidated and versioned in: one UTC day, the coarsest bucket, so every hour belongs to exactly one day.
const KCandleSummaryDayLength = 24 * time.Hour
