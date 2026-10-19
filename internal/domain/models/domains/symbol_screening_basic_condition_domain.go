package domains

import (
	"fmt"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

const symbolScreeningQuoteAsset = "USDT"

// SymbolScreeningBasicConditionDomain rules out what no replay can rescue, in order: not on the venue, not trading, too little history, too little turnover.
type SymbolScreeningBasicConditionDomain struct {
	symbol                         string
	listing                        vo.ContractSymbolListingVo
	kCandles                       []entities.KCandleContract
	period                         SymbolScreeningPeriodDomain
	minimumAverageDailyQuoteVolume decimal.Decimal
}

func NewSymbolScreeningBasicConditionDomain(
	symbol string,
	listing vo.ContractSymbolListingVo,
	kCandles []entities.KCandleContract,
	period SymbolScreeningPeriodDomain,
	minimumAverageDailyQuoteVolume decimal.Decimal,
) SymbolScreeningBasicConditionDomain {
	return SymbolScreeningBasicConditionDomain{
		symbol:                         symbol,
		listing:                        listing,
		kCandles:                       kCandles,
		period:                         period,
		minimumAverageDailyQuoteVolume: minimumAverageDailyQuoteVolume,
	}
}

// Judge answers whether the candidate may go on to the replay; a refusal carries the reason in the words the administrator reads.
func (basicConditionDomain SymbolScreeningBasicConditionDomain) Judge() (bool, string, dto.SymbolScreeningFindingsDto) {
	findings := dto.SymbolScreeningFindingsDto{}
	if !strings.HasSuffix(basicConditionDomain.symbol, symbolScreeningQuoteAsset) || !basicConditionDomain.listing.IsKnownToVenue {
		return false, "幣安沒有這個 USDT 永續合約", findings
	}
	if !basicConditionDomain.listing.IsListed {
		return false, ErrSymbolScreeningSymbolNotTrading.Error(), findings
	}

	period := basicConditionDomain.period
	requiredDays := int(period.EndTime().Sub(period.StartTime()) / symbolScreeningDay)
	historyDays := 0
	firstOpenTime := time.Time{}
	for _, kCandle := range basicConditionDomain.kCandles {
		if firstOpenTime.IsZero() || kCandle.OpenTime.Before(firstOpenTime) {
			firstOpenTime = kCandle.OpenTime
		}
	}
	if !firstOpenTime.IsZero() {
		historyDays = int(period.EndTime().Sub(firstOpenTime) / symbolScreeningDay)
	}
	findings.HistoryDays = &historyDays
	if historyDays < requiredDays {
		return false, fmt.Sprintf("歷史只有 %d 天，需要 %d 天", historyDays, requiredDays), findings
	}

	recentStart := period.EndTime().Add(-symbolScreeningRecentDays * symbolScreeningDay)
	recentQuoteVolume := decimal.Zero
	for _, kCandle := range basicConditionDomain.kCandles {
		if !kCandle.OpenTime.Before(recentStart) && kCandle.OpenTime.Before(period.EndTime()) {
			recentQuoteVolume = recentQuoteVolume.Add(kCandle.QuoteVolume)
		}
	}
	averageDailyQuoteVolume := recentQuoteVolume.Div(decimal.NewFromInt(symbolScreeningRecentDays))
	findings.AverageDailyQuoteVolume = &averageDailyQuoteVolume
	if averageDailyQuoteVolume.LessThan(basicConditionDomain.minimumAverageDailyQuoteVolume) {
		return false, fmt.Sprintf("最近 %d 天日均成交額 %s USDT，低於門檻 %s USDT", symbolScreeningRecentDays,
			averageDailyQuoteVolume.Round(0).String(), basicConditionDomain.minimumAverageDailyQuoteVolume.String()), findings
	}

	return true, "", findings
}
