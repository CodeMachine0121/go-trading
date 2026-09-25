package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// ContractPositionStatisticInterval is the venue's sampling grid for position statistics.
const ContractPositionStatisticInterval = 5 * time.Minute

// ContractPositionStatisticDomain only exists when every rule passed, so both long-short splits are always present.
type ContractPositionStatisticDomain struct {
	statistic entities.ContractPositionStatistic
}

func NewContractPositionStatisticDomain(
	statisticVo vo.ContractPositionStatisticVo, currentTime time.Time,
) (ContractPositionStatisticDomain, error) {
	contractSymbol, symbolError := NewTradingSymbolDomain(statisticVo.Symbol)
	if symbolError != nil {
		return ContractPositionStatisticDomain{}, fmt.Errorf(
			"%w: %w", ErrContractPositionStatisticValidation, symbolError)
	}

	statisticTime := statisticVo.StatisticTime.UTC()
	if !statisticTime.Truncate(ContractPositionStatisticInterval).Equal(statisticTime) {
		return ContractPositionStatisticDomain{}, fmt.Errorf(
			"%w: 統計時間必須落在五分鐘刻度", ErrContractPositionStatisticValidation)
	}
	if statisticTime.After(currentTime) {
		return ContractPositionStatisticDomain{}, fmt.Errorf(
			"%w: 統計時間不得指向未來", ErrContractPositionStatisticValidation)
	}

	if statisticVo.OpenInterest.IsNegative() {
		return ContractPositionStatisticDomain{}, fmt.Errorf(
			"%w: 持倉量不得為負", ErrContractPositionStatisticValidation)
	}
	if statisticVo.OpenInterestValue.IsNegative() {
		return ContractPositionStatisticDomain{}, fmt.Errorf(
			"%w: 持倉價值不得為負", ErrContractPositionStatisticValidation)
	}

	// Fixed order so a statistic missing both splits always names the same one.
	longShortSplits := []struct {
		name                  string
		longShare, shortShare decimal.NullDecimal
		longShortRatio        decimal.NullDecimal
	}{
		{"多空人數比", statisticVo.AccountLongShare, statisticVo.AccountShortShare,
			statisticVo.AccountLongShortRatio},
		{"大戶多空持倉比", statisticVo.TopTraderPositionLongShare, statisticVo.TopTraderPositionShortShare,
			statisticVo.TopTraderPositionLongShortRatio},
	}
	for _, split := range longShortSplits {
		if !split.longShare.Valid || !split.shortShare.Valid || !split.longShortRatio.Valid {
			return ContractPositionStatisticDomain{}, fmt.Errorf(
				"%w: 缺%s", ErrContractPositionStatisticValidation, split.name)
		}

		// A share of 1 is a lopsided market, not a broken reading.
		for _, share := range []decimal.Decimal{split.longShare.Decimal, split.shortShare.Decimal} {
			if share.IsNegative() || share.GreaterThan(decimal.NewFromInt(1)) {
				return ContractPositionStatisticDomain{}, fmt.Errorf(
					"%w: %s的佔比必須介於零與一之間", ErrContractPositionStatisticValidation, split.name)
			}
		}

		if split.longShortRatio.Decimal.IsNegative() {
			return ContractPositionStatisticDomain{}, fmt.Errorf(
				"%w: %s的比值不得為負", ErrContractPositionStatisticValidation, split.name)
		}
	}

	return ContractPositionStatisticDomain{statistic: entities.ContractPositionStatistic{
		Symbol:                          contractSymbol.Value(),
		StatisticTime:                   statisticTime,
		OpenInterest:                    statisticVo.OpenInterest,
		OpenInterestValue:               statisticVo.OpenInterestValue,
		AccountLongShare:                statisticVo.AccountLongShare.Decimal,
		AccountShortShare:               statisticVo.AccountShortShare.Decimal,
		AccountLongShortRatio:           statisticVo.AccountLongShortRatio.Decimal,
		TopTraderPositionLongShare:      statisticVo.TopTraderPositionLongShare.Decimal,
		TopTraderPositionShortShare:     statisticVo.TopTraderPositionShortShare.Decimal,
		TopTraderPositionLongShortRatio: statisticVo.TopTraderPositionLongShortRatio.Decimal,
	}}, nil
}

func (statisticDomain ContractPositionStatisticDomain) ToEntity() entities.ContractPositionStatistic {
	return statisticDomain.statistic
}
