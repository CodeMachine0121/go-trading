package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// When the venue cannot be read, the round still suggests — as a contract with no
// specification and no funding record — rather than going without a suggestion.
func TestStrategyBotRunApplicationStillSuggestsWhenTheVenueCannotBeRead(t *testing.T) {
	underTest := newStrategyBotRunUnderTest(t)
	underTest.makeTheRulesContract(vo.ContractTradingModeLongShort)
	underTest.expectDeliverySetting()
	underTest.expectContractSources(vo.SignalBuy, vo.SignalBuy)
	underTest.expectTheContractsLatestCandle("100")
	storageFailure := errors.New("the database went away")
	underTest.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.ContractTradingSymbol{}, false, storageFailure)
	underTest.contractMaintenanceMarginTierRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(nil, storageFailure)
	underTest.contractFundingRateSettlementRepository.EXPECT().FindLatest(gomock.Any(), "BTCUSDT").
		Return(entities.ContractFundingRateSettlement{}, false, storageFailure)

	dueBot := aDueContractBot("")
	dueBot.PositionPlanCapital = decimal.NewFromInt(1000)
	dueBot.PositionPlanLeverage = decimal.NewFromInt(5)
	underTest.strategyBotRepository.EXPECT().FindDue(gomock.Any(), botRunNow, 4).
		Return([]entities.StrategyBot{dueBot}, nil)
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
		Return(dueBot, nil).AnyTimes()
	underTest.messageDeliveryProxy.EXPECT().
		Deliver(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, _ vo.MessageDeliveryCredentialVo, message string,
		) (vo.DeliveryFailureReasonVo, error) {
			assert.Contains(t, message, "保證金 1000（5 倍槓桿，名目 5000）")
			assert.Contains(t, message, "這個合約標的還沒有交易規格")
			assert.Contains(t, message, "還沒有資金費率紀錄")

			return vo.DeliveryFailureNone, nil
		})
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	_, runError := underTest.strategyBotRunApplication.RunDueRounds(context.Background())

	require.NoError(t, runError)
}
