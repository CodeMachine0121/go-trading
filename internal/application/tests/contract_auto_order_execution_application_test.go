package application_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const (
	autoOrderID        = uint(41)
	autoOrderBotID     = uint(3)
	autoOrderOwnerID   = uint(7)
	autoOrderReplica   = "replica-a"
	autoOrderBotName   = "合約突破"
	openClientOrderID  = "gt-ao-41-open"
	closeClientOrderID = "gt-ao-41-close"
	stopLossClientID   = "gt-ao-41-sl"
	takeProfitClientID = "gt-ao-41-tp"
	// earlierStopLossID and earlierTakeProfitID guard a position an earlier order opened.
	earlierStopLossID   = "gt-ao-30-sl"
	earlierTakeProfitID = "gt-ao-30-tp"
)

// autoOrderRoundRanAt is when the order's round ran; the deadline is two minutes later.
var autoOrderRoundRanAt = time.Date(2026, 10, 4, 6, 30, 0, 0, time.UTC)

var theCredential = vo.TradingKeyCredentialVo{ApiKey: "the-api-key", SecretKey: "the-secret-key"}

type autoOrderExecutionUnderTest struct {
	executionApplication  *application.ContractAutoOrderExecutionApplication
	contractOrderProxy    *mocks.MockIContractOrderProxy
	verificationProxy     *mocks.MockITradingKeyVerificationProxy
	strategyBotRepository *mocks.MockIStrategyBotRepository
	// now is what the clock answers; a test moves it.
	now *time.Time
	// claimed is what the claim answers; claims counts how often it was asked.
	claimed *bool
	claims  *int
	bot     *entities.StrategyBot
	queue   *[]entities.ContractAutoOrder
	// saved, positions, messages and switchedOff are every write, in order.
	saved       *[]entities.ContractAutoOrder
	positions   *[]vo.AutoOrderPositionVo
	messages    *[]entities.PendingMessage
	switchedOff *[][]string
	// failures makes a storage read or write fail, or the claim be lost, at that point.
	failures *autoOrderStorageFailures
	// key is the owner's stored trading key; nil means none is stored.
	key *entities.BinanceTradingKey
	// unsealFailure makes the stored key impossible to open.
	unsealFailure *error
}

type autoOrderStorageFailures struct {
	candidates error
	findBot    error
	findKey    error
	saveError  error
	claimLost  bool
	position   error
	enqueue    error
	switchOff  error
}

func newAutoOrderExecutionUnderTest(t *testing.T) autoOrderExecutionUnderTest {
	controller := gomock.NewController(t)

	now := autoOrderRoundRanAt.Add(10 * time.Second)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().DoAndReturn(func() time.Time { return now }).AnyTimes()

	queue := []entities.ContractAutoOrder{}
	claimed := true
	claims := 0
	saved := []entities.ContractAutoOrder{}
	contractAutoOrderRepository := mocks.NewMockIContractAutoOrderRepository(controller)
	failures := autoOrderStorageFailures{}
	contractAutoOrderRepository.EXPECT().FindDispatchCandidates(gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, int) ([]entities.ContractAutoOrder, error) {
			return queue, failures.candidates
		}).AnyTimes()
	contractAutoOrderRepository.EXPECT().Claim(gomock.Any(), autoOrderID, autoOrderReplica, gomock.Any(), gomock.Any()).
		DoAndReturn(func(context.Context, uint, string, time.Time, time.Time) (bool, error) {
			claims++

			return claimed, nil
		}).AnyTimes()
	contractAutoOrderRepository.EXPECT().SaveProgress(gomock.Any(), gomock.Any(), autoOrderReplica).
		DoAndReturn(func(_ context.Context, order entities.ContractAutoOrder, _ string) (bool, error) {
			if failures.saveError != nil {
				return false, failures.saveError
			}
			if failures.claimLost {
				return false, nil
			}
			saved = append(saved, order)

			return true, nil
		}).AnyTimes()

	bot := entities.StrategyBot{
		ID: autoOrderBotID, OwnerID: autoOrderOwnerID, Name: autoOrderBotName, Symbol: "BTCUSDT",
		MarketDataKind: string(vo.MarketDataKindContractKCandle), RunState: string(vo.StrategyBotRunning),
		AutoOrderEnabled: true,
	}
	positions := []vo.AutoOrderPositionVo{}
	switchedOff := [][]string{}
	strategyBotRepository := mocks.NewMockIStrategyBotRepository(controller)
	strategyBotRepository.EXPECT().FindOne(gomock.Any(), autoOrderBotID).
		DoAndReturn(func(context.Context, uint) (entities.StrategyBot, error) { return bot, failures.findBot }).AnyTimes()
	strategyBotRepository.EXPECT().UpdateAutoOrderPosition(gomock.Any(), autoOrderBotID, gomock.Any()).
		DoAndReturn(func(executionContext context.Context, _ uint, position vo.AutoOrderPositionVo) error {
			assert.True(t, isInsideTransaction(executionContext), "the bot's position is written outside the step's transaction")
			if failures.position != nil {
				return failures.position
			}
			positions = append(positions, position)

			return nil
		}).AnyTimes()
	strategyBotRepository.EXPECT().DisableAutoOrderByOwner(gomock.Any(), autoOrderOwnerID, gomock.Any()).
		DoAndReturn(func(executionContext context.Context, _ uint, kinds []string) error {
			assert.True(t, isInsideTransaction(executionContext), "auto order is switched off outside the settling transaction")
			if failures.switchOff != nil {
				return failures.switchOff
			}
			switchedOff = append(switchedOff, kinds)

			return nil
		}).AnyTimes()

	key := &entities.BinanceTradingKey{
		UserID: autoOrderOwnerID, SealedApiKey: "sealed-api", SealedSecretKey: "sealed-secret",
		ContractTradingEnabled: true,
	}
	binanceTradingKeyRepository := mocks.NewMockIBinanceTradingKeyRepository(controller)
	binanceTradingKeyRepository.EXPECT().FindOneByUser(gomock.Any(), autoOrderOwnerID).
		DoAndReturn(func(context.Context, uint) (entities.BinanceTradingKey, error) {
			if failures.findKey != nil {
				return entities.BinanceTradingKey{}, failures.findKey
			}
			if key == nil {
				return entities.BinanceTradingKey{}, domains.ErrBinanceTradingKeyNotConfigured
			}

			return *key, nil
		}).AnyTimes()
	unsealFailure := error(nil)
	secretSealProxy := mocks.NewMockISecretSealProxy(controller)
	secretSealProxy.EXPECT().Unseal("sealed-api").DoAndReturn(func(string) (string, error) {
		return "the-api-key", unsealFailure
	}).AnyTimes()
	secretSealProxy.EXPECT().Unseal("sealed-secret").Return("the-secret-key", nil).AnyTimes()

	tickSize := decimal.NewNullDecimal(decimal.RequireFromString("0.1"))
	contractTradingSymbolRepository := mocks.NewMockIContractTradingSymbolRepository(controller)
	contractTradingSymbolRepository.EXPECT().FindBySymbol(gomock.Any(), "BTCUSDT").
		Return(entities.ContractTradingSymbol{Symbol: "BTCUSDT", TickSize: tickSize}, true, nil).AnyTimes()

	messages := []entities.PendingMessage{}
	pendingMessageRepository := mocks.NewMockIPendingMessageRepository(controller)
	pendingMessageRepository.EXPECT().Enqueue(gomock.Any(), gomock.Any()).
		DoAndReturn(func(executionContext context.Context, message entities.PendingMessage) error {
			assert.True(t, isInsideTransaction(executionContext), "the result is queued outside the settling transaction")
			if failures.enqueue != nil {
				return failures.enqueue
			}
			messages = append(messages, message)

			return nil
		}).AnyTimes()

	transactionRepository := mocks.NewMockITransactionRepository(controller)
	transactionRepository.EXPECT().Atomically(gomock.Any(), gomock.Any()).
		DoAndReturn(func(executionContext context.Context, work func(context.Context) error) error {
			return work(context.WithValue(executionContext, insideTransaction{}, true))
		}).AnyTimes()

	contractOrderProxy := mocks.NewMockIContractOrderProxy(controller)
	verificationProxy := mocks.NewMockITradingKeyVerificationProxy(controller)

	return autoOrderExecutionUnderTest{
		executionApplication: application.NewContractAutoOrderExecutionApplication(service.NewContractAutoOrderService(
			contractAutoOrderRepository, strategyBotRepository, binanceTradingKeyRepository,
			contractTradingSymbolRepository, pendingMessageRepository, transactionRepository, secretSealProxy,
			contractOrderProxy, verificationProxy, clockProxy, autoOrderReplica, 2*time.Minute, 4)),
		contractOrderProxy:    contractOrderProxy,
		verificationProxy:     verificationProxy,
		strategyBotRepository: strategyBotRepository,
		now:                   &now,
		claimed:               &claimed,
		claims:                &claims,
		bot:                   &bot,
		queue:                 &queue,
		saved:                 &saved,
		positions:             &positions,
		messages:              &messages,
		switchedOff:           &switchedOff,
		failures:              &failures,
		key:                   key,
		unsealFailure:         &unsealFailure,
	}
}

// anAutoOrder is queued by a round that ran at autoOrderRoundRanAt; an empty openQuantity opens nothing.
func anAutoOrder(target string, openQuantity string, stopLoss string, takeProfit string) entities.ContractAutoOrder {
	order := entities.ContractAutoOrder{
		ID: autoOrderID, StrategyBotID: autoOrderBotID, OwnerUserID: autoOrderOwnerID, Symbol: "BTCUSDT",
		RoundDueAt: autoOrderRoundRanAt, RunNumber: 52, TargetPosition: target,
		Leverage: decimal.NewFromInt(3), StopLossPercentage: decimal.RequireFromString(stopLoss),
		TakeProfitPercentage: decimal.RequireFromString(takeProfit),
		Status:               string(vo.ContractAutoOrderReady), NextAttemptAt: autoOrderRoundRanAt,
		ExpiresAt: autoOrderRoundRanAt.Add(2 * time.Minute), CreatedAt: autoOrderRoundRanAt,
	}
	if openQuantity != "" {
		order.OpenQuantity = decimal.NewNullDecimal(decimal.RequireFromString(openQuantity))
	}

	return order
}

func (underTest autoOrderExecutionUnderTest) queues(order entities.ContractAutoOrder) {
	*underTest.queue = append(*underTest.queue, order)
}

// botHolds is the bot's own position before the order runs.
func (underTest autoOrderExecutionUnderTest) botHolds(direction string, quantity string, stopLossID string, takeProfitID string) {
	underTest.bot.AutoOrderPositionDirection = direction
	underTest.bot.AutoOrderPositionQuantity = decimal.RequireFromString(quantity)
	underTest.bot.AutoOrderStopLossClientID = stopLossID
	underTest.bot.AutoOrderTakeProfitClientID = takeProfitID
}

func (underTest autoOrderExecutionUnderTest) execute(t *testing.T) {
	_, executeError := underTest.executionApplication.ExecuteDueAutoOrders(t.Context())

	require.NoError(t, executeError)
}

func (underTest autoOrderExecutionUnderTest) lastSaved(t *testing.T) entities.ContractAutoOrder {
	require.NotEmpty(t, *underTest.saved)

	return (*underTest.saved)[len(*underTest.saved)-1]
}

func (underTest autoOrderExecutionUnderTest) lastPosition(t *testing.T) vo.AutoOrderPositionVo {
	require.NotEmpty(t, *underTest.positions)

	return (*underTest.positions)[len(*underTest.positions)-1]
}

func (underTest autoOrderExecutionUnderTest) onlyMessage(t *testing.T) entities.PendingMessage {
	require.Len(t, *underTest.messages, 1)

	return (*underTest.messages)[0]
}

func (underTest autoOrderExecutionUnderTest) venueHasNo(clientOrderID string) {
	underTest.contractOrderProxy.EXPECT().FindMarketOrder(gomock.Any(), theCredential, "BTCUSDT", clientOrderID).
		Return(vo.ContractOrderFillVo{}, vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureNotFound})
}

func (underTest autoOrderExecutionUnderTest) accountIsOneWay() {
	underTest.contractOrderProxy.EXPECT().ReadPositionMode(gomock.Any(), theCredential).
		Return(false, vo.ContractOrderCallVo{})
}

func (underTest autoOrderExecutionUnderTest) leverageIsSet() {
	underTest.contractOrderProxy.EXPECT().PrepareIsolatedLeverage(gomock.Any(), theCredential, "BTCUSDT", 3).
		Return(vo.ContractOrderCallVo{})
}

// marketOrderFills checks the order sent and answers that it filled this much at this price.
func (underTest autoOrderExecutionUnderTest) marketOrderFills(
	t *testing.T, side vo.ContractOrderSideVo, quantity string, reduceOnly bool, clientOrderID string,
	filledQuantity string, averagePrice string,
) {
	underTest.contractOrderProxy.EXPECT().PlaceMarketOrder(gomock.Any(), theCredential, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ vo.TradingKeyCredentialVo, order vo.ContractMarketOrderVo) (vo.ContractOrderFillVo, vo.ContractOrderCallVo) {
			assert.Equal(t, "BTCUSDT", order.Symbol)
			assert.Equal(t, side, order.Side)
			assert.True(t, order.Quantity.Equal(decimal.RequireFromString(quantity)), "數量 %s", order.Quantity)
			assert.Equal(t, reduceOnly, order.ReduceOnly)
			assert.Equal(t, clientOrderID, order.ClientOrderID)

			return vo.ContractOrderFillVo{
				ExecutedQuantity: decimal.RequireFromString(filledQuantity),
				AveragePrice:     decimal.RequireFromString(averagePrice),
			}, vo.ContractOrderCallVo{}
		})
}

func (underTest autoOrderExecutionUnderTest) marketOrderAnswers(call vo.ContractOrderCallVo) {
	underTest.contractOrderProxy.EXPECT().PlaceMarketOrder(gomock.Any(), theCredential, gomock.Any()).
		Return(vo.ContractOrderFillVo{}, call)
}

// protectiveOrderAnswers checks the protective order sent and gives the venue's answer.
func (underTest autoOrderExecutionUnderTest) protectiveOrderAnswers(
	t *testing.T, kind vo.ContractProtectiveOrderKindVo, side vo.ContractOrderSideVo, triggerPrice string,
	quantity string, clientOrderID string, call vo.ContractOrderCallVo,
) {
	underTest.contractOrderProxy.EXPECT().FindProtectiveOrder(gomock.Any(), theCredential, "BTCUSDT", clientOrderID).
		Return(vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureNotFound})
	underTest.contractOrderProxy.EXPECT().PlaceProtectiveOrder(gomock.Any(), theCredential, gomock.Any()).
		DoAndReturn(func(_ context.Context, _ vo.TradingKeyCredentialVo, order vo.ContractProtectiveOrderVo) vo.ContractOrderCallVo {
			assert.Equal(t, kind, order.Kind)
			assert.Equal(t, side, order.Side)
			assert.True(t, order.TriggerPrice.Equal(decimal.RequireFromString(triggerPrice)), "觸發價 %s", order.TriggerPrice)
			assert.True(t, order.Quantity.Equal(decimal.RequireFromString(quantity)))
			assert.Equal(t, clientOrderID, order.ClientOrderID)

			return call
		})
}

func (underTest autoOrderExecutionUnderTest) protectiveOrdersAreTakenDown(clientOrderIDs ...string) {
	for _, clientOrderID := range clientOrderIDs {
		underTest.contractOrderProxy.EXPECT().CancelProtectiveOrder(gomock.Any(), theCredential, "BTCUSDT", clientOrderID).
			Return(vo.ContractOrderCallVo{})
	}
}

func (underTest autoOrderExecutionUnderTest) venueHolds(longQuantity string, shortQuantity string) {
	underTest.contractOrderProxy.EXPECT().ReadPosition(gomock.Any(), theCredential, "BTCUSDT").
		Return(vo.ContractExchangePositionVo{
			LongQuantity: decimal.RequireFromString(longQuantity), ShortQuantity: decimal.RequireFromString(shortQuantity),
		}, vo.ContractOrderCallVo{})
}

func assertFigure(t *testing.T, expected string, figure decimal.NullDecimal) {
	t.Helper()
	require.True(t, figure.Valid, "expected %s, got nothing", expected)
	assert.True(t, figure.Decimal.Equal(decimal.RequireFromString(expected)), "expected %s, got %s", expected, figure.Decimal)
}

func TestAnAutoOrderOpensALongAndGuardsItFromTheFill(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "1.5", "3"))
	underTest.venueHasNo(openClientOrderID)
	underTest.accountIsOneWay()
	underTest.leverageIsSet()
	underTest.marketOrderFills(t, vo.ContractOrderSideBuy, "0.002", false, openClientOrderID, "0.002", "85000")
	underTest.protectiveOrderAnswers(t, vo.ContractProtectiveOrderStopLoss, vo.ContractOrderSideSell,
		"83725.0", "0.002", stopLossClientID, vo.ContractOrderCallVo{})
	underTest.protectiveOrderAnswers(t, vo.ContractProtectiveOrderTakeProfit, vo.ContractOrderSideSell,
		"87550.0", "0.002", takeProfitClientID, vo.ContractOrderCallVo{})

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderFilled), settled.Outcome)
	assert.Equal(t, string(vo.ContractAutoOrderSettled), settled.Status)
	assertFigure(t, "83725", settled.StopLossPrice)
	assertFigure(t, "87550", settled.TakeProfitPrice)
	assert.False(t, settled.ProtectionMissing)
	position := underTest.lastPosition(t)
	assert.Equal(t, vo.TargetPositionLong, position.Direction)
	assert.True(t, position.Quantity.Equal(decimal.RequireFromString("0.002")))
	assert.Equal(t, stopLossClientID, position.StopLossClientID)
	assert.Equal(t, takeProfitClientID, position.TakeProfitClientID)

	message := underTest.onlyMessage(t)
	assert.Equal(t, string(vo.PendingMessageAutoOrder), message.Kind)
	assert.Equal(t, autoOrderOwnerID, message.RecipientUserID)
	assert.Nil(t, message.RoundDueAt)
	assert.Contains(t, message.Text, "做多")
	assert.Contains(t, message.Text, "0.002 @ 85000")
	assert.Contains(t, message.Text, "止損 83725")
	assert.Contains(t, message.Text, "止盈 87550")
}

func TestAnAutoOrderWhoseSendGotNoClearAnswerWaitsToAskAgain(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "1.5", "3"))
	underTest.venueHasNo(openClientOrderID)
	underTest.accountIsOneWay()
	underTest.leverageIsSet()
	underTest.marketOrderAnswers(vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureUncertain})

	underTest.execute(t)

	waiting := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderReady), waiting.Status)
	assert.Equal(t, 1, waiting.AttemptCount)
	assert.Equal(t, autoOrderRoundRanAt.Add(15*time.Second), waiting.NextAttemptAt)
	assert.Empty(t, waiting.Outcome)
	assert.Empty(t, *underTest.positions)
	assert.Empty(t, *underTest.messages)
}

func TestAnAutoOrderTheVenueAlreadyFilledIsNotSentAgain(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	retried := anAutoOrder("long", "0.002", "0", "0")
	retried.AttemptCount = 1
	underTest.queues(retried)
	underTest.contractOrderProxy.EXPECT().FindMarketOrder(gomock.Any(), theCredential, "BTCUSDT", openClientOrderID).
		Return(vo.ContractOrderFillVo{
			ExecutedQuantity: decimal.RequireFromString("0.002"), AveragePrice: decimal.NewFromInt(85000),
		}, vo.ContractOrderCallVo{})
	// No PlaceMarketOrder expectation: sending it again fails the test.

	underTest.execute(t)

	position := underTest.lastPosition(t)
	assert.Equal(t, vo.TargetPositionLong, position.Direction)
	assert.True(t, position.Quantity.Equal(decimal.RequireFromString("0.002")))
	assert.Equal(t, string(vo.ContractAutoOrderFilled), underTest.lastSaved(t).Outcome)
}

func TestAnAutoOrderTheVenueNeverReceivedIsSentAgainUnderTheSameID(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	retried := anAutoOrder("long", "0.002", "0", "0")
	retried.AttemptCount = 1
	underTest.queues(retried)
	underTest.venueHasNo(openClientOrderID)
	underTest.accountIsOneWay()
	underTest.leverageIsSet()
	underTest.marketOrderFills(t, vo.ContractOrderSideBuy, "0.002", false, openClientOrderID, "0.002", "85000")

	underTest.execute(t)

	assert.Equal(t, string(vo.ContractAutoOrderFilled), underTest.lastSaved(t).Outcome)
}

func TestAnAutoOrderWithNothingToOpenSendsNothing(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	planless := anAutoOrder("long", "", "0", "0")
	planless.NoOpenReason = "沒有部位規劃，不下單"
	underTest.queues(planless)

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderNotPlaced), settled.Outcome)
	assert.Equal(t, "沒有部位規劃，不下單", settled.Reason)
}

func TestAnAutoOrderRefusedForLackOfBalanceKeepsAutoOrderOn(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
	underTest.venueHasNo(openClientOrderID)
	underTest.accountIsOneWay()
	underTest.leverageIsSet()
	underTest.marketOrderAnswers(vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureInsufficientBalance})

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderNotPlaced), settled.Outcome)
	assert.Equal(t, "餘額不足", settled.Reason)
	assert.Empty(t, *underTest.switchedOff)
}

func TestAnAutoOrderClosesOnlyWhatTheBotOpened(t *testing.T) {
	testCases := []struct {
		name             string
		botQuantity      string
		venueLong        string
		expectedQuantity string
	}{
		{name: "使用者自己另有多倉", botQuantity: "0.002", venueLong: "0.012", expectedQuantity: "0.002"},
		{name: "幣安上只剩一部分", botQuantity: "0.01", venueLong: "0.006", expectedQuantity: "0.006"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newAutoOrderExecutionUnderTest(t)
			underTest.queues(anAutoOrder("flat", "", "0", "0"))
			underTest.botHolds("long", testCase.botQuantity, earlierStopLossID, earlierTakeProfitID)
			underTest.venueHasNo(closeClientOrderID)
			underTest.accountIsOneWay()
			underTest.protectiveOrdersAreTakenDown(earlierStopLossID, earlierTakeProfitID)
			underTest.venueHolds(testCase.venueLong, "0")
			underTest.marketOrderFills(t, vo.ContractOrderSideSell, testCase.expectedQuantity, true, closeClientOrderID,
				testCase.expectedQuantity, "84000")

			underTest.execute(t)

			settled := underTest.lastSaved(t)
			assert.Equal(t, string(vo.ContractAutoOrderFilled), settled.Outcome)
			position := underTest.lastPosition(t)
			assert.Empty(t, position.Direction)
			assert.True(t, position.Quantity.IsZero())
			assert.Contains(t, underTest.onlyMessage(t).Text, "平多")
		})
	}
}

func TestAnAutoOrderWithNothingOfItsOwnToCloseSendsNothing(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("flat", "", "0", "0"))

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderNotPlaced), settled.Outcome)
	assert.Equal(t, "沒有自動開出的倉位，不需要平倉", settled.Reason)
}

func TestAnAutoOrderNeverAddsToWhatTheBotAlreadyHolds(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
	underTest.botHolds("long", "0.002", "", "")

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderNotPlaced), settled.Outcome)
	assert.Equal(t, "已經持有多倉，不加倉", settled.Reason)
}

func TestAnAutoOrderOpensAShortWithItsExitsSwapped(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("short", "0.002", "1.5", "3"))
	underTest.venueHasNo(openClientOrderID)
	underTest.accountIsOneWay()
	underTest.leverageIsSet()
	underTest.marketOrderFills(t, vo.ContractOrderSideSell, "0.002", false, openClientOrderID, "0.002", "85000")
	underTest.protectiveOrderAnswers(t, vo.ContractProtectiveOrderStopLoss, vo.ContractOrderSideBuy,
		"86275.0", "0.002", stopLossClientID, vo.ContractOrderCallVo{})
	underTest.protectiveOrderAnswers(t, vo.ContractProtectiveOrderTakeProfit, vo.ContractOrderSideBuy,
		"82450.0", "0.002", takeProfitClientID, vo.ContractOrderCallVo{})

	underTest.execute(t)

	assert.Equal(t, vo.TargetPositionShort, underTest.lastPosition(t).Direction)
	assert.Contains(t, underTest.onlyMessage(t).Text, "做空")
}

func TestAnAutoOrderClosesAShortByBuyingBack(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("flat", "", "0", "0"))
	underTest.botHolds("short", "0.002", earlierStopLossID, earlierTakeProfitID)
	underTest.venueHasNo(closeClientOrderID)
	underTest.accountIsOneWay()
	underTest.protectiveOrdersAreTakenDown(earlierStopLossID, earlierTakeProfitID)
	underTest.venueHolds("0", "0.002")
	underTest.marketOrderFills(t, vo.ContractOrderSideBuy, "0.002", true, closeClientOrderID, "0.002", "84000")

	underTest.execute(t)

	assert.Empty(t, underTest.lastPosition(t).Direction)
	assert.Contains(t, underTest.onlyMessage(t).Text, "平空")
}

func TestAnAutoOrderReversesByClosingThenOpening(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("short", "0.003", "0", "0"))
	underTest.botHolds("long", "0.002", "", "")
	underTest.venueHasNo(closeClientOrderID)
	underTest.accountIsOneWay()
	underTest.venueHolds("0.002", "0")
	underTest.marketOrderFills(t, vo.ContractOrderSideSell, "0.002", true, closeClientOrderID, "0.002", "84000")
	underTest.venueHasNo(openClientOrderID)
	underTest.leverageIsSet()
	underTest.marketOrderFills(t, vo.ContractOrderSideSell, "0.003", false, openClientOrderID, "0.003", "84000")

	underTest.execute(t)

	position := underTest.lastPosition(t)
	assert.Equal(t, vo.TargetPositionShort, position.Direction)
	assert.True(t, position.Quantity.Equal(decimal.RequireFromString("0.003")))
	assert.Equal(t, string(vo.ContractAutoOrderFilled), underTest.lastSaved(t).Outcome)
	assert.Contains(t, underTest.onlyMessage(t).Text, "反手做空")
}

func TestAReverseWhoseOpenIsRefusedSaysItClosedButDidNotOpen(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("short", "0.003", "0", "0"))
	underTest.botHolds("long", "0.002", "", "")
	underTest.venueHasNo(closeClientOrderID)
	underTest.accountIsOneWay()
	underTest.venueHolds("0.002", "0")
	underTest.marketOrderFills(t, vo.ContractOrderSideSell, "0.002", true, closeClientOrderID, "0.002", "84000")
	underTest.venueHasNo(openClientOrderID)
	underTest.leverageIsSet()
	underTest.marketOrderAnswers(vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureInsufficientBalance})

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderPartiallyDone), settled.Outcome)
	assert.Equal(t, "已平倉，但做空沒開成：餘額不足", settled.Reason)
	assert.Empty(t, underTest.lastPosition(t).Direction)
}

func TestAnAutoOrderFindingThePositionGoneSendsNoClose(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("flat", "", "0", "0"))
	underTest.botHolds("long", "0.002", earlierStopLossID, earlierTakeProfitID)
	underTest.venueHasNo(closeClientOrderID)
	underTest.accountIsOneWay()
	underTest.protectiveOrdersAreTakenDown(earlierStopLossID, earlierTakeProfitID)
	underTest.venueHolds("0", "0")

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderNotPlaced), settled.Outcome)
	assert.Contains(t, settled.Reason, "幣安上已沒有這筆倉位")
	assert.Empty(t, underTest.lastPosition(t).Direction)
}

func TestAStopTheVenueRefusesLeavesThePositionAndSaysSoLoudly(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "1.5", "3"))
	underTest.venueHasNo(openClientOrderID)
	underTest.accountIsOneWay()
	underTest.leverageIsSet()
	underTest.marketOrderFills(t, vo.ContractOrderSideBuy, "0.002", false, openClientOrderID, "0.002", "85000")
	underTest.protectiveOrderAnswers(t, vo.ContractProtectiveOrderStopLoss, vo.ContractOrderSideSell,
		"83725.0", "0.002", stopLossClientID,
		vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureVenueRefused, ExchangeMessage: "Order would immediately trigger."})
	underTest.protectiveOrderAnswers(t, vo.ContractProtectiveOrderTakeProfit, vo.ContractOrderSideSell,
		"87550.0", "0.002", takeProfitClientID, vo.ContractOrderCallVo{})

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderFilled), settled.Outcome)
	assert.True(t, settled.ProtectionMissing)
	assert.Equal(t, vo.TargetPositionLong, underTest.lastPosition(t).Direction)
	assert.True(t, strings.HasPrefix(underTest.onlyMessage(t).Text, "⚠️ 止損沒有掛上，請立刻到幣安自己處理"),
		underTest.onlyMessage(t).Text)
}

func TestAnAutoOrderWithoutExitDistancesPlacesNoProtection(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
	underTest.venueHasNo(openClientOrderID)
	underTest.accountIsOneWay()
	underTest.leverageIsSet()
	underTest.marketOrderFills(t, vo.ContractOrderSideBuy, "0.002", false, openClientOrderID, "0.002", "85000")

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderFilled), settled.Outcome)
	assert.False(t, settled.StopLossPrice.Valid)
	assert.False(t, settled.TakeProfitPrice.Valid)
}

func TestAnAutoOrderTheAccountSettingsRefuseKeepsAutoOrderOn(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
	underTest.venueHasNo(openClientOrderID)
	underTest.accountIsOneWay()
	underTest.contractOrderProxy.EXPECT().PrepareIsolatedLeverage(gomock.Any(), theCredential, "BTCUSDT", 3).
		Return(vo.ContractOrderCallVo{
			Failure: vo.ContractOrderFailureAccountSettingRefused, ExchangeMessage: "Margin type cannot be changed if there exists position.",
		})

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderNotPlaced), settled.Outcome)
	assert.True(t, strings.HasPrefix(settled.Reason, "幣安帳戶設定不符"), settled.Reason)
	assert.Empty(t, *underTest.switchedOff)
}

func TestAnAutoOrderOnAHedgeModeAccountSendsNothing(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
	underTest.venueHasNo(openClientOrderID)
	underTest.contractOrderProxy.EXPECT().ReadPositionMode(gomock.Any(), theCredential).
		Return(true, vo.ContractOrderCallVo{})

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderNotPlaced), settled.Outcome)
	assert.Equal(t, "請先在幣安把持倉模式改成單向持倉", settled.Reason)
}

func TestAnAutoOrderAnotherReplicaTookIsLeftAlone(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
	*underTest.claimed = false

	underTest.execute(t)

	assert.Empty(t, *underTest.saved)
}

func TestAnAutoOrderNotSentByItsDeadlineIsGivenUp(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
	*underTest.now = autoOrderRoundRanAt.Add(2 * time.Minute)
	underTest.venueHasNo(openClientOrderID)

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderAbandoned), settled.Outcome)
	assert.Equal(t, "訊號已經過時，沒有下單", settled.Reason)
}

func TestAnAutoOrderThatCannotReachTheVenueWaitsWithinItsDeadline(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
	*underTest.now = autoOrderRoundRanAt.Add(time.Minute)
	underTest.contractOrderProxy.EXPECT().FindMarketOrder(gomock.Any(), theCredential, "BTCUSDT", openClientOrderID).
		Return(vo.ContractOrderFillVo{}, vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureUncertain})

	underTest.execute(t)

	waiting := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderReady), waiting.Status)
	assert.Equal(t, autoOrderRoundRanAt.Add(time.Minute+5*time.Second), waiting.NextAttemptAt)
}

func TestAKeyTheVenueRejectsSwitchesOffEveryAutoOrder(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
	underTest.venueHasNo(openClientOrderID)
	underTest.accountIsOneWay()
	underTest.leverageIsSet()
	underTest.marketOrderAnswers(vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureKeyRejected})
	underTest.verificationProxy.EXPECT().VerifyTradingKey(gomock.Any(), theCredential).
		Return(vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureKeyRejected}, nil)

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderNotPlaced), settled.Outcome)
	assert.Contains(t, settled.Reason, "重存")
	require.Len(t, *underTest.switchedOff, 1)
	assert.Empty(t, (*underTest.switchedOff)[0])
}

func TestAKeyWithoutContractTradingSwitchesOffOnlyContractBots(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
	underTest.venueHasNo(openClientOrderID)
	underTest.accountIsOneWay()
	underTest.leverageIsSet()
	underTest.marketOrderAnswers(vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureKeyRejected})
	underTest.verificationProxy.EXPECT().VerifyTradingKey(gomock.Any(), theCredential).
		Return(vo.TradingKeyVerificationVo{SpotTradingEnabled: true}, nil)

	underTest.execute(t)

	require.Len(t, *underTest.switchedOff, 1)
	assert.Equal(t, []string{string(vo.MarketDataKindContractKCandle)}, (*underTest.switchedOff)[0])
}

func TestAnAutoOrderSendsNothingOnceTheBrakeIsPulled(t *testing.T) {
	testCases := []struct {
		name           string
		pullTheBrake   func(bot *entities.StrategyBot)
		expectedReason string
	}{
		{name: "自動下單已關", pullTheBrake: func(bot *entities.StrategyBot) { bot.AutoOrderEnabled = false },
			expectedReason: "自動下單已關閉，沒有下單"},
		{name: "機器人已停止", pullTheBrake: func(bot *entities.StrategyBot) { bot.RunState = string(vo.StrategyBotStopped) },
			expectedReason: "機器人已停止，沒有下單"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newAutoOrderExecutionUnderTest(t)
			underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
			testCase.pullTheBrake(underTest.bot)
			underTest.venueHasNo(openClientOrderID)

			underTest.execute(t)

			settled := underTest.lastSaved(t)
			assert.Equal(t, string(vo.ContractAutoOrderAbandoned), settled.Outcome)
			assert.Equal(t, testCase.expectedReason, settled.Reason)
		})
	}
}

// anOpenedOrder has its open done a minute after its round and still owes its stop.
func anOpenedOrder() entities.ContractAutoOrder {
	opened := anAutoOrder("long", "0.002", "1.5", "0")
	openedAt := autoOrderRoundRanAt.Add(time.Minute)
	opened.OpenDone = true
	opened.OpenedAt = &openedAt
	opened.OpenedQuantity = decimal.NewNullDecimal(decimal.RequireFromString("0.002"))
	opened.OpenAveragePrice = decimal.NewNullDecimal(decimal.NewFromInt(85000))
	opened.StopLossPrice = decimal.NewNullDecimal(decimal.NewFromInt(83725))

	return opened
}

func TestAProtectiveOrderWithNoClearAnswerIsTriedAgainForAWhile(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anOpenedOrder())
	underTest.botHolds("long", "0.002", "", "")
	*underTest.now = autoOrderRoundRanAt.Add(2 * time.Minute)
	underTest.protectiveOrderAnswers(t, vo.ContractProtectiveOrderStopLoss, vo.ContractOrderSideSell,
		"83725", "0.002", stopLossClientID, vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureUncertain})

	underTest.execute(t)

	waiting := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderReady), waiting.Status)
	assert.Empty(t, *underTest.messages)
}

func TestAProtectiveOrderStillStuckAfterAWhileIsHandedToTheOwner(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anOpenedOrder())
	underTest.botHolds("long", "0.002", "", "")
	*underTest.now = autoOrderRoundRanAt.Add(6 * time.Minute)
	underTest.protectiveOrderAnswers(t, vo.ContractProtectiveOrderStopLoss, vo.ContractOrderSideSell,
		"83725", "0.002", stopLossClientID, vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureUncertain})

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderFilled), settled.Outcome)
	assert.True(t, settled.ProtectionMissing)
}

func TestAFractionalLeverageIsNeverSentToTheVenue(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	fractional := anAutoOrder("long", "0.002", "0", "0")
	fractional.Leverage = decimal.RequireFromString("2.5")
	underTest.queues(fractional)

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderNotPlaced), settled.Outcome)
	assert.Equal(t, "交易所不收這一筆：幣安只接受整數槓桿", settled.Reason)
}
