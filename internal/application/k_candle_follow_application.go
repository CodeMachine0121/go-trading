package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// KCandleFollowApplication orchestrates the one use case of following a market
// live: a viewer says what they are looking at, and gets the updates for it.
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
