package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type KCandleFollowApplication struct {
	kCandleFollowService *service.KCandleFollowService
}

func NewKCandleFollowApplication(
	kCandleFollowService *service.KCandleFollowService,
) *KCandleFollowApplication {
	return &KCandleFollowApplication{kCandleFollowService: kCandleFollowService}
}

// WatchKCandles streams one symbol's updates until ctx is cancelled.
func (kCandleFollowApplication *KCandleFollowApplication) WatchKCandles(
	executionContext context.Context, symbol string,
) (<-chan dto.KCandleFollowUpdateDto, error) {
	return kCandleFollowApplication.kCandleFollowService.WatchKCandles(executionContext, symbol)
}

// RefreshFixedFollows reallocates live-follow slots on markets that cap how many symbols can be followed at once.
func (kCandleFollowApplication *KCandleFollowApplication) RefreshFixedFollows(
	executionContext context.Context,
) error {
	return kCandleFollowApplication.kCandleFollowService.RefreshFixedFollows(executionContext)
}

func (kCandleFollowApplication *KCandleFollowApplication) Stop() {
	kCandleFollowApplication.kCandleFollowService.Stop()
}
