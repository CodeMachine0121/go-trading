package vo

// StrategyBotHaltReasonVo is why the system stopped a bot its owner had started.
//
// What they have in common is that running them ten thousand more times gives
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
	// StrategyBotHaltStrategyScriptUnavailable is a signal source pointing at a strategy script
	// that can no longer be seen — deleted by its owner, or adopted from the
	// marketplace and since withdrawn. The two are one reason on purpose: not being
	// able to see a strategy script is one fact, and telling them apart would say whether
	// somebody else's strategy script still exists.
	//
	// The stored value stays as it was written before a strategy became a strategy
	// script. It is read straight back out of the bot it halted and handed to
	// whoever asks, so renaming it would leave every bot halted before the rename
	// reporting one spelling and every bot halted after it reporting another.
	StrategyBotHaltStrategyScriptUnavailable StrategyBotHaltReasonVo = "strategyUnavailable"
	// StrategyBotHaltTradingStrategyUnavailable is the trading strategy this bot
	// follows no longer being readable.
	//
	// Deleting one is refused while any bot follows it, so this is very nearly
	// unreachable — but "very nearly" is not "never": a delete and a create naming
	// the same trading strategy can pass each other, and the bot left behind points
	// at nothing. Skipping the round instead would leave that bot reporting itself
	// as running for ever while saying nothing, which is the one outcome its owner
	// can neither see nor fix.
	StrategyBotHaltTradingStrategyUnavailable StrategyBotHaltReasonVo = "tradingStrategyUnavailable"
	// StrategyBotHaltScriptFailed is a script that would not run, or that reached
	// for a parameter name nobody declared.
	StrategyBotHaltScriptFailed StrategyBotHaltReasonVo = "scriptFailed"
	// StrategyBotHaltCredentialRejected is Telegram refusing the bot token. Only a
	// new token fixes it, so waiting is not one of the things to try.
	StrategyBotHaltCredentialRejected StrategyBotHaltReasonVo = "credentialRejected"
	// StrategyBotHaltDestinationNotFound is Telegram not knowing the chat. Only a
	// different chat identifier fixes it.
	StrategyBotHaltDestinationNotFound StrategyBotHaltReasonVo = "destinationNotFound"
	// StrategyBotHaltDeliveryNotConfigured is the owner having removed their
	// delivery setting while the bot was running.
	//
	// It is a halt and not a skipped round, even though nothing is broken: a bot
	// with nowhere to speak is a bot for which running and stopped are the same
	// state, and that is the very reason starting one is refused. Without it, a
	// person who removes their setting is left with bots that stay "running",
	// carry no reason, say nothing, and never will.
	StrategyBotHaltDeliveryNotConfigured StrategyBotHaltReasonVo = "deliveryNotConfigured"
)
