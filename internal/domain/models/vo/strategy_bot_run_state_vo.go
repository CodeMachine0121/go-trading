package vo

// StrategyBotRunStateVo is whether a strategy bot is running.
//
// It is stored rather than held in memory, and that is the whole reason it is a
// value with a name instead of a goroutine that happens to exist. A bot whose
// "running" lived only in a process would stop the next time that process
// restarted, without anybody being told — and a person who pressed play and then
// received nothing all night cannot tell that apart from a quiet market.
type StrategyBotRunStateVo string

const (
	// StrategyBotStopped is the state every bot is created in, and the state it
	// returns to when its owner presses stop or when the system halts it.
	StrategyBotStopped StrategyBotRunStateVo = "stopped"
	// StrategyBotRunning means the scan is entitled to pick this bot up.
	StrategyBotRunning StrategyBotRunStateVo = "running"
)
