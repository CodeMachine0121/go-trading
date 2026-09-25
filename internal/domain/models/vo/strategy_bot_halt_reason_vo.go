package vo

// StrategyBotHaltReasonVo is why the system stopped a bot; only permanent failures qualify, never transient ones like a closed market or Telegram being down.
type StrategyBotHaltReasonVo string

const (
	// StrategyBotHaltNone means the bot was not halted by the system.
	StrategyBotHaltNone StrategyBotHaltReasonVo = ""
	// StrategyBotHaltStrategyScriptUnavailable covers both a deleted and a withdrawn script so it does not reveal whether someone else's script exists; the stored value predates the strategy-script rename and must stay.
	StrategyBotHaltStrategyScriptUnavailable StrategyBotHaltReasonVo = "strategyUnavailable"
	// StrategyBotHaltTradingStrategyUnavailable is nearly unreachable (deleting a followed strategy is refused) but a racing delete can still orphan a bot.
	StrategyBotHaltTradingStrategyUnavailable StrategyBotHaltReasonVo = "tradingStrategyUnavailable"
	// StrategyBotHaltScriptFailed is a script that would not run or read an undeclared parameter.
	StrategyBotHaltScriptFailed StrategyBotHaltReasonVo = "scriptFailed"
	// StrategyBotHaltCredentialRejected means only a new bot token fixes it.
	StrategyBotHaltCredentialRejected StrategyBotHaltReasonVo = "credentialRejected"
	// StrategyBotHaltDestinationNotFound means only a different chat identifier fixes it.
	StrategyBotHaltDestinationNotFound StrategyBotHaltReasonVo = "destinationNotFound"
	// StrategyBotHaltDeliveryNotConfigured halts a bot whose owner removed their delivery setting, since a bot with nowhere to speak is effectively stopped.
	StrategyBotHaltDeliveryNotConfigured StrategyBotHaltReasonVo = "deliveryNotConfigured"
)
