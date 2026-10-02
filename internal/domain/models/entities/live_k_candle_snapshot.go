package entities

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// LiveKCandleSnapshot is the latest the replica on duty saw of one rostered symbol's minute, one row per symbol and minute,
// so replicas off duty can show the same live updates, closing candles included, without spending the source's limited places.
type LiveKCandleSnapshot struct {
	Symbol              string              `gorm:"primaryKey;size:64"`
	OpenTime            time.Time           `gorm:"primaryKey;type:timestamptz"`
	Open                decimal.Decimal     `gorm:"type:numeric(38,18);not null"`
	High                decimal.Decimal     `gorm:"type:numeric(38,18);not null"`
	Low                 decimal.Decimal     `gorm:"type:numeric(38,18);not null"`
	Close               decimal.Decimal     `gorm:"type:numeric(38,18);not null"`
	Volume              decimal.Decimal     `gorm:"type:numeric(38,18);not null"`
	QuoteVolume         decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	TakerBuyBaseVolume  decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	TakerBuyQuoteVolume decimal.NullDecimal `gorm:"type:numeric(38,18)"`
	Closed              bool                `gorm:"not null"`
	// ObservedAt is when the replica on duty last heard from the source, so a relay can tell a fresh report of the same candle from silence.
	ObservedAt time.Time `gorm:"type:timestamptz;not null;index:idx_live_k_candle_snapshots_observed_at"`
}

func (liveKCandleSnapshot LiveKCandleSnapshot) TableName() string {
	return "LiveKCandleSnapshots"
}

func (liveKCandleSnapshot LiveKCandleSnapshot) ToLiveKCandleVo() vo.LiveKCandleVo {
	return vo.LiveKCandleVo{
		Symbol:              liveKCandleSnapshot.Symbol,
		OpenTime:            liveKCandleSnapshot.OpenTime.UTC(),
		Open:                liveKCandleSnapshot.Open,
		High:                liveKCandleSnapshot.High,
		Low:                 liveKCandleSnapshot.Low,
		Close:               liveKCandleSnapshot.Close,
		Volume:              liveKCandleSnapshot.Volume,
		QuoteVolume:         liveKCandleSnapshot.QuoteVolume,
		TakerBuyBaseVolume:  liveKCandleSnapshot.TakerBuyBaseVolume,
		TakerBuyQuoteVolume: liveKCandleSnapshot.TakerBuyQuoteVolume,
		Closed:              liveKCandleSnapshot.Closed,
	}
}
