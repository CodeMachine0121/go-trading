package vo

// StrategyBotHaltReasonVo is why the system stopped a bot its owner had started.
//
// What the four have in common is that running them ten thousand more times gives
// the same answer. Everything that might come right on its own — a closed market,
// candles that have not arrived yet, Telegram not answering — is deliberately
// absent: halting for those would switch off every bot in the system each weekend.
//
// It is a named value rather than a sentence, for the same reason as a delivery
// failure reason: whoever shows it writes their own wording and tells the four apart
// without matching text.
type StrategyBotHaltReasonVo string

const (
	// StrategyBotHaltNone is not a halt. It is what a bot that was stopped by its
	// own owner carries, so that "is it halted" and "why" are one value rather than
	// two that can contradict each other.
	StrategyBotHaltNone StrategyBotHaltReasonVo = ""
	// StrategyBotHaltStrategyUnavailable is a signal source pointing at a strategy
	// that can no longer be seen — deleted by its owner, or adopted from the
	// marketplace and since withdrawn. The two are one reason on purpose: not being
	// able to see a strategy is one fact, and telling them apart would say whether
	// somebody else's strategy still exists.
	StrategyBotHaltStrategyUnavailable StrategyBotHaltReasonVo = "strategyUnavailable"
	// StrategyBotHaltScriptFailed is a script that would not run, or that reached
	// for a parameter name nobody declared.
	StrategyBotHaltScriptFailed StrategyBotHaltReasonVo = "scriptFailed"
	// StrategyBotHaltCredentialRejected is Telegram refusing the bot token. Only a
	// new token fixes it, so waiting is not one of the things to try.
	StrategyBotHaltCredentialRejected StrategyBotHaltReasonVo = "credentialRejected"
	// StrategyBotHaltDestinationNotFound is Telegram not knowing the chat. Only a
	// different chat identifier fixes it.
	StrategyBotHaltDestinationNotFound StrategyBotHaltReasonVo = "destinationNotFound"
)
