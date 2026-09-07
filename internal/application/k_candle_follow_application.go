package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// KCandleFollowApplication orchestrates following a market live: a viewer says what
// they are looking at and gets the updates for it, and the markets that limit how
// many symbols may be followed at once get their places handed out again.
type KCandleFollowApplication struct {
	kCandleFollowService *service.KCandleFollowService
}

func NewKCandleFollowApplication(
	kCandleFollowService *service.KCandleFollowService,
) *KCandleFollowApplication {
	return &KCandleFollowApplication{kCandleFollowService: kCandleFollowService}
}

// WatchKCandles hands back the updates for one trading symbol. The viewer leaves by
// ending the context they handed in.
func (kCandleFollowApplication *KCandleFollowApplication) WatchKCandles(
	executionContext context.Context, symbol string,
) (<-chan dto.KCandleFollowUpdateDto, error) {
	return kCandleFollowApplication.kCandleFollowService.WatchKCandles(executionContext, symbol)
}

// RefreshFixedFollows hands out each limited market's live places again, following
// what should be followed and letting go of what should not.
func (kCandleFollowApplication *KCandleFollowApplication) RefreshFixedFollows(
	executionContext context.Context,
) error {
	return kCandleFollowApplication.kCandleFollowService.RefreshFixedFollows(executionContext)
}

// Stop ends every live follow and closes every viewer's updates.
//
// It is here rather than reached for on the domain service directly because the
// service is not otherwise handed out: everything above it holds this, so shutting
// the follows down would otherwise be the one thing that had to reach past it.
func (kCandleFollowApplication *KCandleFollowApplication) Stop() {
	kCandleFollowApplication.kCandleFollowService.Stop()
}
