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

func facing(directions ...string) *[]string {
	return &directions
}

// aStoredContractBotFacing is a stopped contract bot already holding this direction.
func aStoredContractBotFacing(mode vo.ContractTradingModeVo) entities.StrategyBot {
	bot := storedContractBot(vo.StrategyBotStopped)
	bot.ContractTradingMode = string(mode)

	return bot
}

// expectsTheRewrite answers the stored bot and hands back whatever is saved.
func (underTest strategyBotApplicationUnderTest) expectsTheRewrite(stored entities.StrategyBot) *entities.StrategyBot {
	saved := &entities.StrategyBot{}
	underTest.expectTheNamedTradingStrategyIsOfKind(vo.MarketDataKindVo(stored.MarketDataKind))
	if stored.MarketDataKind == string(vo.MarketDataKindContractKCandle) {
		underTest.expectTheContract(true, aLadderAllowing(125))
	}
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(stored, nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, bot entities.StrategyBot) (entities.StrategyBot, error) {
			*saved = bot
			return bot, nil
		}).MaxTimes(1)

	return saved
}

func aContractBotRewrite() dto.StrategyBotWriteDto {
	writeDto := aContractBotWrite()
	writeDto.ID = strategyBotID
	writeDto.DeclaredLeverage = decimal.NewFromInt(5)

	return writeDto
}

func TestStrategyBotApplicationCreatesABotWithItsDirectionAndNotifications(t *testing.T) {
	off, on := false, true
	testCases := []struct {
		name                  string
		writeDto              func() dto.StrategyBotWriteDto
		contract              bool
		expectedDirections    []string
		expectedNotifications dto.StrategyBotNotificationsDto
	}{
		{name: "a contract bot facing long",
			writeDto: func() dto.StrategyBotWriteDto {
				writeDto := aContractBotWrite()
				writeDto.DeclaredLeverage = decimal.NewFromInt(5)
				writeDto.TradingDirections = facing("long")
				return writeDto
			}, contract: true, expectedDirections: []string{"long"},
			expectedNotifications: dto.StrategyBotNotificationsDto{Buy: true, Sell: true, Hold: false}},
		{name: "a contract bot saying nothing faces both",
			writeDto: func() dto.StrategyBotWriteDto {
				writeDto := aContractBotWrite()
				writeDto.DeclaredLeverage = decimal.NewFromInt(5)
				return writeDto
			}, contract: true, expectedDirections: []string{"long", "short"},
			expectedNotifications: dto.StrategyBotNotificationsDto{Buy: true, Sell: true, Hold: false}},
		{name: "a spot bot choosing its notifications",
			writeDto: func() dto.StrategyBotWriteDto {
				writeDto := aBotWrite()
				writeDto.Notifications = dto.StrategyBotNotificationsWriteDto{Buy: &off, Hold: &on}
				return writeDto
			}, contract: false, expectedDirections: nil,
			expectedNotifications: dto.StrategyBotNotificationsDto{Buy: false, Sell: true, Hold: true}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotApplicationUnderTest(t)
			if testCase.contract {
				underTest.expectTheNamedTradingStrategyIsOfKind(vo.MarketDataKindContractKCandle)
				underTest.expectTheContract(true, aLadderAllowing(125))
			} else {
				underTest.expectTheNamedTradingStrategyIsThisPersons()
			}
			underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
				DoAndReturn(func(_ context.Context, bot entities.StrategyBot) (entities.StrategyBot, error) {
					return bot, nil
				})

			botDto, createError := underTest.strategyBotApplication.CreateStrategyBot(
				context.Background(), strategyBotOwnerID, testCase.writeDto())

			require.NoError(t, createError)
			assert.Equal(t, testCase.expectedDirections, botDto.TradingDirections)
			assert.Equal(t, testCase.expectedNotifications, botDto.Notifications)
		})
	}
}

func TestStrategyBotApplicationRefusesADirectionItCannotSettle(t *testing.T) {
	testCases := []struct {
		name            string
		writeDto        func() dto.StrategyBotWriteDto
		contract        bool
		expectedMessage string
	}{
		{name: "a contract bot facing nothing", contract: true,
			writeDto: func() dto.StrategyBotWriteDto {
				writeDto := aContractBotWrite()
				writeDto.TradingDirections = facing()
				return writeDto
			}, expectedMessage: "至少要選一個交易方向"},
		{name: "a spot bot given a direction", contract: false,
			writeDto: func() dto.StrategyBotWriteDto {
				writeDto := aBotWrite()
				writeDto.TradingDirections = facing("long")
				return writeDto
			}, expectedMessage: "現貨機器人沒有交易方向"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotApplicationUnderTest(t)
			if testCase.contract {
				underTest.expectTheNamedTradingStrategyIsOfKind(vo.MarketDataKindContractKCandle)
			} else {
				underTest.expectTheNamedTradingStrategyIsThisPersons()
			}
			underTest.strategyBotRepository.EXPECT().Save(gomock.Any(), gomock.Any()).Times(0)

			_, createError := underTest.strategyBotApplication.CreateStrategyBot(
				context.Background(), strategyBotOwnerID, testCase.writeDto())

			require.ErrorIs(t, createError, domains.ErrStrategyBotValidation)
			assert.ErrorContains(t, createError, testCase.expectedMessage)
		})
	}
}

func TestStrategyBotApplicationTurnsABotAroundOnlyWhenItIsSafe(t *testing.T) {
	testCases := []struct {
		name               string
		stored             func() entities.StrategyBot
		unsettledSymbol    string
		declaredDirections *[]string
		expectedError      error
		expectedMessage    string
		expectedMode       string
	}{
		{name: "stopped, auto orders off and flat: turned around",
			stored:             func() entities.StrategyBot { return aStoredContractBotFacing(vo.ContractTradingModeLongOnly) },
			declaredDirections: facing("short"), expectedMode: "shortOnly"},
		{name: "auto orders on: refused",
			stored: func() entities.StrategyBot {
				bot := aStoredContractBotFacing(vo.ContractTradingModeLongOnly)
				bot.AutoOrderEnabled = true
				return bot
			},
			declaredDirections: facing("short"),
			expectedError:      domains.ErrStrategyBotTradingDirectionAutoOrderOn, expectedMessage: "請先到網頁上這台機器人的詳細頁關掉自動下單"},
		{name: "a watched symbol holding a position: refused",
			stored: func() entities.StrategyBot {
				bot := aStoredContractBotFacing(vo.ContractTradingModeLongOnly)
				bot.WatchedSymbols[0].AutoOrderPositionDirection = "long"
				return bot
			},
			declaredDirections: facing("short"),
			expectedError:      domains.ErrStrategyBotWatchedSymbolHoldsPosition, expectedMessage: "BTCUSDT 還有機器人持倉"},
		{name: "a watched symbol with an auto order not yet done: refused",
			stored:             func() entities.StrategyBot { return aStoredContractBotFacing(vo.ContractTradingModeLongOnly) },
			unsettledSymbol:    "BTCUSDT",
			declaredDirections: facing("short"),
			expectedError:      domains.ErrStrategyBotWatchedSymbolAutoOrderOutstanding, expectedMessage: "BTCUSDT 還有沒做完的自動下單"},
		{name: "auto orders on, direction unchanged: saved",
			stored: func() entities.StrategyBot {
				bot := aStoredContractBotFacing(vo.ContractTradingModeLongOnly)
				bot.AutoOrderEnabled = true
				bot.WatchedSymbols[0].AutoOrderPositionDirection = "long"
				return bot
			},
			declaredDirections: facing("long"), expectedMode: "longOnly"},
		{name: "auto orders on, direction left out: kept and saved",
			stored: func() entities.StrategyBot {
				bot := aStoredContractBotFacing(vo.ContractTradingModeShortOnly)
				bot.AutoOrderEnabled = true
				return bot
			},
			declaredDirections: nil, expectedMode: "shortOnly"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotApplicationUnderTest(t)
			if testCase.unsettledSymbol != "" {
				underTest.unsettledAutoOrderSymbols[testCase.unsettledSymbol] = true
			}
			saved := underTest.expectsTheRewrite(testCase.stored())
			writeDto := aContractBotRewrite()
			writeDto.TradingDirections = testCase.declaredDirections

			_, updateError := underTest.strategyBotApplication.UpdateStrategyBot(
				context.Background(), strategyBotOwnerID, writeDto)

			if testCase.expectedError != nil {
				require.ErrorIs(t, updateError, testCase.expectedError)
				assert.ErrorContains(t, updateError, testCase.expectedMessage)
				assert.Zero(t, saved.ID, "nothing was saved")
				return
			}
			require.NoError(t, updateError)
			assert.Equal(t, testCase.expectedMode, saved.ContractTradingMode)
		})
	}
}

func TestStrategyBotApplicationKeepsTheNotificationsARewriteLeavesOut(t *testing.T) {
	on := true
	underTest := newStrategyBotApplicationUnderTest(t)
	stored := storedBot(vo.StrategyBotStopped)
	stored.MutesSellNotification = true
	saved := underTest.expectsTheRewrite(stored)
	writeDto := aBotWrite()
	writeDto.ID = strategyBotID
	writeDto.Notifications = dto.StrategyBotNotificationsWriteDto{Hold: &on}

	_, updateError := underTest.strategyBotApplication.UpdateStrategyBot(
		context.Background(), strategyBotOwnerID, writeDto)

	require.NoError(t, updateError)
	assert.Equal(t, dto.StrategyBotNotificationsDto{Buy: true, Sell: false, Hold: true}, saved.NotificationsDto())
}

func TestStrategyBotApplicationReportsAnAutoOrderItCouldNotReadWhenTurningABotAround(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)
	readFailure := errors.New("storage unreachable")
	*underTest.autoOrderReadFailure = readFailure
	saved := underTest.expectsTheRewrite(aStoredContractBotFacing(vo.ContractTradingModeLongOnly))
	writeDto := aContractBotRewrite()
	writeDto.TradingDirections = facing("short")

	_, updateError := underTest.strategyBotApplication.UpdateStrategyBot(
		context.Background(), strategyBotOwnerID, writeDto)

	require.ErrorIs(t, updateError, readFailure)
	assert.Zero(t, saved.ID, "nothing was saved")
}

func TestStrategyBotApplicationRefusesToTurnOrSilenceARunningBot(t *testing.T) {
	off := false
	testCases := []struct {
		name   string
		change func(writeDto *dto.StrategyBotWriteDto)
	}{
		{name: "turning it around", change: func(writeDto *dto.StrategyBotWriteDto) {
			writeDto.TradingDirections = facing("short")
		}},
		{name: "silencing its buys", change: func(writeDto *dto.StrategyBotWriteDto) {
			writeDto.Notifications.Buy = &off
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newStrategyBotApplicationUnderTest(t)
			running := aStoredContractBotFacing(vo.ContractTradingModeLongOnly)
			running.RunState = string(vo.StrategyBotRunning)
			underTest.tradingStrategyRepository.EXPECT().FindOne(gomock.Any(), botsTradingStrategyID).
				Return(entities.TradingStrategy{
					ID: botsTradingStrategyID, OwnerID: strategyBotOwnerID, MarketDataKind: "contractKCandle",
				}, nil).AnyTimes()
			underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(running, nil).AnyTimes()
			// No Save is stubbed: the refusal lands before anything is written.
			writeDto := aContractBotRewrite()
			testCase.change(&writeDto)

			_, updateError := underTest.strategyBotApplication.UpdateStrategyBot(
				context.Background(), strategyBotOwnerID, writeDto)

			require.ErrorIs(t, updateError, domains.ErrStrategyBotRunning)
		})
	}
}

// Auto orders are asked about first: switching them off is the one fix that must come before anything else.
func TestStrategyBotApplicationNamesAutoOrdersBeforePositionsWhenTurningABotAround(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)
	stored := aStoredContractBotFacing(vo.ContractTradingModeLongOnly)
	stored.AutoOrderEnabled = true
	stored.WatchedSymbols[0].AutoOrderPositionDirection = "long"
	underTest.expectsTheRewrite(stored)
	writeDto := aContractBotRewrite()
	writeDto.TradingDirections = facing("short")

	_, updateError := underTest.strategyBotApplication.UpdateStrategyBot(
		context.Background(), strategyBotOwnerID, writeDto)

	require.ErrorIs(t, updateError, domains.ErrStrategyBotTradingDirectionAutoOrderOn)
}

func TestStrategyBotApplicationStartsABotThatAnnouncesNoConclusionStillSayingItStarted(t *testing.T) {
	underTest := newStrategyBotApplicationUnderTest(t)
	silent := storedBot(vo.StrategyBotStopped)
	silent.MutesBuyNotification = true
	silent.MutesSellNotification = true
	underTest.telegramDeliveryRepository.EXPECT().
		FindOneByUser(gomock.Any(), strategyBotOwnerID).
		Return(entities.TelegramDelivery{UserID: strategyBotOwnerID}, nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().FindOne(gomock.Any(), strategyBotID).Return(silent, nil).AnyTimes()
	underTest.strategyBotRepository.EXPECT().CountRunningByOwner(gomock.Any(), strategyBotOwnerID).Return(0, nil)
	underTest.strategyBotRepository.EXPECT().UpdateRunState(gomock.Any(), gomock.Any()).Return(nil)

	_, startError := underTest.strategyBotApplication.StartStrategyBot(
		context.Background(), strategyBotOwnerID, strategyBotID)

	require.NoError(t, startError)
	require.Len(t, underTest.announced(), 1)
	assert.Contains(t, underTest.announced()[0], "已啟動")
}
