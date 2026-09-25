package dto

// StrategyScriptReferencesDto answers both questions a rewrite asks at once: does anything use the
// script at all, and which of the owner's own bots are running on it right now.
type StrategyScriptReferencesDto struct {
	// TotalCount counts every bot of any owner, running or not.
	TotalCount int
	// RunningBotNames holds only the script owner's running bots, since only they block the owner.
	RunningBotNames []string
}
