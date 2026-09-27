package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func aContractRoundSuggestingAShort() dto.StrategyBotRoundDto {
	return dto.StrategyBotRoundDto{
		Symbol: "ETHUSDT", MarketDataKind: string(vo.MarketDataKindContractKCandle), HasPositionPlan: true,
		PositionPlan: dto.PositionPlanDto{Affordable: true, Direction: "short", ForContract: true},
	}
}

func TestTradeJournalLinkServiceOfferJournalLink(t *testing.T) {
	t.Run("a suggested contract position carries a freshly minted link", func(t *testing.T) {
		opaqueIdentifierProxy := mocks.NewMockIOpaqueIdentifierProxy(gomock.NewController(t))
		opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{Value: "k3y"}, nil)

		round := service.NewTradeJournalLinkService(
			opaqueIdentifierProxy, nil, nil, "https://app.example.com").
			OfferJournalLink(aContractRoundSuggestingAShort())

		assert.Equal(t, "k3y", round.JournalLinkIdentifier)
		assert.Equal(t, "https://app.example.com/contract-trade-journal/new?journalLink=k3y", round.JournalLinkUrl)
	})

	t.Run("a spot round with neither a buy nor an exit mints nothing", func(t *testing.T) {
		opaqueIdentifierProxy := mocks.NewMockIOpaqueIdentifierProxy(gomock.NewController(t))
		spotRound := aContractRoundSuggestingAShort()
		spotRound.MarketDataKind = string(vo.MarketDataKindKCandle)

		round := service.NewTradeJournalLinkService(
			opaqueIdentifierProxy, nil, nil, "https://app.example.com").
			OfferJournalLink(spotRound)

		assert.Empty(t, round.JournalLinkUrl)
	})

	t.Run("a spot exit carries a link to the spot journal", func(t *testing.T) {
		opaqueIdentifierProxy := mocks.NewMockIOpaqueIdentifierProxy(gomock.NewController(t))
		opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{Value: "s9"}, nil)
		spotExit := dto.StrategyBotRoundDto{
			Symbol: "2330", MarketDataKind: string(vo.MarketDataKindKCandle), Verdict: string(vo.SignalSell),
		}

		round := service.NewTradeJournalLinkService(
			opaqueIdentifierProxy, nil, nil, "https://app.example.com").OfferJournalLink(spotExit)

		assert.Equal(t, "https://app.example.com/spot-trade-journal/new?journalLink=s9", round.JournalLinkUrl)
	})

	t.Run("a failed mint still lets the message go without a link", func(t *testing.T) {
		opaqueIdentifierProxy := mocks.NewMockIOpaqueIdentifierProxy(gomock.NewController(t))
		opaqueIdentifierProxy.EXPECT().Mint().Return(vo.OpaqueIdentifierVo{}, errors.New("no randomness"))

		round := service.NewTradeJournalLinkService(
			opaqueIdentifierProxy, nil, nil, "https://app.example.com").
			OfferJournalLink(aContractRoundSuggestingAShort())

		assert.Empty(t, round.JournalLinkIdentifier)
		assert.Empty(t, round.JournalLinkUrl)
	})
}

func TestTradeJournalLinkServiceFindOwnedRound(t *testing.T) {
	const ownerID = uint(7)
	ranAt := time.Date(2026, 9, 25, 14, 0, 0, 0, time.UTC)
	rememberedRound := entities.StrategyBotRunRecord{
		StrategyBotID: 3, RunNumber: 412, RanAt: ranAt, Result: "buy",
		ReferencePrice: decimal.NewNullDecimal(decimal.NewFromInt(1050)),
		SuggestedStake: decimal.NewNullDecimal(decimal.NewFromInt(105000)),
	}
	spotBot := entities.StrategyBot{
		ID: 3, OwnerID: ownerID, Name: "台積電波段", Symbol: "2330",
		MarketDataKind: string(vo.MarketDataKindKCandle), TradingStrategyID: 11,
	}

	newLinkService := func(t *testing.T, found bool, bot entities.StrategyBot, botError error) *service.TradeJournalLinkService {
		mockController := gomock.NewController(t)
		runRecordRepository := mocks.NewMockIStrategyBotRunRecordRepository(mockController)
		runRecordRepository.EXPECT().FindByJournalLinkIdentifier(gomock.Any(), "abc").
			Return(rememberedRound, found, nil).AnyTimes()
		botRepository := mocks.NewMockIStrategyBotRepository(mockController)
		botRepository.EXPECT().FindOne(gomock.Any(), uint(3)).Return(bot, botError).AnyTimes()

		return service.NewTradeJournalLinkService(
			mocks.NewMockIOpaqueIdentifierProxy(mockController), runRecordRepository, botRepository, "https://app.example.com")
	}

	t.Run("the owner reads the round as it was, with the bot that ran it", func(t *testing.T) {
		linkRound, findError := newLinkService(t, true, spotBot, nil).FindOwnedRound(context.Background(), ownerID, "abc")

		require.NoError(t, findError)
		assert.Equal(t, "2330", linkRound.Symbol)
		assert.Equal(t, 412, linkRound.RunNumber)
		assert.Equal(t, uint(11), linkRound.TradingStrategyID)
		assert.Equal(t, string(vo.MarketDataKindKCandle), linkRound.MarketDataKind)
		assert.True(t, linkRound.ReferencePrice.Decimal.Equal(decimal.NewFromInt(1050)))
	})

	t.Run("a round the bot has forgotten says so", func(t *testing.T) {
		_, findError := newLinkService(t, false, spotBot, nil).FindOwnedRound(context.Background(), ownerID, "abc")

		assert.ErrorIs(t, findError, domains.ErrJournalLinkNotFound)
		assert.Contains(t, findError.Error(), "這一輪的建議已不在紀錄中")
	})

	t.Run("somebody else's bot is not found", func(t *testing.T) {
		_, findError := newLinkService(t, true, spotBot, nil).FindOwnedRound(context.Background(), ownerID+1, "abc")

		assert.ErrorIs(t, findError, domains.ErrJournalLinkNotFound)
		assert.Contains(t, findError.Error(), "找不到這一輪的建議")
	})

	t.Run("a deleted bot is not found", func(t *testing.T) {
		_, findError := newLinkService(t, true, entities.StrategyBot{}, domains.StrategyBotNotFound(3)).
			FindOwnedRound(context.Background(), ownerID, "abc")

		assert.ErrorIs(t, findError, domains.ErrJournalLinkNotFound)
	})

	t.Run("a bot that cannot be read passes the failure on", func(t *testing.T) {
		_, findError := newLinkService(t, true, entities.StrategyBot{}, errors.New("database down")).
			FindOwnedRound(context.Background(), ownerID, "abc")

		assert.EqualError(t, findError, "database down")
	})

	t.Run("a blank identifier or a forgotten round leaves no round to copy", func(t *testing.T) {
		assert.Nil(t, newLinkService(t, true, spotBot, nil).FindOwnedRoundIfRemembered(context.Background(), ownerID, ""))
		assert.Nil(t, newLinkService(t, false, spotBot, nil).FindOwnedRoundIfRemembered(context.Background(), ownerID, "abc"))
		assert.NotNil(t, newLinkService(t, true, spotBot, nil).FindOwnedRoundIfRemembered(context.Background(), ownerID, "abc"))
	})
}
