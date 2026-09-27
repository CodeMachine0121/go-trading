package service_test

import (
	"errors"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"go.uber.org/mock/gomock"
)

func aContractRoundSuggestingAShort() dto.StrategyBotRoundDto {
	return dto.StrategyBotRoundDto{
		Symbol: "ETHUSDT", MarketDataKind: string(vo.MarketDataKindContractKCandle), HasPositionPlan: true,
		PositionPlan: dto.PositionPlanDto{Affordable: true, Direction: "short", ForContract: true},
	}
}

func TestContractTradeJournalLinkServiceOfferJournalLink(t *testing.T) {
	t.Run("a suggested contract position carries a freshly minted link", func(t *testing.T) {
		opaqueIdentifierProxy := mocks.NewMockIOpaqueIdentifierProxy(gomock.NewController(t))
		opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{Value: "k3y"}, nil)

		round := service.NewContractTradeJournalLinkService(opaqueIdentifierProxy, "https://app.example.com").
			OfferJournalLink(aContractRoundSuggestingAShort())

		assert.Equal(t, "k3y", round.JournalLinkIdentifier)
		assert.Equal(t, "https://app.example.com/contract-trade-journal/new?journalLink=k3y", round.JournalLinkUrl)
	})

	t.Run("a round with nothing to open mints nothing", func(t *testing.T) {
		opaqueIdentifierProxy := mocks.NewMockIOpaqueIdentifierProxy(gomock.NewController(t))
		spotRound := aContractRoundSuggestingAShort()
		spotRound.MarketDataKind = string(vo.MarketDataKindKCandle)

		round := service.NewContractTradeJournalLinkService(opaqueIdentifierProxy, "https://app.example.com").
			OfferJournalLink(spotRound)

		assert.Empty(t, round.JournalLinkUrl)
	})

	t.Run("a failed mint still lets the message go without a link", func(t *testing.T) {
		opaqueIdentifierProxy := mocks.NewMockIOpaqueIdentifierProxy(gomock.NewController(t))
		opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{}, errors.New("no randomness"))

		round := service.NewContractTradeJournalLinkService(opaqueIdentifierProxy, "https://app.example.com").
			OfferJournalLink(aContractRoundSuggestingAShort())

		assert.Empty(t, round.JournalLinkIdentifier)
		assert.Empty(t, round.JournalLinkUrl)
	})
}
