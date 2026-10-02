package domains

import (
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// LiveKCandleSnapshotDomain is a live candle as the replica on duty passes it to the others: the candle and when the source last reported it.
type LiveKCandleSnapshotDomain struct {
	liveKCandle vo.LiveKCandleVo
	observedAt  time.Time
}

func NewLiveKCandleSnapshotDomain(liveKCandle vo.LiveKCandleVo, observedAt time.Time) LiveKCandleSnapshotDomain {
	return LiveKCandleSnapshotDomain{liveKCandle: liveKCandle, observedAt: observedAt.UTC()}
}

func (liveKCandleSnapshotDomain LiveKCandleSnapshotDomain) ToEntity() entities.LiveKCandleSnapshot {
	liveKCandle := liveKCandleSnapshotDomain.liveKCandle

	return entities.LiveKCandleSnapshot{
		Symbol:              liveKCandle.Symbol,
		OpenTime:            liveKCandle.OpenTime.UTC(),
		Open:                liveKCandle.Open,
		High:                liveKCandle.High,
		Low:                 liveKCandle.Low,
		Close:               liveKCandle.Close,
		Volume:              liveKCandle.Volume,
		QuoteVolume:         liveKCandle.QuoteVolume,
		TakerBuyBaseVolume:  liveKCandle.TakerBuyBaseVolume,
		TakerBuyQuoteVolume: liveKCandle.TakerBuyQuoteVolume,
		Closed:              liveKCandle.Closed,
		ObservedAt:          liveKCandleSnapshotDomain.observedAt,
	}
}
