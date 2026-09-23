package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var settlementCurrentTime = time.Date(2026, 9, 23, 10, 0, 0, 0, time.UTC)

func reportedSettlement(rate string, markPrice decimal.NullDecimal) vo.ContractFundingRateSettlementVo {
	return vo.ContractFundingRateSettlementVo{
		Symbol:         "BTCUSDT",
		SettlementTime: time.Date(2026, 9, 23, 8, 0, 0, 0, time.UTC),
		FundingRate:    decimal.RequireFromString(rate),
		MarkPrice:      markPrice,
	}
}

func TestNewContractFundingRateSettlementDomainKeepsWhatTheVenueSettled(t *testing.T) {
	testCases := []struct {
		name              string
		rate              string
		markPrice         decimal.NullDecimal
		expectedRate      string
		expectsMarkPrice  bool
		expectedMarkPrice string
	}{
		{name: "一般的結算", rate: "0.0001", markPrice: figure("87000"),
			expectedRate: "0.0001", expectsMarkPrice: true, expectedMarkPrice: "87000"},
		{name: "負的費率", rate: "-0.00003", markPrice: figure("87000"),
			expectedRate: "-0.00003", expectsMarkPrice: true, expectedMarkPrice: "87000"},
		{name: "零費率", rate: "0", markPrice: figure("87000"),
			expectedRate: "0", expectsMarkPrice: true, expectedMarkPrice: "87000"},
		{name: "早年沒給標記價格", rate: "0.0001", markPrice: decimal.NullDecimal{},
			expectedRate: "0.0001", expectsMarkPrice: false},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			settlementDomain, buildError := domains.NewContractFundingRateSettlementDomain(
				reportedSettlement(testCase.rate, testCase.markPrice), settlementCurrentTime)

			require.NoError(t, buildError)
			stored := settlementDomain.ToEntity()
			assert.Equal(t, "BTCUSDT", stored.Symbol)
			assert.Equal(t, time.Date(2026, 9, 23, 8, 0, 0, 0, time.UTC), stored.SettlementTime)
			assert.True(t, decimal.RequireFromString(testCase.expectedRate).Equal(stored.FundingRate))
			assert.Equal(t, testCase.expectsMarkPrice, stored.MarkPrice.Valid)
			if testCase.expectsMarkPrice {
				assert.True(t, decimal.RequireFromString(testCase.expectedMarkPrice).Equal(stored.MarkPrice.Decimal))
			}
		})
	}
}

func TestNewContractFundingRateSettlementDomainRefusesWhatCannotBeASettlement(t *testing.T) {
	testCases := []struct {
		name            string
		settlement      vo.ContractFundingRateSettlementVo
		expectedMessage string
	}{
		{name: "標記價格為零", settlement: reportedSettlement("0.0001", figure("0")),
			expectedMessage: "結算當下的標記價格必須大於零"},
		{name: "標記價格為負", settlement: reportedSettlement("0.0001", figure("-1")),
			expectedMessage: "結算當下的標記價格必須大於零"},
		{name: "結算時間指向未來", settlement: func() vo.ContractFundingRateSettlementVo {
			settlement := reportedSettlement("0.0001", figure("87000"))
			settlement.SettlementTime = settlementCurrentTime.Add(time.Millisecond)

			return settlement
		}(), expectedMessage: "結算時間不得指向未來"},
		{name: "沒有代號", settlement: func() vo.ContractFundingRateSettlementVo {
			settlement := reportedSettlement("0.0001", figure("87000"))
			settlement.Symbol = ""

			return settlement
		}(), expectedMessage: "交易標的"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, buildError := domains.NewContractFundingRateSettlementDomain(
				testCase.settlement, settlementCurrentTime)

			assert.ErrorIs(t, buildError, domains.ErrContractFundingRateSettlementValidation)
			assert.ErrorContains(t, buildError, testCase.expectedMessage)
		})
	}
}

func TestNewContractFundingRateSettlementDomainKeepsTheMillisecondTheVenueNamed(t *testing.T) {
	settlement := reportedSettlement("0.0001", figure("87000"))
	settlement.SettlementTime = time.Date(2026, 9, 23, 8, 0, 0, int(time.Millisecond), time.UTC)

	settlementDomain, buildError := domains.NewContractFundingRateSettlementDomain(
		settlement, settlementCurrentTime)

	require.NoError(t, buildError)
	assert.Equal(t, time.Date(2026, 9, 23, 8, 0, 0, int(time.Millisecond), time.UTC),
		settlementDomain.ToEntity().SettlementTime)
}
