package dto

// AvailableStrategyScriptsDto is what one person picks from day to day: the strategy scripts
// they wrote, and the ones they took off the marketplace.
//
// The two are separate lists rather than one sorted list, and that is deliberate.
// A single list would need one type able to hold both — which means a script that
// is sometimes there, which is the one thing this feature must not have. Splitting
// them also makes "mine first, adopted after" a fact about the shape instead of an
// ordering convention a caller has to preserve.
type AvailableStrategyScriptsDto struct {
	// Mine are the caller's own, script included, ordered by name.
	Mine []StrategyScriptDto `json:"mine"`
	// Adopted are the ones taken from the marketplace, without their scripts,
	// ordered by name. A strategy script whose owner has since withdrawn it is not here:
	// the adoption went with the publication.
	Adopted []PublishedStrategyScriptDto `json:"adopted"`
}
