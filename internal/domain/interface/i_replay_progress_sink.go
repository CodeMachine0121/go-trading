package _interface

//go:generate go tool mockgen -source=i_replay_progress_sink.go -destination=mocks/mock_i_replay_progress_sink.go -package=mocks

// IReplayProgressSink hears how far a replay has got, bar by bar, so a run that outlives its request can be watched.
type IReplayProgressSink interface {
	ReplayAdvanced(completedBarCount int, totalBarCount int)
}
