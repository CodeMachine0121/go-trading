package dto

// AvailableStrategyScriptsDto keeps own and adopted scripts as separate lists so adopted
// entries never need a script field.
type AvailableStrategyScriptsDto struct {
	// Mine are ordered by name and include the script.
	Mine []StrategyScriptDto `json:"mine"`
	// Adopted are ordered by name, exclude the script, and drop scripts whose owner has
	// withdrawn the publication.
	Adopted []PublishedStrategyScriptDto `json:"adopted"`
}
