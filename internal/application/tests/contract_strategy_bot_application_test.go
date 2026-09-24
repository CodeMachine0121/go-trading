package application_test

import (
	"context"
	"errors"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// aContractBotWrite is a contract bot as it arrives to be saved: it watches BTCUSDT on
// the contract side and follows the rules botsTradingStrategyID names.
func aContractBotWrite() dto.StrategyBotWriteDto {
	writeDto := aBotWrite()
	writeDto.Name = "合約突破"
	writeDto.MarketDataKind = string(vo.MarketDataKindContractKCandle)

	return writeDto
}

// expectTheNamedTradingStrategyIsOfKind makes the rules the bot names this person's,
// written for this kind of market.
func (underTest strategyBotApplicationUnderTest) expectTheNamedTradingStrategyIsOfKind(
	marketDataKind vo.MarketDataKindVo,
) {
	underTest.tradingStrategyRepository.EXPECT().
		FindOne(gomock.Any(), botsTradingStrategyID).
		Return(entities.TradingStrategy{
			ID: botsTradingStrategyID, OwnerID: strategyBotOwnerID, Name: "合約黃金交叉",
			MarketDataKind: string(marketDataKind), TradingMode: string(vo.ContractTradingModeLongShort),
		}, nil)
}

// expectTheContract says how the system stands with BTCUSDT on the contract side:
// whether it follows it, and the ladder it holds for it.
func (underTest strategyBotApplicationUnderTest) expectTheContract(
	isWatched bool, tiers []entities.ContractMaintenanceMarginTier,
) {
	underTest.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: isWatched}, true, nil)
	underTest.contractMaintenanceMarginTierRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(tiers, nil)
}

// aLadderAllowing is a ladder whose smallest tier allows that much leverage.
func aLadderAllowing(maximumLeverage int) []entities.ContractMaintenanceMarginTier {
	return []entities.ContractMaintenanceMarginTier{
		{Symbol: "BTCUSDT", Tier: 1, MaximumLeverage: maximumLeverage},
		{Symbol: "BTCUSDT", Tier: 2, MaximumLeverage: maximumLeverage / 5},
	}
}

// storedContractBot is a contract bot as it comes back out of storage.
func storedContractBot(runState vo.StrategyBotRunStateVo) entities.StrategyBot {
	bot := storedBot(runState)
	bot.Name = "合約突破"
	bot.MarketDataKind = string(vo.MarketDataKindContractKCandle)
	bot.PositionPlanLeverage = decimal.NewFromInt(5)

	return bot
}

func TestStrategyBotApplicationCreatesAContractBot(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)
	underTest.expectTheNamedTradingStrategyIsOfKind(vo.MarketDataKindContractKCandle)
	underTest.expectTheContract(true, aLadderAllowing(125))
	underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) (entities.StrategyBot, error) {
			assert.Equal(t, string(vo.MarketDataKindContractKCandle), bot.MarketDataKind)
			assert.Equal(t, "5", bot.PositionPlanLeverage.String())

			return storedContractBot(vo.StrategyBotStopped), nil
		})

	writeDto := aContractBotWrite()
	writeDto.DeclaredLeverage = decimal.NewFromInt(5)

	botDto, createError := underTest.strategyBotApplication.CreateStrategyBot(
		context.Background(), strategyBotOwnerID, writeDto)

	require.NoError(t, createError)
	assert.Equal(t, string(vo.MarketDataKindContractKCandle), botDto.MarketDataKind)
	assert.Equal(t, "5", botDto.PositionPlan.Leverage.String())
}

func TestStrategyBotApplicationRefusesAContractBotItCannotSave(t *testing.T) {
	testCases := []struct {
		name            string
		arrange         func(underTest strategyBotApplicationUnderTest)
		leverage        int64
		expectedRefusal string
	}{
		{
			name: "one following a K candle trading strategy",
			arrange: func(underTest strategyBotApplicationUnderTest) {
				underTest.expectTheNamedTradingStrategyIsOfKind(vo.MarketDataKindKCandle)
			},
			expectedRefusal: "這台機器人吃的是合約行情，那份交易策略吃的是 K 線",
		},
		{
			name: "one watching a contract nobody follows",
			arrange: func(underTest strategyBotApplicationUnderTest) {
				underTest.expectTheNamedTradingStrategyIsOfKind(vo.MarketDataKindContractKCandle)
				underTest.expectTheContract(false, nil)
			},
			expectedRefusal: "BTCUSDT 不在合約追蹤名單上",
		},
		{
			name: "one asking for more leverage than the contract allows",
			arrange: func(underTest strategyBotApplicationUnderTest) {
				underTest.expectTheNamedTradingStrategyIsOfKind(vo.MarketDataKindContractKCandle)
				underTest.expectTheContract(true, aLadderAllowing(125))
			},
			leverage:        150,
			expectedRefusal: "這個合約標的最高只能開 125 倍槓桿",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotApplicationUnderTest(t)
			testCase.arrange(underTest)
			underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

			writeDto := aContractBotWrite()
			writeDto.DeclaredLeverage = decimal.NewFromInt(testCase.leverage)

			_, createError := underTest.strategyBotApplication.CreateStrategyBot(
				context.Background(), strategyBotOwnerID, writeDto)

			require.ErrorIs(t, createError, domains.ErrStrategyBotValidation)
			assert.ErrorContains(t, createError, testCase.expectedRefusal)
		})
	}
}

// A contract with no ladder yet names no ceiling, and saving does not invent one.
func TestStrategyBotApplicationSavesAContractBotWhoseContractHasNoLadderYet(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)
	underTest.expectTheNamedTradingStrategyIsOfKind(vo.MarketDataKindContractKCandle)
	underTest.expectTheContract(true, nil)
	underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		Return(storedContractBot(vo.StrategyBotStopped), nil)

	writeDto := aContractBotWrite()
	writeDto.DeclaredLeverage = decimal.NewFromInt(150)

	_, createError := underTest.strategyBotApplication.CreateStrategyBot(
		context.Background(), strategyBotOwnerID, writeDto)

	assert.NoError(t, createError)
}

// A spot bot is not asked about the contract side at all: the two contract readers
// have no expectations, so reaching either would fail this test.
func TestStrategyBotApplicationDoesNotAskTheContractSideAboutASpotBot(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)
	underTest.expectTheNamedTradingStrategyIsOfKind(vo.MarketDataKindKCandle)
	underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) (entities.StrategyBot, error) {
			assert.Equal(t, string(vo.MarketDataKindKCandle), bot.MarketDataKind)
			assert.True(t, bot.PositionPlanLeverage.IsZero())

			return storedBot(vo.StrategyBotStopped), nil
		})

	botDto, createError := underTest.strategyBotApplication.CreateStrategyBot(
		context.Background(), strategyBotOwnerID, aBotWrite())

	require.NoError(t, createError)
	assert.Equal(t, string(vo.MarketDataKindKCandle), botDto.MarketDataKind)
}

func TestStrategyBotApplicationKeepsABotsKindOnARewrite(t *testing.T) {
	t.Run("a spot bot cannot become a contract bot", func(t *testing.T) {
		underTest := newStrategyBotApplicationUnderTest(t)
		underTest.expectTheNamedTradingStrategyIsOfKind(vo.MarketDataKindContractKCandle)
		underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
			Return(storedBot(vo.StrategyBotStopped), nil)
		underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

		writeDto := aContractBotWrite()
		writeDto.ID = strategyBotID

		_, updateError := underTest.strategyBotApplication.UpdateStrategyBot(
			context.Background(), strategyBotOwnerID, writeDto)

		require.ErrorIs(t, updateError, domains.ErrStrategyBotValidation)
		assert.ErrorContains(t, updateError, "行情種類建立後不得更換")
	})

	t.Run("a contract bot renamed without naming its kind stays a contract bot", func(t *testing.T) {
		underTest := newStrategyBotApplicationUnderTest(t)
		underTest.expectTheNamedTradingStrategyIsOfKind(vo.MarketDataKindContractKCandle)
		underTest.expectTheContract(true, aLadderAllowing(125))
		underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
			Return(storedContractBot(vo.StrategyBotStopped), nil)
		underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, bot entities.StrategyBot) (entities.StrategyBot, error) {
				assert.Equal(t, "新名字", bot.Name)
				assert.Equal(t, string(vo.MarketDataKindContractKCandle), bot.MarketDataKind)

				return storedContractBot(vo.StrategyBotStopped), nil
			})

		writeDto := aContractBotWrite()
		writeDto.ID = strategyBotID
		writeDto.Name = "新名字"
		writeDto.MarketDataKind = ""

		_, updateError := underTest.strategyBotApplication.UpdateStrategyBot(
			context.Background(), strategyBotOwnerID, writeDto)

		require.NoError(t, updateError)
	})
}

func TestStrategyBotApplicationListsOneKindOfBotWhenAskedTo(t *testing.T) {
	storedBots := []entities.StrategyBot{
		storedBot(vo.StrategyBotStopped), storedBot(vo.StrategyBotRunning),
		storedContractBot(vo.StrategyBotStopped),
	}

	testCases := []struct {
		name          string
		kind          string
		expectedKinds []string
	}{
		{name: "only contract bots", kind: "contractKCandle", expectedKinds: []string{"contractKCandle"}},
		{name: "only spot bots", kind: "kCandle", expectedKinds: []string{"kCandle", "kCandle"}},
		{name: "every bot when no kind is named", kind: "",
			expectedKinds: []string{"kCandle", "kCandle", "contractKCandle"}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotApplicationUnderTest(t)
			underTest.strategyBotRepository.EXPECT().FindAllByOwner(gomock.Any(), strategyBotOwnerID).
				Return(storedBots, nil)

			botDtos, listError := underTest.strategyBotApplication.ListStrategyBots(
				context.Background(), strategyBotOwnerID, testCase.kind)

			require.NoError(t, listError)
			listedKinds := make([]string, 0, len(botDtos))
			for _, botDto := range botDtos {
				listedKinds = append(listedKinds, botDto.MarketDataKind)
			}
			assert.Equal(t, testCase.expectedKinds, listedKinds)
		})
	}
}

// Naming a kind nobody recognises is refused rather than answered with nothing, which
// would read as "you have none".
func TestStrategyBotApplicationRefusesToListAKindNobodyRecognises(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)

	_, listError := underTest.strategyBotApplication.ListStrategyBots(
		context.Background(), strategyBotOwnerID, "期貨")

	require.ErrorIs(t, listError, domains.ErrStrategyBotValidation)
}

// The running limit is this person's, not one per line: spot and contract bots count
// together.
func TestStrategyBotApplicationCountsContractBotsTowardsTheRunningLimit(t *testing.T) {
	testCases := []struct {
		name           string
		alreadyRunning int
		expectsStart   bool
	}{
		{name: "the tenth starts", alreadyRunning: 9, expectsStart: true},
		{name: "the eleventh is refused", alreadyRunning: 10, expectsStart: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotApplicationUnderTest(t)
			underTest.telegramDeliveryRepository.EXPECT().
				FindOneByUser(gomock.Any(), strategyBotOwnerID).
				Return(entities.TelegramDelivery{
					UserID: strategyBotOwnerID, SealedBotToken: "sealed", ChatID: "987654",
				}, nil).AnyTimes()
			underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).
				Return(storedContractBot(vo.StrategyBotStopped), nil).AnyTimes()
			underTest.strategyBotRepository.EXPECT().CountRunningByOwner(gomock.Any(), strategyBotOwnerID).
				Return(testCase.alreadyRunning, nil)
			if testCase.expectsStart {
				underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)
			}

			_, startError := underTest.strategyBotApplication.StartStrategyBot(
				context.Background(), strategyBotOwnerID, strategyBotID)

			if testCase.expectsStart {
				require.NoError(t, startError)
				// What it says about itself names the perpetual contract.
				require.Len(t, underTest.announced(), 1)
				assert.Contains(t, underTest.announced()[0], "【已啟動】合約突破 · BTCUSDT 永續合約")

				return
			}

			assert.ErrorIs(t, startError, domains.ErrStrategyBotRunningLimitReached)
		})
	}
}

// A contract bot being saved whose contract side cannot be read is not saved, and the
// failure is reported rather than read as a refusal.
func TestStrategyBotApplicationReportsAContractSideItCouldNotRead(t *testing.T) {
	testCases := []struct {
		name    string
		arrange func(underTest strategyBotApplicationUnderTest)
	}{
		{
			name: "the contract's entry",
			arrange: func(underTest strategyBotApplicationUnderTest) {
				underTest.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
					Return(entities.ContractTradingSymbol{}, false, errors.New("the database went away"))
			},
		},
		{
			name: "the contract's ladder",
			arrange: func(underTest strategyBotApplicationUnderTest) {
				underTest.contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
					Return(entities.ContractTradingSymbol{IsWatched: true}, true, nil)
				underTest.contractMaintenanceMarginTierRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
					Return(nil, errors.New("the database went away"))
			},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotApplicationUnderTest(t)
			underTest.expectTheNamedTradingStrategyIsOfKind(vo.MarketDataKindContractKCandle)
			testCase.arrange(underTest)
			underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

			_, createError := underTest.strategyBotApplication.CreateStrategyBot(
				context.Background(), strategyBotOwnerID, aContractBotWrite())

			assert.EqualError(t, createError, "the database went away")
		})
	}
}

// A stored kind nobody recognises is not quietly rewritten into one.
func TestStrategyBotApplicationRefusesToRewriteABotOfAnUnreadableKind(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)
	underTest.expectTheNamedTradingStrategyIsOfKind(vo.MarketDataKindKCandle)
	unreadable := storedBot(vo.StrategyBotStopped)
	unreadable.MarketDataKind = "option"
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(unreadable, nil)
	underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

	writeDto := aBotWrite()
	writeDto.ID = strategyBotID

	_, updateError := underTest.strategyBotApplication.UpdateStrategyBot(
		context.Background(), strategyBotOwnerID, writeDto)

	assert.ErrorIs(t, updateError, domains.ErrStrategyBotValidation)
}
