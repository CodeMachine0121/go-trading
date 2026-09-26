package dto

// RunnableStrategyScriptDto lets runs name a script instead of carrying it, and never leaves
// the application layer so it cannot be used to read a script.
type RunnableStrategyScriptDto struct {
	Script         string
	ResultType     string
	MarketDataKind string
	// Parameters are as declared; per-run values are never written back.
	Parameters []StrategyScriptParameterWriteDto
	// AuthoredByViewer is false for someone else's published script and for a marketplace copy, whose failure
	// wording its author controls.
	AuthoredByViewer bool
}
