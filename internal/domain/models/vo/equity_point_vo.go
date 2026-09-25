package vo

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// EquityPointVo is the total value after one candle closed; there is exactly one per candle.
type EquityPointVo struct {
	OpenTime time.Time
	Equity   decimal.Decimal
}

func (equityPointVo EquityPointVo) ToDto() dto.EquityPointDto {
	return dto.EquityPointDto{
		OpenTime: equityPointVo.OpenTime.UTC(),
		Equity:   equityPointVo.Equity,
	}
}
