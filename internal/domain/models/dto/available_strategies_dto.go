package dto

// AvailableStrategyScriptsDto keeps own and adopted scripts as separate lists so adopted
// entries never need a script field.
type AvailableStrategyScriptsDto struct {
	// Mine are ordered by name and include the script.
	Mine []StrategyScriptDto `json:"mine"`
	// Adopted are the viewer's marketplace copies, ordered by name and without the script.
	Adopted []StrategyScriptDto `json:"adopted"`
}
