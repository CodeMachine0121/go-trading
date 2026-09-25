package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// ContractPositionStatisticArchiveDomain turns one daily-archive reading into a position statistic judged by the same rules as a live one.
// The archive stores a long-short ratio r, so shares are derived as r÷(1+r) and 1÷(1+r), keeping their extra decimals unrounded.
type ContractPositionStatisticArchiveDomain struct {
	statistic ContractPositionStatisticDomain
}

// NewContractPositionStatisticArchiveDomain refuses a negative ratio or missing open interest itself and leaves every other rule to NewContractPositionStatisticDomain.
func NewContractPositionStatisticArchiveDomain(
	archiveVo vo.ContractPositionStatisticArchiveVo, currentTime time.Time,
) (ContractPositionStatisticArchiveDomain, error) {
	statistic := vo.ContractPositionStatisticVo{
		Symbol:            archiveVo.Symbol,
		StatisticTime:     archiveVo.StatisticTime,
		OpenInterest:      archiveVo.OpenInterest.Decimal,
		OpenInterestValue: archiveVo.OpenInterestValue.Decimal,
	}

	if !archiveVo.OpenInterest.Valid {
		return ContractPositionStatisticArchiveDomain{}, fmt.Errorf(
			"%w: 缺持倉量", ErrContractPositionStatisticValidation)
	}
	if !archiveVo.OpenInterestValue.Valid {
		return ContractPositionStatisticArchiveDomain{}, fmt.Errorf(
			"%w: 缺持倉價值", ErrContractPositionStatisticValidation)
	}

	// Same order as ContractPositionStatisticDomain, so a reading wrong on both sides always names the same one.
	splits := []struct {
		name                  string
		ratio                 decimal.NullDecimal
		longShare, shortShare *decimal.NullDecimal
		longShortRatio        *decimal.NullDecimal
	}{
		{"多空人數比", archiveVo.AccountLongShortRatio,
			&statistic.AccountLongShare, &statistic.AccountShortShare, &statistic.AccountLongShortRatio},
		{"大戶多空持倉比", archiveVo.TopTraderPositionLongShortRatio,
			&statistic.TopTraderPositionLongShare, &statistic.TopTraderPositionShortShare,
			&statistic.TopTraderPositionLongShortRatio},
	}
	for _, split := range splits {
		// An absent ratio stays absent so the live rules report it as missing.
		if !split.ratio.Valid {
			continue
		}

		if split.ratio.Decimal.IsNegative() {
			return ContractPositionStatisticArchiveDomain{}, fmt.Errorf(
				"%w: %s的比值不得為負", ErrContractPositionStatisticValidation, split.name)
		}

		whole := decimal.NewFromInt(1).Add(split.ratio.Decimal)
		*split.longShare = decimal.NewNullDecimal(split.ratio.Decimal.Div(whole))
		*split.shortShare = decimal.NewNullDecimal(decimal.NewFromInt(1).Div(whole))
		*split.longShortRatio = split.ratio
	}

	statisticDomain, validationError := NewContractPositionStatisticDomain(statistic, currentTime)
	if validationError != nil {
		return ContractPositionStatisticArchiveDomain{}, validationError
	}

	return ContractPositionStatisticArchiveDomain{statistic: statisticDomain}, nil
}

func (archiveDomain ContractPositionStatisticArchiveDomain) ToEntity() entities.ContractPositionStatistic {
	return archiveDomain.statistic.ToEntity()
}
