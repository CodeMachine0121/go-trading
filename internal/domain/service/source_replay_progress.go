package service

import domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"

// sourceReplayProgress relays one signal source's bar count as a share of the whole replay, so a strategy with three sources reports a single 0 to 100.
type sourceReplayProgress struct {
	whole       domaininterface.IReplayProgressSink
	sourceIndex int
	sourceCount int
}

// newSourceReplayProgress returns nil when nobody listens, so the proxy is not called for nothing.
func newSourceReplayProgress(
	whole domaininterface.IReplayProgressSink, sourceIndex int, sourceCount int,
) domaininterface.IReplayProgressSink {
	if whole == nil {
		return nil
	}

	return sourceReplayProgress{whole: whole, sourceIndex: sourceIndex, sourceCount: sourceCount}
}

func (sourceReplayProgress sourceReplayProgress) ReplayAdvanced(completedBarCount int, totalBarCount int) {
	sourceReplayProgress.whole.ReplayAdvanced(
		sourceReplayProgress.sourceIndex*totalBarCount+completedBarCount,
		sourceReplayProgress.sourceCount*totalBarCount)
}
