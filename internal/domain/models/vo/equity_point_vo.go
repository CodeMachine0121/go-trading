package vo

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/shopspring/decimal"
)

// EquityPointVo is what everything on hand was worth once one candle had closed.
// One candle produces exactly one of these, so the curve and the candles it was
// replayed over are always the same length.
type EquityPointVo struct {
	OpenTime time.Time
	Equity   decimal.Decimal
}

// ToDto hands the point on in the shape it leaves the domain in.
func (equityPointVo EquityPointVo) ToDto() dto.EquityPointDto {
	return dto.EquityPointDto{
		OpenTime: equityPointVo.OpenTime.UTC(),
		Equity:   equityPointVo.Equity,
	}
}
