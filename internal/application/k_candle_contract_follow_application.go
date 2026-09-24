package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// KCandleContractFollowApplication orchestrates following a perpetual contract live: a
// viewer says which contract they are looking at and gets the updates for it. It is the
// contract twin of KCandleFollowApplication and shares nothing with it but the shape of
// an update.
type KCandleContractFollowApplication struct {
	kCandleContractFollowService *service.KCandleContractFollowService
}

func NewKCandleContractFollowApplication(
	kCandleContractFollowService *service.KCandleContractFollowService,
) *KCandleContractFollowApplication {
	return &KCandleContractFollowApplication{kCandleContractFollowService: kCandleContractFollowService}
}

// WatchKCandleContracts hands back the live updates for one contract. The viewer leaves
// by ending the context they handed in.
func (kCandleContractFollowApplication *KCandleContractFollowApplication) WatchKCandleContracts(
	executionContext context.Context, symbol string,
) (<-chan dto.KCandleFollowUpdateDto, error) {
	return kCandleContractFollowApplication.kCandleContractFollowService.WatchKCandleContracts(
		executionContext, symbol)
}

// Stop ends every contract follow and closes every viewer's updates.
func (kCandleContractFollowApplication *KCandleContractFollowApplication) Stop() {
	kCandleContractFollowApplication.kCandleContractFollowService.Stop()
}
