package dto

import "time"

// MaintenanceMarginBasisDto is which maintenance margin figures a contract replay used,
// said out loud because both answers have a cost the reader should know about.
type MaintenanceMarginBasisDto struct {
	// Kind is "tiers" when the symbol's full ladder was used, or "smallestTier" when
	// there was no ladder and the trading specification's smallest tier stood in for
	// every size — which reads the liquidation price of a large position too far away.
	Kind string `json:"kind"`
	// ConfirmedAt is when the ladder used was confirmed. The ladder has no history, so a
	// replay of last year uses today's; this is the date that makes that visible. It is
	// absent for the smallest tier.
	ConfirmedAt *time.Time `json:"confirmedAt"`
}
