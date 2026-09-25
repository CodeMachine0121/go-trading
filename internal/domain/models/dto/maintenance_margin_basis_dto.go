package dto

import "time"

type MaintenanceMarginBasisDto struct {
	// Kind is "tiers" for the full ladder, or "smallestTier" when none existed, which places
	// large positions' liquidation price too far away.
	Kind string `json:"kind"`
	// ConfirmedAt shows when the ladder was confirmed, since ladders have no history and old
	// replays use today's; nil for smallestTier.
	ConfirmedAt *time.Time `json:"confirmedAt"`
}
