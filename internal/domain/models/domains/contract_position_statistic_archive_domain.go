package domains

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// ContractPositionStatisticArchiveDomain is one reading from the venue's daily
// archive, turned into a position statistic that may be stored — judged by exactly
// the rules a live reading is, so that nothing downstream learns where it came from.
//
// **The archive keeps a long-short ratio where the live source keeps two shares.**
// The shares are worked back out of it: a ratio r of long to short means the long
// side holds r parts of every r + 1, so the long share is r ÷ (1 + r) and the short
// share 1 ÷ (1 + r). They come out with more decimals than the live source's four,
// and that difference is accepted rather than rounded away.
//
// **Only the rules the arithmetic depends on are checked here.** A negative ratio is
// refused before anything is worked out, because at −1 the sum is zero and below it
// the shares would be numbers with no meaning; open interest missing is named because
// there is nothing to carry over. Every other rule — a split missing, a share out of
// range, a time off the grid or in the future — is ContractPositionStatisticDomain's,
// which this hands the worked-out reading to rather than repeating.
type ContractPositionStatisticArchiveDomain struct {
	statistic ContractPositionStatisticDomain
}

// NewContractPositionStatisticArchiveDomain works the shares out of one archive
// reading and judges the result, judging "in the future" against currentTime. Any
// rule broken on the way refuses the reading, with that rule named.
func NewContractPositionStatisticArchiveDomain(
	archiveVo vo.ContractPositionStatisticArchiveVo, currentTime time.Time,
) (ContractPositionStatisticArchiveDomain, error) {
	statistic := vo.ContractPositionStatisticVo{
		Symbol:            archiveVo.Symbol,
		StatisticTime:     archiveVo.StatisticTime,
		OpenInterest:      archiveVo.OpenInterest.Decimal,
		OpenInterestValue: archiveVo.OpenInterestValue.Decimal,
	}

	// The two figures of open interest are what makes a reading a reading at all, so
	// one missing is named here rather than stored as a zero nobody asked for.
	if !archiveVo.OpenInterest.Valid {
		return ContractPositionStatisticArchiveDomain{}, fmt.Errorf(
			"%w: 缺持倉量", ErrContractPositionStatisticValidation)
	}
	if !archiveVo.OpenInterestValue.Valid {
		return ContractPositionStatisticArchiveDomain{}, fmt.Errorf(
			"%w: 缺持倉價值", ErrContractPositionStatisticValidation)
	}

	// Listed in the order ContractPositionStatisticDomain names them, so a reading
	// wrong on both sides is always told about the same one.
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
		// An absent ratio stays absent on all three figures, and the live rules below
		// say "缺…" about it exactly as they would for a live reading missing a split.
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

// ToEntity converts the judged reading into the record shape that is stored.
func (archiveDomain ContractPositionStatisticArchiveDomain) ToEntity() entities.ContractPositionStatistic {
	return archiveDomain.statistic.ToEntity()
}
