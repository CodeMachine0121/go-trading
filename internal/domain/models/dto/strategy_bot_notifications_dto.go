package dto

// StrategyBotNotificationsDto says which conclusions send their round message; turning one off never stops the conclusion itself.
type StrategyBotNotificationsDto struct {
	Buy  bool `json:"buy"`
	Sell bool `json:"sell"`
	// Hold sends one message when a watched symbol returns to holding.
	Hold bool `json:"hold"`
}

// StrategyBotNotificationsWriteDto leaves out what was not said: the default on create, the stored setting on rewrite.
type StrategyBotNotificationsWriteDto struct {
	Buy  *bool
	Sell *bool
	Hold *bool
}
