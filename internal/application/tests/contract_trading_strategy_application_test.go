package application_test

import (
	"context"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func aContractScriptOwnedByTheCaller(id uint) entities.StrategyScript {
	strategyScript := aScriptOwnedByTheCaller(id)
	strategyScript.MarketDataKind = string(vo.MarketDataKindContractKCandle)

	return strategyScript
}

func TestTradingStrategyApplicationCreatesAContractTradingStrategy(t *testing.T) {
	underTest := newTradingStrategyApplicationUnderTest(t)

	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(aContractScriptOwnedByTheCaller(9), nil)
	underTest.expectNoMarketplaceQuestion()
	underTest.tradingStrategyRepository.EXPECT().
		Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, tradingStrategy entities.TradingStrategy,
		) (entities.TradingStrategy, error) {
			assert.Equal(t, "contractKCandle", tradingStrategy.MarketDataKind)
			assert.Equal(t, "shortOnly", tradingStrategy.TradingMode)

			return tradingStrategy, nil
		})

	writeDto := aTradingStrategyWrite()
	writeDto.MarketDataKind = "contractKCandle"
	writeDto.TradingMode = "shortOnly"

	tradingStrategyDto, createError := underTest.tradingStrategyApplication.CreateTradingStrategy(
		context.Background(), strategyBotOwnerID, writeDto)

	require.NoError(t, createError)
	assert.Equal(t, "contractKCandle", tradingStrategyDto.MarketDataKind)
	assert.Equal(t, "shortOnly", tradingStrategyDto.TradingMode)
}

func TestTradingStrategyApplicationRefusesAKCandleTradingStrategyOfContractScripts(t *testing.T) {
	underTest := newTradingStrategyApplicationUnderTest(t)

	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(aContractScriptOwnedByTheCaller(9), nil)
	underTest.expectNoMarketplaceQuestion()

	_, createError := underTest.tradingStrategyApplication.CreateTradingStrategy(
		context.Background(), strategyBotOwnerID, aTradingStrategyWrite())

	require.ErrorIs(t, createError, domains.ErrTradingStrategyValidation)
	assert.ErrorContains(t, createError, `信號來源 "A"`)
}

func TestTradingStrategyApplicationKeepsTheKindOnARewrite(t *testing.T) {
	storedContractStrategy := storedTradingStrategy()
	storedContractStrategy.MarketDataKind = "contractKCandle"
	storedContractStrategy.TradingMode = "longShort"

	t.Run("a rewrite saying nothing about the kind keeps it", func(t *testing.T) {
		underTest := newTradingStrategyApplicationUnderTest(t)
		underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).
			Return(storedContractStrategy, nil).Times(2)
		underTest.expectFollowingBots()
		underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
			Return(aContractScriptOwnedByTheCaller(9), nil)
		underTest.expectNoMarketplaceQuestion()
		underTest.tradingStrategyRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(
				_ context.Context, tradingStrategy entities.TradingStrategy,
			) (entities.TradingStrategy, error) {
				assert.Equal(t, "contractKCandle", tradingStrategy.MarketDataKind)
				assert.Equal(t, "longOnly", tradingStrategy.TradingMode)

				return tradingStrategy, nil
			})

		writeDto := aTradingStrategyWrite()
		writeDto.ID = tradingStrategyID
		writeDto.TradingMode = "longOnly"

		_, updateError := underTest.tradingStrategyApplication.UpdateTradingStrategy(
			context.Background(), strategyBotOwnerID, writeDto)

		require.NoError(t, updateError)
	})

	t.Run("a rewrite naming the other kind is refused", func(t *testing.T) {
		underTest := newTradingStrategyApplicationUnderTest(t)
		underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).
			Return(storedTradingStrategy(), nil).Times(2)
		underTest.expectFollowingBots()
		underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
			Return(aContractScriptOwnedByTheCaller(9), nil)
		underTest.expectNoMarketplaceQuestion()

		writeDto := aTradingStrategyWrite()
		writeDto.ID = tradingStrategyID
		writeDto.MarketDataKind = "contractKCandle"

		_, updateError := underTest.tradingStrategyApplication.UpdateTradingStrategy(
			context.Background(), strategyBotOwnerID, writeDto)

		require.ErrorIs(t, updateError, domains.ErrTradingStrategyValidation)
		assert.ErrorContains(t, updateError, "行情種類建立後不得更換")
	})
}

func TestStrategyBotApplicationRefusesAContractTradingStrategy(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)
	underTest.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), botsTradingStrategyID).
		Return(entities.TradingStrategy{
			ID: botsTradingStrategyID, OwnerID: strategyBotOwnerID, Name: "合約黃金交叉",
			MarketDataKind: "contractKCandle", TradingMode: "longShort",
		}, nil)
	underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	_, createError := underTest.strategyBotApplication.CreateStrategyBot(
		context.Background(), strategyBotOwnerID, aBotWrite())

	require.ErrorIs(t, createError, domains.ErrStrategyBotValidation)
	assert.ErrorContains(t, createError, "策略機器人目前只跑 K 線")
}

func TestStrategyBotApplicationRefusesATradingStrategyOfAnUnreadableKind(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)
	underTest.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), botsTradingStrategyID).
		Return(entities.TradingStrategy{
			ID: botsTradingStrategyID, OwnerID: strategyBotOwnerID, Name: "壞掉的", MarketDataKind: "option",
		}, nil)

	_, createError := underTest.strategyBotApplication.CreateStrategyBot(
		context.Background(), strategyBotOwnerID, aBotWrite())

	assert.Error(t, createError)
}

func TestTradingStrategyApplicationRefusesARewriteOfAStoredStrategyOfAnUnreadableKind(t *testing.T) {
	underTest := newTradingStrategyApplicationUnderTest(t)
	unreadable := storedTradingStrategy()
	unreadable.MarketDataKind = "option"
	underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).
		Return(unreadable, nil).Times(2)
	underTest.expectFollowingBots()
	underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
		Return(aScriptOwnedByTheCaller(9), nil)
	underTest.expectNoMarketplaceQuestion()

	writeDto := aTradingStrategyWrite()
	writeDto.ID = tradingStrategyID

	_, updateError := underTest.tradingStrategyApplication.UpdateTradingStrategy(
		context.Background(), strategyBotOwnerID, writeDto)

	assert.Error(t, updateError)
}
