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

func tierOf(tier int, floor string, cap string, rate string, amount string, leverage int) vo.ContractMaintenanceMarginTierVo {
	return vo.ContractMaintenanceMarginTierVo{
		Tier:                  tier,
		NotionalFloor:         decimal.RequireFromString(floor),
		NotionalCap:           decimal.RequireFromString(cap),
		MaintenanceMarginRate: decimal.RequireFromString(rate),
		MaintenanceAmount:     decimal.RequireFromString(amount),
		MaximumLeverage:       leverage,
	}
}

func TestContractMaintenanceMarginLadderDomainKeepsASensibleLadderInTierOrder(t *testing.T) {
	// Reported out of order: the ladder is read by tier, not by the order it arrived in.
	ladder := vo.ContractMaintenanceMarginLadderVo{Symbol: "btcusdt", Tiers: []vo.ContractMaintenanceMarginTierVo{
		tierOf(2, "50000", "250000", "0.005", "50", 100),
		tierOf(1, "0", "50000", "0.004", "0", 125),
	}}
	confirmedAt := time.Date(2026, 9, 23, 8, 0, 0, 0, time.FixedZone("UTC+8", 8*3600))

	ladderDomain, buildError := domains.NewContractMaintenanceMarginLadderDomain(ladder)
	require.NoError(t, buildError)
	stored := ladderDomain.ToEntities(confirmedAt)

	assert.Equal(t, "BTCUSDT", ladderDomain.Symbol())
	require.Len(t, stored, 2)
	assert.Equal(t, 1, stored[0].Tier)
	assert.Equal(t, "BTCUSDT", stored[0].Symbol)
	assert.True(t, decimal.Zero.Equal(stored[0].NotionalFloor))
	assert.True(t, decimal.RequireFromString("50000").Equal(stored[0].NotionalCap))
	assert.True(t, decimal.RequireFromString("0.004").Equal(stored[0].MaintenanceMarginRate))
	assert.True(t, decimal.Zero.Equal(stored[0].MaintenanceAmount))
	assert.Equal(t, 125, stored[0].MaximumLeverage)
	assert.Equal(t, 2, stored[1].Tier)
	assert.True(t, decimal.RequireFromString("50").Equal(stored[1].MaintenanceAmount))
	assert.Equal(t, time.Date(2026, 9, 23, 0, 0, 0, 0, time.UTC), stored[0].ConfirmedAt)
	assert.Equal(t, time.UTC, stored[1].ConfirmedAt.Location())
}

func TestContractMaintenanceMarginLadderDomainRefusesALadderThatCannotBeOne(t *testing.T) {
	sensible := tierOf(1, "0", "50000", "0.004", "0", 125)
	testCases := []struct {
		name            string
		tiers           []vo.ContractMaintenanceMarginTierVo
		symbol          string
		expectedMessage string
	}{
		{name: "一級都沒有", tiers: nil, expectedMessage: "至少要有一級"},
		{name: "名目上限等於下限", tiers: []vo.ContractMaintenanceMarginTierVo{tierOf(1, "100", "100", "0.004", "0", 125)},
			expectedMessage: "名目上限必須大於下限"},
		{name: "名目下限為負", tiers: []vo.ContractMaintenanceMarginTierVo{tierOf(1, "-1", "100", "0.004", "0", 125)},
			expectedMessage: "名目下限不得為負"},
		{name: "維持保證金率大於一", tiers: []vo.ContractMaintenanceMarginTierVo{tierOf(1, "0", "100", "1.2", "0", 125)},
			expectedMessage: "維持保證金率必須介於零與一之間"},
		{name: "維持保證金率為負", tiers: []vo.ContractMaintenanceMarginTierVo{tierOf(1, "0", "100", "-0.1", "0", 125)},
			expectedMessage: "維持保證金率必須介於零與一之間"},
		{name: "速算額為負", tiers: []vo.ContractMaintenanceMarginTierVo{tierOf(1, "0", "100", "0.004", "-1", 125)},
			expectedMessage: "維持保證金速算額不得為負"},
		{name: "最高槓桿為零", tiers: []vo.ContractMaintenanceMarginTierVo{tierOf(1, "0", "100", "0.004", "0", 0)},
			expectedMessage: "最高槓桿至少一倍"},
		{name: "級數為零", tiers: []vo.ContractMaintenanceMarginTierVo{tierOf(0, "0", "100", "0.004", "0", 125)},
			expectedMessage: "級數至少是第一級"},
		{name: "第二級與第一級重疊", tiers: []vo.ContractMaintenanceMarginTierVo{
			sensible, tierOf(2, "40000", "250000", "0.005", "50", 100)}, expectedMessage: "第 1 級與第 2 級的分級重疊"},
		{name: "同一級出現兩次", tiers: []vo.ContractMaintenanceMarginTierVo{
			sensible, tierOf(1, "50000", "250000", "0.005", "50", 100)}, expectedMessage: "第 1 級出現兩次"},
		{name: "沒有代號", symbol: " ", tiers: []vo.ContractMaintenanceMarginTierVo{sensible}, expectedMessage: "交易標的"},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			symbol := "BTCUSDT"
			if testCase.symbol != "" {
				symbol = ""
			}

			_, buildError := domains.NewContractMaintenanceMarginLadderDomain(
				vo.ContractMaintenanceMarginLadderVo{Symbol: symbol, Tiers: testCase.tiers})

			assert.ErrorIs(t, buildError, domains.ErrContractMaintenanceMarginTierValidation)
			assert.ErrorContains(t, buildError, testCase.expectedMessage)
		})
	}
}

func TestContractMaintenanceMarginLadderDomainAcceptsTiersThatMeetEdgeToEdge(t *testing.T) {
	_, buildError := domains.NewContractMaintenanceMarginLadderDomain(vo.ContractMaintenanceMarginLadderVo{
		Symbol: "BTCUSDT", Tiers: []vo.ContractMaintenanceMarginTierVo{
			tierOf(1, "0", "50000", "0", "0", 125),
			tierOf(2, "50000", "250000", "1", "50", 1),
		}})

	assert.NoError(t, buildError)
}
