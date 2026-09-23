package domains_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func reportedSpecification() vo.ContractTradingSpecificationVo {
	return vo.ContractTradingSpecificationVo{
		Symbol:                "BTCUSDT",
		TickSize:              decimal.RequireFromString("0.1"),
		QuantityStep:          decimal.RequireFromString("0.001"),
		MinimumQuantity:       decimal.RequireFromString("0.002"),
		MinimumNotional:       decimal.RequireFromString("50"),
		MaintenanceMarginRate: decimal.RequireFromString("0.025"),
		LiquidationFeeRate:    decimal.RequireFromString("0.0125"),
	}
}

func TestContractTradingSpecificationDomainFillsTheFundingIntervalTheVenueLeftUnsaid(t *testing.T) {
	fourHours := 4
	testCases := []struct {
		name          string
		reported      *int
		expectedHours int
	}{
		{name: "沒特別列出就是八小時", reported: nil, expectedHours: 8},
		{name: "四小時結算的標的", reported: &fourHours, expectedHours: 4},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			specification := reportedSpecification()
			specification.FundingIntervalHours = testCase.reported
			confirmedAt := time.Date(2026, 9, 23, 8, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))

			specificationDomain, buildError := domains.NewContractTradingSpecificationDomain(specification)
			require.NoError(t, buildError)
			applied := specificationDomain.ApplyTo(
				entities.ContractTradingSymbol{Symbol: "BTCUSDT", IsWatched: true}, confirmedAt)

			assert.Equal(t, "BTCUSDT", applied.Symbol)
			assert.True(t, applied.IsWatched)
			assert.True(t, decimal.RequireFromString("0.1").Equal(applied.TickSize.Decimal))
			assert.True(t, decimal.RequireFromString("0.001").Equal(applied.QuantityStep.Decimal))
			assert.True(t, decimal.RequireFromString("0.002").Equal(applied.MinimumQuantity.Decimal))
			assert.True(t, decimal.RequireFromString("50").Equal(applied.MinimumNotional.Decimal))
			assert.True(t, decimal.RequireFromString("0.025").Equal(applied.MaintenanceMarginRate.Decimal))
			assert.True(t, decimal.RequireFromString("0.0125").Equal(applied.LiquidationFeeRate.Decimal))
			require.NotNil(t, applied.FundingIntervalHours)
			assert.Equal(t, testCase.expectedHours, *applied.FundingIntervalHours)
			require.NotNil(t, applied.SpecificationUpdatedAt)
			assert.Equal(t, time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC), *applied.SpecificationUpdatedAt)
		})
	}
}

func TestContractTradingSpecificationDomainAcceptsWaivedFees(t *testing.T) {
	specification := reportedSpecification()
	specification.MinimumNotional = decimal.Zero
	specification.MaintenanceMarginRate = decimal.Zero
	specification.LiquidationFeeRate = decimal.Zero

	_, buildError := domains.NewContractTradingSpecificationDomain(specification)

	assert.NoError(t, buildError)
}

func TestContractTradingSpecificationDomainRefusesWhatCannotBeASpecification(t *testing.T) {
	zeroHours := 0
	testCases := []struct {
		name            string
		breakIt         func(specification *vo.ContractTradingSpecificationVo)
		expectedMessage string
	}{
		{name: "價格跳動為零", breakIt: func(specification *vo.ContractTradingSpecificationVo) {
			specification.TickSize = decimal.Zero
		}, expectedMessage: "價格跳動單位必須大於零"},
		{name: "數量步進為零", breakIt: func(specification *vo.ContractTradingSpecificationVo) {
			specification.QuantityStep = decimal.Zero
		}, expectedMessage: "數量步進必須大於零"},
		{name: "最小下單量為負", breakIt: func(specification *vo.ContractTradingSpecificationVo) {
			specification.MinimumQuantity = decimal.RequireFromString("-1")
		}, expectedMessage: "最小下單量必須大於零"},
		{name: "最小名目為負", breakIt: func(specification *vo.ContractTradingSpecificationVo) {
			specification.MinimumNotional = decimal.RequireFromString("-1")
		}, expectedMessage: "最小名目不得為負"},
		{name: "維持保證金率為負", breakIt: func(specification *vo.ContractTradingSpecificationVo) {
			specification.MaintenanceMarginRate = decimal.RequireFromString("-0.1")
		}, expectedMessage: "維持保證金率不得為負"},
		{name: "強平手續費率為負", breakIt: func(specification *vo.ContractTradingSpecificationVo) {
			specification.LiquidationFeeRate = decimal.RequireFromString("-0.1")
		}, expectedMessage: "強平手續費率不得為負"},
		{name: "結算間隔為零", breakIt: func(specification *vo.ContractTradingSpecificationVo) {
			specification.FundingIntervalHours = &zeroHours
		}, expectedMessage: "資金費率結算間隔必須大於零"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			specification := reportedSpecification()
			testCase.breakIt(&specification)

			_, buildError := domains.NewContractTradingSpecificationDomain(specification)

			assert.ErrorIs(t, buildError, domains.ErrContractTradingSpecificationValidation)
			assert.ErrorContains(t, buildError, testCase.expectedMessage)
		})
	}
}
