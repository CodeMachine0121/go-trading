package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

type KCandleContractFollowApplication struct {
	kCandleContractFollowService *service.KCandleContractFollowService
}

func NewKCandleContractFollowApplication(
	kCandleContractFollowService *service.KCandleContractFollowService,
) *KCandleContractFollowApplication {
	return &KCandleContractFollowApplication{kCandleContractFollowService: kCandleContractFollowService}
}

// WatchKCandleContracts streams one contract's updates until ctx is cancelled.
func (kCandleContractFollowApplication *KCandleContractFollowApplication) WatchKCandleContracts(
	executionContext context.Context, symbol string,
) (<-chan dto.KCandleFollowUpdateDto, error) {
	return kCandleContractFollowApplication.kCandleContractFollowService.WatchKCandleContracts(
		executionContext, symbol)
}

func (kCandleContractFollowApplication *KCandleContractFollowApplication) Stop() {
	kCandleContractFollowApplication.kCandleContractFollowService.Stop()
}
