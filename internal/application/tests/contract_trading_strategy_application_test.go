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

			return tradingStrategy, nil
		})

	writeDto := aTradingStrategyWrite()
	writeDto.MarketDataKind = "contractKCandle"

	tradingStrategyDto, createError := underTest.tradingStrategyApplication.CreateTradingStrategy(
		context.Background(), strategyBotOwnerID, writeDto)

	require.NoError(t, createError)
	assert.Equal(t, "contractKCandle", tradingStrategyDto.MarketDataKind)
}

func TestTradingStrategyApplicationRefusesATradingModeBeforeAnythingIsWritten(t *testing.T) {
	testCases := []struct {
		name    string
		rewrite bool
	}{
		{name: "on creation", rewrite: false},
		{name: "on a rewrite", rewrite: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			// No Save is stubbed: the refusal lands before anything is written.
			underTest := newTradingStrategyApplicationUnderTest(t)
			storedContractStrategy := storedTradingStrategy()
			storedContractStrategy.MarketDataKind = "contractKCandle"
			underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), tradingStrategyID).
				Return(storedContractStrategy, nil).AnyTimes()
			if testCase.rewrite {
				underTest.expectFollowingBots()
			}
			underTest.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(9)).
				Return(aContractScriptOwnedByTheCaller(9), nil)
			underTest.expectNoMarketplaceQuestion()
			writeDto := aTradingStrategyWrite()
			writeDto.MarketDataKind = "contractKCandle"
			writeDto.DeclaredTradingMode = "shortOnly"

			var writeError error
			if testCase.rewrite {
				writeDto.ID = tradingStrategyID
				_, writeError = underTest.tradingStrategyApplication.UpdateTradingStrategy(
					context.Background(), strategyBotOwnerID, writeDto)
			} else {
				_, writeError = underTest.tradingStrategyApplication.CreateTradingStrategy(
					context.Background(), strategyBotOwnerID, writeDto)
			}

			require.ErrorIs(t, writeError, domains.ErrTradingStrategyValidation)
			assert.ErrorContains(t, writeError, "交易方向改由機器人在自己身上選")
		})
	}
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

				return tradingStrategy, nil
			})

		writeDto := aTradingStrategyWrite()
		writeDto.ID = tradingStrategyID

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

// A bot with no kind is a spot bot and may not follow contract rules.
func TestStrategyBotApplicationRefusesASpotBotFollowingAContractTradingStrategy(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)
	underTest.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), botsTradingStrategyID).
		Return(entities.TradingStrategy{
			ID: botsTradingStrategyID, OwnerID: strategyBotOwnerID, Name: "合約黃金交叉",
			MarketDataKind: "contractKCandle",
		}, nil)
	underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	_, createError := underTest.strategyBotApplication.CreateStrategyBot(
		context.Background(), strategyBotOwnerID, aBotWrite())

	require.ErrorIs(t, createError, domains.ErrStrategyBotValidation)
	assert.ErrorContains(t, createError, "這台機器人吃的是 K 線，那份交易策略吃的是合約行情")
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
