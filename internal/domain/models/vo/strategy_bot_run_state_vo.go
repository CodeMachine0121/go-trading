package vo

// StrategyBotRunStateVo is whether a bot is running; it is persisted so bots survive restarts.
type StrategyBotRunStateVo string

const (
	// StrategyBotStopped is the initial state and the state after a stop or a halt.
	StrategyBotStopped StrategyBotRunStateVo = "stopped"
	// StrategyBotRunning means the scheduler may pick this bot up.
	StrategyBotRunning StrategyBotRunStateVo = "running"
)
