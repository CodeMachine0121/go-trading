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

// DefaultMistakeTagNames are what every person starts with, so the first review has something to pick from.
var DefaultMistakeTagNames = []string{"追價進場", "移動止損", "提早出場", "部位過大", "報復性交易"}

type TradeJournalSettingDomain struct {
	setting entities.TradeJournalSetting
}

func NewTradeJournalSettingDomain(setting entities.TradeJournalSetting) TradeJournalSettingDomain {
	return TradeJournalSettingDomain{setting: setting}
}

// WithFeeRates refuses a negative rate, which would pay the person for trading.
func (tradeJournalSettingDomain TradeJournalSettingDomain) WithFeeRates(
	writeDto dto.TradeJournalSettingWriteDto,
) (TradeJournalSettingDomain, error) {
	for _, feeRate := range []decimal.NullDecimal{writeDto.MakerFeeRate, writeDto.TakerFeeRate} {
		if feeRate.Valid && feeRate.Decimal.IsNegative() {
			return TradeJournalSettingDomain{}, fmt.Errorf(
				"%w: 手續費率不得為負", ErrTradeJournalSettingValidation)
		}
	}

	updatedSetting := tradeJournalSettingDomain.setting
	updatedSetting.MakerFeeRate = writeDto.MakerFeeRate
	updatedSetting.TakerFeeRate = writeDto.TakerFeeRate

	return TradeJournalSettingDomain{setting: updatedSetting}, nil
}

// FeeFor prices a fill at the rate for its liquidity; rateMissing marks a zero that only means no rate was set.
func (tradeJournalSettingDomain TradeJournalSettingDomain) FeeFor(
	price decimal.Decimal, quantity decimal.Decimal, liquidity vo.TradeFillLiquidityVo,
) (fee decimal.Decimal, rateMissing bool) {
	feeRate := tradeJournalSettingDomain.setting.TakerFeeRate
	if liquidity == vo.TradeFillLiquidityMaker {
		feeRate = tradeJournalSettingDomain.setting.MakerFeeRate
	}

	if !feeRate.Valid {
		return decimal.Zero, true
	}

	return price.Mul(quantity).Mul(feeRate.Decimal).Div(oneHundredPercent), false
}

// TakerFeeRate is zero when unset, which is how a comparison replay treats an unknown cost.
func (tradeJournalSettingDomain TradeJournalSettingDomain) TakerFeeRate() decimal.Decimal {
	return tradeJournalSettingDomain.setting.TakerFeeRate.Decimal
}

func (tradeJournalSettingDomain TradeJournalSettingDomain) NeedsDefaultMistakeTags() bool {
	return tradeJournalSettingDomain.setting.DefaultMistakeTagsSeededAt == nil
}

func (tradeJournalSettingDomain TradeJournalSettingDomain) ToEntity(userID uint) entities.TradeJournalSetting {
	entity := tradeJournalSettingDomain.setting
	entity.UserID = userID

	return entity
}

// PricedFill turns what the person wrote into a fill: a blank time is now, a blank liquidity is taker (the costlier guess), and a blank fee comes from the rate.
func (tradeJournalSettingDomain TradeJournalSettingDomain) PricedFill(
	writeDto dto.ContractTradeFillWriteDto, now time.Time,
) entities.ContractTradeFill {
	filledAt := now
	if writeDto.FilledAt != nil {
		filledAt = *writeDto.FilledAt
	}

	liquidity := vo.TradeFillLiquidityVo(strings.TrimSpace(writeDto.Liquidity))
	if liquidity == "" {
		liquidity = vo.TradeFillLiquidityTaker
	}

	fill := entities.ContractTradeFill{
		Kind:      strings.TrimSpace(writeDto.Kind),
		FilledAt:  filledAt.UTC(),
		Price:     writeDto.Price,
		Quantity:  writeDto.Quantity,
		Liquidity: string(liquidity),
		Fee:       writeDto.Fee.Decimal,
	}

	if !writeDto.Fee.Valid {
		fill.Fee, fill.FeeRateMissing = tradeJournalSettingDomain.FeeFor(
			writeDto.Price, writeDto.Quantity, liquidity)
	}

	return fill
}
