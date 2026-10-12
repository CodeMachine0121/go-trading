package application_test

import (
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// workingEntry is an entry order still resting at the venue with this much filled.
func workingEntry(executedQuantity string) vo.ContractOrderStateVo {
	return vo.ContractOrderStateVo{
		ExchangeOrderID: "9001", Progress: vo.ContractOrderWorking,
		ExecutedQuantity: decimal.RequireFromString(executedQuantity), AveragePrice: decimal.NewFromInt(85010),
	}
}

// endedEntry is an entry order the venue will execute no more of, having filled this much.
func endedEntry(executedQuantity string, averagePrice string) vo.ContractOrderStateVo {
	return vo.ContractOrderStateVo{
		ExchangeOrderID: "9001", Progress: vo.ContractOrderEnded,
		ExecutedQuantity: decimal.RequireFromString(executedQuantity), AveragePrice: decimal.RequireFromString(averagePrice),
	}
}

// aPlacedEntryOrder is an order whose long entry of 0.002 at 85085 has rested at the venue since ten seconds after its round.
func aPlacedEntryOrder(stopLoss string) entities.ContractAutoOrder {
	order := anAutoOrder("long", "0.002", stopLoss, "0")
	placedAt := autoOrderRoundRanAt.Add(10 * time.Second)
	order.EntryPlacedAt = &placedAt
	order.EntryLimitPrice = decimal.NewNullDecimal(decimal.NewFromInt(85085))
	order.EntryExchangeOrderID = "9001"
	order.EntryExecutedQuantity = decimal.NewNullDecimal(decimal.Zero)
	order.EntryStatus = string(vo.ContractEntryWorking)

	return order
}

func (underTest autoOrderExecutionUnderTest) entryStands(state vo.ContractOrderStateVo) *gomock.Call {
	return underTest.contractOrderProxy.EXPECT().FindOrder(gomock.Any(), theCredential, "BTCUSDT", openClientOrderID).
		Return(state, vo.ContractOrderCallVo{})
}

func (underTest autoOrderExecutionUnderTest) entryCancelAnswers(state vo.ContractOrderStateVo, call vo.ContractOrderCallVo) *gomock.Call {
	return underTest.contractOrderProxy.EXPECT().CancelOrder(gomock.Any(), theCredential, "BTCUSDT", openClientOrderID).
		Return(state, call)
}

func TestAnEntryOrderIsPricedFromTheReferenceWithinTheTolerance(t *testing.T) {
	testCases := []struct {
		name          string
		target        string
		tolerance     string
		expectedSide  vo.ContractOrderSideVo
		expectedPrice string
	}{
		{name: "做多往下取整", target: "long", tolerance: "0.1", expectedSide: vo.ContractOrderSideBuy, expectedPrice: "84238.1"},
		{name: "做空往上取整", target: "short", tolerance: "0.1", expectedSide: vo.ContractOrderSideSell, expectedPrice: "84069.9"},
		{name: "容忍度 0 掛在參考價", target: "long", tolerance: "0", expectedSide: vo.ContractOrderSideBuy, expectedPrice: "84154"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newAutoOrderExecutionUnderTest(t)
			order := anAutoOrder(testCase.target, "0.002", "0", "0")
			order.ReferencePrice = decimal.NewNullDecimal(decimal.NewFromInt(84154))
			order.EntrySlippageTolerancePercentage = decimal.RequireFromString(testCase.tolerance)
			underTest.queues(order)
			underTest.entryIsNotAtTheVenue()
			underTest.accountIsOneWay()
			leverage := underTest.contractOrderProxy.EXPECT().PrepareIsolatedLeverage(gomock.Any(), theCredential, "BTCUSDT", 3).
				Return(vo.ContractOrderCallVo{})
			placing := underTest.contractOrderProxy.EXPECT().PlaceLimitOrder(gomock.Any(), theCredential, gomock.Any()).
				DoAndReturn(func(_ any, _ vo.TradingKeyCredentialVo, entryOrder vo.ContractLimitOrderVo) (vo.ContractOrderStateVo, vo.ContractOrderCallVo) {
					assert.Equal(t, testCase.expectedSide, entryOrder.Side)
					assert.True(t, entryOrder.Price.Equal(decimal.RequireFromString(testCase.expectedPrice)), "限價 %s", entryOrder.Price)
					assert.True(t, entryOrder.Quantity.Equal(decimal.RequireFromString("0.002")))
					assert.Equal(t, openClientOrderID, entryOrder.ClientOrderID)

					return workingEntry("0"), vo.ContractOrderCallVo{}
				})
			gomock.InOrder(leverage, placing)
			// No PlaceMarketOrder expectation: opening at market fails the test.

			underTest.execute(t)

			waiting := underTest.lastSaved(t)
			assertFigure(t, testCase.expectedPrice, waiting.EntryLimitPrice)
		})
	}
}

func TestAnEntryOrderStillWorkingIsLookedAtAgainShortly(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "1.5", "0"))
	underTest.entryIsNotAtTheVenue()
	underTest.accountIsOneWay()
	underTest.leverageIsSet()
	underTest.entryOrderStands(t, vo.ContractOrderSideBuy, "0.002", workingEntry("0"))

	underTest.execute(t)

	waiting := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderReady), waiting.Status)
	assert.Equal(t, autoOrderRoundRanAt.Add(15*time.Second), waiting.NextAttemptAt)
	assert.Equal(t, string(vo.ContractEntryWorking), waiting.EntryStatus)
	assert.Equal(t, "9001", waiting.EntryExchangeOrderID)
	require.NotNil(t, waiting.EntryPlacedAt)
	assert.Equal(t, autoOrderRoundRanAt.Add(10*time.Second), *waiting.EntryPlacedAt)
	assert.Empty(t, *underTest.positions)
	assert.Empty(t, *underTest.messages)
}

func TestAnEntryOrderFilledAtOnceIsGuardedAndReportedWithItsSlippage(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	order := anAutoOrder("long", "0.002", "1.5", "0")
	order.ReferencePrice = decimal.NewNullDecimal(decimal.NewFromInt(84154))
	underTest.queues(order)
	underTest.entryIsNotAtTheVenue()
	underTest.accountIsOneWay()
	underTest.leverageIsSet()
	underTest.contractOrderProxy.EXPECT().PlaceLimitOrder(gomock.Any(), theCredential, gomock.Any()).
		Return(vo.ContractOrderStateVo{
			ExchangeOrderID: "9001", Progress: vo.ContractOrderFilled,
			ExecutedQuantity: decimal.RequireFromString("0.002"), AveragePrice: decimal.NewFromInt(84180),
		}, vo.ContractOrderCallVo{})
	underTest.protectiveOrderAnswers(t, vo.ContractProtectiveOrderStopLoss, vo.ContractOrderSideSell,
		"82917.3", "0.002", stopLossClientID, vo.ContractOrderCallVo{})

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderFilled), settled.Outcome)
	assert.Equal(t, string(vo.ContractEntryFilled), settled.EntryStatus)
	position := underTest.lastPosition(t)
	assert.Equal(t, vo.TargetPositionLong, position.Direction)
	assert.True(t, position.Quantity.Equal(decimal.RequireFromString("0.002")))
	text := underTest.onlyMessage(t).Text
	assert.Contains(t, text, "✅ 成交 0.002 @ 84180")
	assert.Contains(t, text, "📌 參考價 84154｜限價 84238.1｜滑價 +0.031%")
}

func TestAnEntryOrderAlreadyAtTheVenueIsFollowedNotSentAgain(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
	underTest.entryStands(workingEntry("0"))
	// No PlaceLimitOrder expectation: sending it again fails the test.

	underTest.execute(t)

	waiting := underTest.lastSaved(t)
	require.NotNil(t, waiting.EntryPlacedAt)
	assertFigure(t, "85085", waiting.EntryLimitPrice)
	assert.Equal(t, string(vo.ContractAutoOrderReady), waiting.Status)
}

func TestAnEntryOrderThatCannotBePricedSendsNothing(t *testing.T) {
	testCases := []struct {
		name           string
		prepare        func(underTest autoOrderExecutionUnderTest, order *entities.ContractAutoOrder)
		expectedReason string
	}{
		{name: "沒有參考價", prepare: func(_ autoOrderExecutionUnderTest, order *entities.ContractAutoOrder) {
			order.ReferencePrice = decimal.NullDecimal{}
		}, expectedReason: "這一輪沒有參考價，無法掛限價進場單，不下單"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newAutoOrderExecutionUnderTest(t)
			order := anAutoOrder("long", "0.002", "0", "0")
			testCase.prepare(underTest, &order)
			underTest.queues(order)

			underTest.execute(t)

			settled := underTest.lastSaved(t)
			assert.Equal(t, string(vo.ContractAutoOrderNotPlaced), settled.Outcome)
			assert.Equal(t, testCase.expectedReason, settled.Reason)
		})
	}
}

func TestAnEntryOrderOnAContractWithoutATickSizeSendsNothing(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	*underTest.tickSize = decimal.NullDecimal{}
	underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
	underTest.accountIsOneWay()

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderNotPlaced), settled.Outcome)
	assert.Equal(t, "這個合約標的還沒有價格跳動單位，無法掛限價進場單，不下單", settled.Reason)
}

func TestAWorkingEntryOrderFillingLaterIsGuardedThen(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(aPlacedEntryOrder("1.5"))
	*underTest.now = autoOrderRoundRanAt.Add(5 * time.Minute)
	underTest.entryStands(vo.ContractOrderStateVo{
		ExchangeOrderID: "9001", Progress: vo.ContractOrderFilled,
		ExecutedQuantity: decimal.RequireFromString("0.002"), AveragePrice: decimal.NewFromInt(85000),
	})
	underTest.protectiveOrderAnswers(t, vo.ContractProtectiveOrderStopLoss, vo.ContractOrderSideSell,
		"83725", "0.002", stopLossClientID, vo.ContractOrderCallVo{})

	underTest.execute(t)

	assert.Equal(t, string(vo.ContractAutoOrderFilled), underTest.lastSaved(t).Outcome)
	assert.True(t, underTest.lastPosition(t).Quantity.Equal(decimal.RequireFromString("0.002")))
}

func TestAWorkingEntryOrderWithinItsWindowWaitsKeepingWhatItFilled(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(aPlacedEntryOrder("0"))
	*underTest.now = autoOrderRoundRanAt.Add(5 * time.Minute)
	underTest.entryStands(workingEntry("0.001"))
	// No CancelOrder expectation: withdrawing it fails the test.

	underTest.execute(t)

	waiting := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderReady), waiting.Status)
	assertFigure(t, "0.001", waiting.EntryExecutedQuantity)
	assert.Equal(t, autoOrderRoundRanAt.Add(5*time.Minute+5*time.Second), waiting.NextAttemptAt)
	assert.Empty(t, *underTest.positions)
}

func TestAnEntryOrderThatMustComeDownIsCancelledAndResolvedByWhatItFilled(t *testing.T) {
	testCases := []struct {
		name  string
		setUp func(underTest autoOrderExecutionUnderTest)
	}{
		{name: "進場時限到了", setUp: func(underTest autoOrderExecutionUnderTest) {
			*underTest.now = autoOrderRoundRanAt.Add(15 * time.Minute)
		}},
		{name: "有新的結論排在後面", setUp: func(underTest autoOrderExecutionUnderTest) {
			*underTest.laterOrderQueued = true
		}},
		{name: "自動下單已關", setUp: func(underTest autoOrderExecutionUnderTest) {
			underTest.bot.AutoOrderEnabled = false
		}},
		{name: "機器人已停止", setUp: func(underTest autoOrderExecutionUnderTest) {
			underTest.bot.RunState = string(vo.StrategyBotStopped)
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newAutoOrderExecutionUnderTest(t)
			underTest.queues(aPlacedEntryOrder("1.5"))
			*underTest.now = autoOrderRoundRanAt.Add(5 * time.Minute)
			testCase.setUp(underTest)
			looking := underTest.entryStands(workingEntry("0.001"))
			cancelling := underTest.entryCancelAnswers(endedEntry("0.001", "85000"), vo.ContractOrderCallVo{})
			gomock.InOrder(looking, cancelling)
			underTest.protectiveOrderAnswers(t, vo.ContractProtectiveOrderStopLoss, vo.ContractOrderSideSell,
				"83725", "0.001", stopLossClientID, vo.ContractOrderCallVo{})

			underTest.execute(t)

			settled := underTest.lastSaved(t)
			assert.Equal(t, string(vo.ContractAutoOrderFilled), settled.Outcome)
			assert.Equal(t, string(vo.ContractEntryPartiallyFilled), settled.EntryStatus)
			position := underTest.lastPosition(t)
			assert.Equal(t, vo.TargetPositionLong, position.Direction)
			assert.True(t, position.Quantity.Equal(decimal.RequireFromString("0.001")))
			assert.Contains(t, underTest.onlyMessage(t).Text, "⚠️ 部分成交 0.001／0.002，其餘已撤單")
		})
	}
}

func TestAnEntryOrderThatFilledNothingByItsDeadlineIsCancelledAndSaidSo(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(aPlacedEntryOrder("1.5"))
	*underTest.now = autoOrderRoundRanAt.Add(15 * time.Minute)
	underTest.entryStands(workingEntry("0"))
	underTest.entryCancelAnswers(endedEntry("0", "0"), vo.ContractOrderCallVo{})

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderUnfilled), settled.Outcome)
	assert.Equal(t, string(vo.ContractEntryUnfilled), settled.EntryStatus)
	assert.Equal(t, "進場單沒有成交，已撤單", settled.Reason)
	assert.Empty(t, *underTest.positions)
	text := underTest.onlyMessage(t).Text
	assert.Contains(t, text, "📌 參考價 85000｜限價 85085")
	assert.Contains(t, text, "❌ 進場單沒有成交，已撤單")
}

func TestAnEntryOrderTheVenueEndedByItselfIsResolved(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(aPlacedEntryOrder("0"))
	underTest.entryStands(endedEntry("0", "0"))

	underTest.execute(t)

	assert.Equal(t, string(vo.ContractAutoOrderUnfilled), underTest.lastSaved(t).Outcome)
}

func TestAnEntryOrderGoneWhenCancelledIsLookedUpAgain(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(aPlacedEntryOrder("0"))
	*underTest.now = autoOrderRoundRanAt.Add(15 * time.Minute)
	looking := underTest.entryStands(workingEntry("0.001"))
	cancelling := underTest.entryCancelAnswers(vo.ContractOrderStateVo{},
		vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureNotFound})
	lookingAgain := underTest.entryStands(vo.ContractOrderStateVo{
		ExchangeOrderID: "9001", Progress: vo.ContractOrderFilled,
		ExecutedQuantity: decimal.RequireFromString("0.002"), AveragePrice: decimal.NewFromInt(85050),
	})
	gomock.InOrder(looking, cancelling, lookingAgain)

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderFilled), settled.Outcome)
	assert.Equal(t, string(vo.ContractEntryFilled), settled.EntryStatus)
	assert.True(t, underTest.lastPosition(t).Quantity.Equal(decimal.RequireFromString("0.002")))
}

func TestAnEntryOrderWhoseCancelIsUnclearIsTriedAgain(t *testing.T) {
	testCases := []struct {
		name   string
		cancel func(underTest autoOrderExecutionUnderTest)
	}{
		{name: "撤單沒有明確回應", cancel: func(underTest autoOrderExecutionUnderTest) {
			underTest.entryCancelAnswers(vo.ContractOrderStateVo{}, vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureUncertain})
		}},
		{name: "撤單後幣安仍說在等", cancel: func(underTest autoOrderExecutionUnderTest) {
			underTest.entryCancelAnswers(workingEntry("0"), vo.ContractOrderCallVo{})
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newAutoOrderExecutionUnderTest(t)
			underTest.queues(aPlacedEntryOrder("0"))
			*underTest.now = autoOrderRoundRanAt.Add(15 * time.Minute)
			underTest.entryStands(workingEntry("0"))
			testCase.cancel(underTest)

			underTest.execute(t)

			waiting := underTest.lastSaved(t)
			assert.Equal(t, string(vo.ContractAutoOrderReady), waiting.Status)
			assert.Empty(t, waiting.Outcome)
			assert.Empty(t, *underTest.messages)
		})
	}
}

func TestAReverseWhoseEntryFilledNothingSaysItClosedButDidNotOpen(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	order := aPlacedEntryOrder("0")
	order.TargetPosition = string(vo.TargetPositionShort)
	order.CloseDone = true
	order.ClosedDirection = string(vo.TargetPositionLong)
	order.ClosedQuantity = decimal.NewNullDecimal(decimal.RequireFromString("0.002"))
	order.CloseAveragePrice = decimal.NewNullDecimal(decimal.NewFromInt(84000))
	underTest.queues(order)
	*underTest.now = autoOrderRoundRanAt.Add(15 * time.Minute)
	underTest.entryStands(workingEntry("0"))
	underTest.entryCancelAnswers(endedEntry("0", "0"), vo.ContractOrderCallVo{})

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderPartiallyDone), settled.Outcome)
	assert.Equal(t, "已平倉，但做空沒開成：進場單沒有成交，已撤單", settled.Reason)
}

func TestAWorkingEntryOrderWhoseKeyIsGoneIsHandedToTheOwner(t *testing.T) {
	testCases := []struct {
		name           string
		withdraw       func(underTest autoOrderExecutionUnderTest)
		expectedReason string
	}{
		{name: "金鑰不在", withdraw: func(underTest autoOrderExecutionUnderTest) {
			underTest.key.ContractTradingEnabled = false
		}, expectedReason: "沒有可交易合約的幣安交易金鑰；進場單可能還掛在幣安上，請自己到幣安撤單"},
		{name: "金鑰打不開", withdraw: func(underTest autoOrderExecutionUnderTest) {
			*underTest.unsealFailure = assert.AnError
		}, expectedReason: "系統目前無法開啟你的幣安交易金鑰；進場單可能還掛在幣安上，請自己到幣安撤單"},
		{name: "金鑰被拒", withdraw: func(underTest autoOrderExecutionUnderTest) {
			underTest.contractOrderProxy.EXPECT().FindOrder(gomock.Any(), theCredential, "BTCUSDT", openClientOrderID).
				Return(vo.ContractOrderStateVo{}, vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureKeyRejected})
			underTest.verificationProxy.EXPECT().VerifyTradingKey(gomock.Any(), theCredential).
				Return(vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureKeyRejected}, nil)
		}, expectedReason: "幣安不接受你的交易金鑰，已關掉你所有機器人的自動下單，請重存金鑰；進場單可能還掛在幣安上，請自己到幣安撤單"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newAutoOrderExecutionUnderTest(t)
			underTest.queues(aPlacedEntryOrder("0"))
			testCase.withdraw(underTest)

			underTest.execute(t)

			settled := underTest.lastSaved(t)
			assert.Equal(t, string(vo.ContractAutoOrderNotPlaced), settled.Outcome)
			assert.Equal(t, testCase.expectedReason, settled.Reason)
			assert.True(t, strings.HasPrefix(underTest.onlyMessage(t).Text, "🤖【自動下單】"))
		})
	}
}

func TestAnEntryOrderWhoseQueueCannotBeReadWaits(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.failures.laterOrders = storageWentAway
	underTest.queues(aPlacedEntryOrder("0"))
	underTest.entryStands(workingEntry("0"))

	settledCount, executeError := underTest.executionApplication.ExecuteDueAutoOrders(t.Context())

	require.NoError(t, executeError)
	assert.Zero(t, settledCount)
	assert.Empty(t, *underTest.saved)
}

func TestAnEntryFilledBetterThanTheReferenceShowsANegativeSlippage(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(aPlacedEntryOrder("0"))
	underTest.entryStands(vo.ContractOrderStateVo{
		ExchangeOrderID: "9001", Progress: vo.ContractOrderFilled,
		ExecutedQuantity: decimal.RequireFromString("0.002"), AveragePrice: decimal.NewFromInt(84990),
	})

	underTest.execute(t)

	assert.Contains(t, underTest.onlyMessage(t).Text, "📌 參考價 85000｜限價 85085｜滑價 -0.012%")
}
