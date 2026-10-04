package application_test

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

var storageWentAway = errors.New("the database went away")

func TestAnAutoOrderWaitingForItsRetryIsNotTakenEarly(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	waiting := anAutoOrder("long", "0.002", "0", "0")
	waiting.NextAttemptAt = autoOrderRoundRanAt.Add(15 * time.Second)
	underTest.queues(waiting)

	settledCount, executeError := underTest.executionApplication.ExecuteDueAutoOrders(t.Context())

	require.NoError(t, executeError)
	assert.Zero(t, settledCount)
	assert.Zero(t, *underTest.claims)
}

func TestAnAutoOrderAnotherReplicaStillHoldsIsLeftUntilItsClaimRunsOut(t *testing.T) {
	held := anAutoOrder("long", "", "0", "0")
	held.NoOpenReason = "沒有部位規劃，不下單"
	held.Status = string(vo.ContractAutoOrderExecuting)

	stillHeldUntil := autoOrderRoundRanAt.Add(time.Minute)
	underTest := newAutoOrderExecutionUnderTest(t)
	held.ClaimedUntil = &stillHeldUntil
	underTest.queues(held)
	underTest.execute(t)
	assert.Zero(t, *underTest.claims)

	ranOutAt := autoOrderRoundRanAt.Add(5 * time.Second)
	resumed := newAutoOrderExecutionUnderTest(t)
	held.ClaimedUntil = &ranOutAt
	resumed.queues(held)
	resumed.execute(t)
	assert.Equal(t, string(vo.ContractAutoOrderNotPlaced), resumed.lastSaved(t).Outcome)
}

func TestASettledAutoOrderIsNeverTakenAgain(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	settled := anAutoOrder("long", "0.002", "0", "0")
	settled.Status = string(vo.ContractAutoOrderSettled)
	underTest.queues(settled)

	underTest.execute(t)

	assert.Zero(t, *underTest.claims)
}

func TestAnAutoOrderWithoutAContractTradingKeySendsNothing(t *testing.T) {
	testCases := []struct {
		name     string
		withdraw func(underTest autoOrderExecutionUnderTest)
	}{
		{name: "沒有金鑰", withdraw: func(underTest autoOrderExecutionUnderTest) {
			underTest.failures.findKey = domains.ErrBinanceTradingKeyNotConfigured
		}},
		{name: "金鑰沒有合約權限", withdraw: func(underTest autoOrderExecutionUnderTest) {
			underTest.key.ContractTradingEnabled = false
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newAutoOrderExecutionUnderTest(t)
			underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
			testCase.withdraw(underTest)

			underTest.execute(t)

			settled := underTest.lastSaved(t)
			assert.Equal(t, string(vo.ContractAutoOrderAbandoned), settled.Outcome)
			assert.Equal(t, "沒有可交易合約的幣安交易金鑰，沒有下單", settled.Reason)
		})
	}
}

func TestAnAutoOrderWhoseKeyCannotBeOpenedSendsNothing(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
	*underTest.unsealFailure = errors.New("the seal key changed")

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderNotPlaced), settled.Outcome)
	assert.Equal(t, "系統目前無法開啟你的幣安交易金鑰", settled.Reason)
}

func TestAnAutoOrderPassesTheVenuesOwnWordsOn(t *testing.T) {
	testCases := []struct {
		name           string
		call           vo.ContractOrderCallVo
		expectedReason string
	}{
		{name: "交易所不收", call: vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureVenueRefused, ExchangeMessage: "X"},
			expectedReason: "交易所不收這一筆：X"},
		{name: "認不得", call: vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureOtherRefusal, ExchangeMessage: "Y"},
			expectedReason: "幣安拒絕：Y"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newAutoOrderExecutionUnderTest(t)
			underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
			underTest.venueHasNo(openClientOrderID)
			underTest.accountIsOneWay()
			underTest.leverageIsSet()
			underTest.marketOrderAnswers(testCase.call)

			underTest.execute(t)

			assert.Equal(t, testCase.expectedReason, underTest.lastSaved(t).Reason)
		})
	}
}

func TestATargetTheVenueRefusesIsNamedLoudly(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "1.5", "3"))
	underTest.venueHasNo(openClientOrderID)
	underTest.accountIsOneWay()
	underTest.leverageIsSet()
	underTest.marketOrderFills(t, vo.ContractOrderSideBuy, "0.002", false, openClientOrderID, "0.002", "85000")
	underTest.protectiveOrderAnswers(t, vo.ContractProtectiveOrderStopLoss, vo.ContractOrderSideSell,
		"83725.0", "0.002", stopLossClientID, vo.ContractOrderCallVo{})
	underTest.protectiveOrderAnswers(t, vo.ContractProtectiveOrderTakeProfit, vo.ContractOrderSideSell,
		"87550.0", "0.002", takeProfitClientID,
		vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureVenueRefused, ExchangeMessage: "no"})

	underTest.execute(t)

	assert.True(t, strings.HasPrefix(underTest.onlyMessage(t).Text, "⚠️ 止盈沒有掛上，請立刻到幣安自己處理"),
		underTest.onlyMessage(t).Text)
	assert.Equal(t, stopLossClientID, underTest.lastPosition(t).StopLossClientID)
	assert.Empty(t, underTest.lastPosition(t).TakeProfitClientID)
}

func TestAnOpenedOrderWithNoOpeningTimeIsNotRetriedForever(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	opened := anOpenedOrder()
	opened.OpenedAt = nil
	underTest.queues(opened)
	underTest.botHolds("long", "0.002", "", "")
	underTest.protectiveOrderAnswers(t, vo.ContractProtectiveOrderStopLoss, vo.ContractOrderSideSell,
		"83725", "0.002", stopLossClientID, vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureUncertain})

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderFilled), settled.Outcome)
	assert.True(t, settled.ProtectionMissing)
}

func TestAProtectiveOrderAlreadyAtTheVenueIsNotPlacedAgain(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anOpenedOrder())
	underTest.botHolds("long", "0.002", "", "")
	underTest.contractOrderProxy.EXPECT().FindProtectiveOrder(gomock.Any(), theCredential, "BTCUSDT", stopLossClientID).
		Return(vo.ContractOrderCallVo{})
	// No PlaceProtectiveOrder expectation: placing it again fails the test.

	underTest.execute(t)

	assert.False(t, underTest.lastSaved(t).ProtectionMissing)
	assert.Equal(t, stopLossClientID, underTest.lastPosition(t).StopLossClientID)
}

func TestACloseTheVenueAlreadyFilledIsNotSentAgain(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("flat", "", "0", "0"))
	underTest.botHolds("long", "0.002", earlierStopLossID, earlierTakeProfitID)
	underTest.contractOrderProxy.EXPECT().FindMarketOrder(gomock.Any(), theCredential, "BTCUSDT", closeClientOrderID).
		Return(vo.ContractOrderFillVo{
			ExecutedQuantity: decimal.RequireFromString("0.002"), AveragePrice: decimal.NewFromInt(84000),
		}, vo.ContractOrderCallVo{})

	underTest.execute(t)

	assert.Empty(t, underTest.lastPosition(t).Direction)
	assert.Equal(t, string(vo.ContractAutoOrderFilled), underTest.lastSaved(t).Outcome)
}

func TestAnAutoOrderWaitsWhenAnyStepOfACloseGetsNoClearAnswer(t *testing.T) {
	uncertain := vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureUncertain}
	testCases := []struct {
		name   string
		answer func(underTest autoOrderExecutionUnderTest)
	}{
		{name: "查平倉單", answer: func(underTest autoOrderExecutionUnderTest) {
			underTest.contractOrderProxy.EXPECT().FindMarketOrder(gomock.Any(), theCredential, "BTCUSDT", closeClientOrderID).
				Return(vo.ContractOrderFillVo{}, uncertain)
		}},
		{name: "撤保護單", answer: func(underTest autoOrderExecutionUnderTest) {
			underTest.venueHasNo(closeClientOrderID)
			underTest.accountIsOneWay()
			underTest.contractOrderProxy.EXPECT().CancelProtectiveOrder(gomock.Any(), theCredential, "BTCUSDT", earlierStopLossID).
				Return(uncertain)
		}},
		{name: "讀倉位", answer: func(underTest autoOrderExecutionUnderTest) {
			underTest.venueHasNo(closeClientOrderID)
			underTest.accountIsOneWay()
			underTest.protectiveOrdersAreTakenDown(earlierStopLossID, earlierTakeProfitID)
			underTest.contractOrderProxy.EXPECT().ReadPosition(gomock.Any(), theCredential, "BTCUSDT").
				Return(vo.ContractExchangePositionVo{}, uncertain)
		}},
		{name: "讀持倉模式", answer: func(underTest autoOrderExecutionUnderTest) {
			underTest.venueHasNo(closeClientOrderID)
			underTest.contractOrderProxy.EXPECT().ReadPositionMode(gomock.Any(), theCredential).Return(false, uncertain)
		}},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newAutoOrderExecutionUnderTest(t)
			underTest.queues(anAutoOrder("flat", "", "0", "0"))
			underTest.botHolds("long", "0.002", earlierStopLossID, earlierTakeProfitID)
			testCase.answer(underTest)

			underTest.execute(t)

			assert.Equal(t, string(vo.ContractAutoOrderReady), underTest.lastSaved(t).Status)
			assert.Empty(t, *underTest.messages)
		})
	}
}

func TestACloseTheVenueRefusesIsReported(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("flat", "", "0", "0"))
	underTest.botHolds("long", "0.002", "", "")
	underTest.venueHasNo(closeClientOrderID)
	underTest.accountIsOneWay()
	underTest.venueHolds("0.002", "0")
	underTest.marketOrderAnswers(vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureVenueRefused, ExchangeMessage: "X"})

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderNotPlaced), settled.Outcome)
	assert.Equal(t, "交易所不收這一筆：X", settled.Reason)
}

func TestARejectedKeyThatCannotBeAskedAboutAgainWaits(t *testing.T) {
	testCases := []struct {
		name         string
		verification vo.TradingKeyVerificationVo
		verifyError  error
	}{
		{name: "連不上", verification: vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureUnreachable}},
		{name: "等太久", verification: vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureTimedOut}},
		{name: "本地失敗", verifyError: errors.New("could not build the request")},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newAutoOrderExecutionUnderTest(t)
			underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
			underTest.venueHasNo(openClientOrderID)
			underTest.accountIsOneWay()
			underTest.leverageIsSet()
			underTest.marketOrderAnswers(vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureKeyRejected})
			underTest.verificationProxy.EXPECT().VerifyTradingKey(gomock.Any(), theCredential).
				Return(testCase.verification, testCase.verifyError)

			underTest.execute(t)

			assert.Equal(t, string(vo.ContractAutoOrderReady), underTest.lastSaved(t).Status)
			assert.Empty(t, *underTest.switchedOff)
		})
	}
}

func TestAnAutoOrderReportsAQueueThatCannotBeRead(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.failures.candidates = storageWentAway

	_, executeError := underTest.executionApplication.ExecuteDueAutoOrders(t.Context())

	require.ErrorIs(t, executeError, storageWentAway)
}

func TestAnAutoOrderThatCannotReadWhatItDecidesOnWritesNothing(t *testing.T) {
	testCases := []struct {
		name string
		fail func(failures *autoOrderStorageFailures)
	}{
		{name: "讀機器人", fail: func(failures *autoOrderStorageFailures) { failures.findBot = storageWentAway }},
		{name: "讀金鑰", fail: func(failures *autoOrderStorageFailures) { failures.findKey = storageWentAway }},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newAutoOrderExecutionUnderTest(t)
			underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
			testCase.fail(underTest.failures)

			settledCount, executeError := underTest.executionApplication.ExecuteDueAutoOrders(t.Context())

			require.NoError(t, executeError)
			assert.Zero(t, settledCount)
			assert.Empty(t, *underTest.saved)
			assert.Empty(t, *underTest.messages)
		})
	}
}

func TestAnAutoOrderTakenOverMidWayLeavesEverythingToItsNewHolder(t *testing.T) {
	// The first three settle at once on an order that opens nothing; the rest reach the venue first.
	testCases := []struct {
		name           string
		opensSomething bool
		fail           func(failures *autoOrderStorageFailures)
		setUp          func(underTest autoOrderExecutionUnderTest)
	}{
		{name: "結束時 claim 已被接手", fail: func(failures *autoOrderStorageFailures) { failures.claimLost = true },
			setUp: func(autoOrderExecutionUnderTest) {}},
		{name: "結束時寫不進去", fail: func(failures *autoOrderStorageFailures) { failures.saveError = storageWentAway },
			setUp: func(autoOrderExecutionUnderTest) {}},
		{name: "排通知失敗", fail: func(failures *autoOrderStorageFailures) { failures.enqueue = storageWentAway },
			setUp: func(autoOrderExecutionUnderTest) {}},
		{name: "重試時 claim 已被接手", opensSomething: true, fail: func(failures *autoOrderStorageFailures) { failures.claimLost = true },
			setUp: func(underTest autoOrderExecutionUnderTest) {
				underTest.contractOrderProxy.EXPECT().FindMarketOrder(gomock.Any(), theCredential, "BTCUSDT", openClientOrderID).
					Return(vo.ContractOrderFillVo{}, vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureUncertain})
			}},
		{name: "重試時寫不進去", opensSomething: true, fail: func(failures *autoOrderStorageFailures) { failures.saveError = storageWentAway },
			setUp: func(underTest autoOrderExecutionUnderTest) {
				underTest.contractOrderProxy.EXPECT().FindMarketOrder(gomock.Any(), theCredential, "BTCUSDT", openClientOrderID).
					Return(vo.ContractOrderFillVo{}, vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureUncertain})
			}},
		{name: "記步驟時 claim 已被接手", opensSomething: true, fail: func(failures *autoOrderStorageFailures) { failures.claimLost = true },
			setUp: func(underTest autoOrderExecutionUnderTest) { underTest.openIsAlreadyFilled() }},
		{name: "記步驟時寫不進去", opensSomething: true, fail: func(failures *autoOrderStorageFailures) { failures.saveError = storageWentAway },
			setUp: func(underTest autoOrderExecutionUnderTest) { underTest.openIsAlreadyFilled() }},
		{name: "記持倉時寫不進去", opensSomething: true, fail: func(failures *autoOrderStorageFailures) { failures.position = storageWentAway },
			setUp: func(underTest autoOrderExecutionUnderTest) { underTest.openIsAlreadyFilled() }},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newAutoOrderExecutionUnderTest(t)
			order := anAutoOrder("long", "", "0", "0")
			order.NoOpenReason = "沒有部位規劃，不下單"
			if testCase.opensSomething {
				order = anAutoOrder("long", "0.002", "0", "0")
			}
			underTest.queues(order)
			testCase.setUp(underTest)
			testCase.fail(underTest.failures)

			settledCount, executeError := underTest.executionApplication.ExecuteDueAutoOrders(t.Context())

			require.NoError(t, executeError)
			assert.Zero(t, settledCount)
		})
	}
}

// openIsAlreadyFilled is a venue that already filled this order's open, so the next write is the open's step.
func (underTest autoOrderExecutionUnderTest) openIsAlreadyFilled() {
	underTest.contractOrderProxy.EXPECT().FindMarketOrder(gomock.Any(), theCredential, "BTCUSDT", openClientOrderID).
		Return(vo.ContractOrderFillVo{
			ExecutedQuantity: decimal.RequireFromString("0.002"), AveragePrice: decimal.NewFromInt(85000),
		}, vo.ContractOrderCallVo{})
}

func TestAReverseFindingItsOldSideGoneStillOpensTheNewOne(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("short", "0.003", "0", "0"))
	underTest.botHolds("long", "0.002", "", "")
	underTest.venueHasNo(closeClientOrderID)
	underTest.accountIsOneWay()
	underTest.venueHolds("0", "0")
	underTest.venueHasNo(openClientOrderID)
	underTest.leverageIsSet()
	underTest.marketOrderFills(t, vo.ContractOrderSideSell, "0.003", false, openClientOrderID, "0.003", "84000")

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderFilled), settled.Outcome)
	assert.Contains(t, settled.Reason, "幣安上已沒有這筆倉位")
	assert.Contains(t, underTest.onlyMessage(t).Text, "做空")
	assert.Contains(t, underTest.onlyMessage(t).Text, "ℹ️ 幣安上已沒有這筆倉位")
}

func TestAnOpenedPositionWhoseKeyIsGoneIsHandedToTheOwner(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anOpenedOrder())
	underTest.botHolds("long", "0.002", "", "")
	underTest.failures.findKey = domains.ErrBinanceTradingKeyNotConfigured

	underTest.execute(t)

	settled := underTest.lastSaved(t)
	assert.Equal(t, string(vo.ContractAutoOrderFilled), settled.Outcome)
	assert.True(t, settled.ProtectionMissing)
}

func TestAnAutoOrderWhoseSwitchesCannotBeTurnedOffIsNotSettled(t *testing.T) {
	underTest := newAutoOrderExecutionUnderTest(t)
	underTest.queues(anAutoOrder("long", "0.002", "0", "0"))
	underTest.venueHasNo(openClientOrderID)
	underTest.accountIsOneWay()
	underTest.leverageIsSet()
	underTest.marketOrderAnswers(vo.ContractOrderCallVo{Failure: vo.ContractOrderFailureKeyRejected})
	underTest.verificationProxy.EXPECT().VerifyTradingKey(gomock.Any(), theCredential).
		Return(vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureKeyRejected}, nil)
	underTest.failures.switchOff = storageWentAway

	settledCount, executeError := underTest.executionApplication.ExecuteDueAutoOrders(t.Context())

	require.NoError(t, executeError)
	assert.Zero(t, settledCount)
	assert.Empty(t, *underTest.messages)
}
