package dto

import "time"

// PublishedStrategyScriptDto is the non-owner view and deliberately has no Script field, so
// redaction is enforced by the type.
type PublishedStrategyScriptDto struct {
	ID             uint   `json:"id"`
	Name           string `json:"name"`
	Description    string `json:"description"`
	ResultType     string `json:"resultType"`
	MarketDataKind string `json:"marketDataKind"`
	// Usage is the script's current usage, chart or calculation.
	Usage string `json:"usage"`
	// PublisherEmail is read from the script owner, not stored on the publication.
	PublisherEmail string                       `json:"publisherEmail"`
	PublishedAt    time.Time                    `json:"publishedAt"`
	Parameters     []StrategyScriptParameterDto `json:"parameters"`
}
