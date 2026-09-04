package marketdata

import (
	"fmt"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

// binanceLiveKLineMessage is one message from the live channel exactly as it
// arrives. Unlike the fetched form, the live one names its fields — but it names
// them in one or two letters, so this struct is where those letters are translated
// once and never again.
type binanceLiveKLineMessage struct {
	KLine binanceLiveKLine `json:"k"`
}

type binanceLiveKLine struct {
	OpenTimeMilliseconds int64  `json:"t"`
	Symbol               string `json:"s"`
	Open                 string `json:"o"`
	Close                string `json:"c"`
	High                 string `json:"h"`
	Low                  string `json:"l"`
	Volume               string `json:"v"`
	QuoteVolume          string `json:"q"`
	TakerBuyBaseVolume   string `json:"V"`
	TakerBuyQuoteVolume  string `json:"Q"`
	Closed               bool   `json:"x"`

	// The two below are never read. They are declared because decoding matches a key
	// case-insensitively once no field claims it exactly, and this source names two
	// pairs of different things with the same letter in different cases:
	//
	//   "L" is the last trade's number — without this field it lands in "l", the low,
	//       and a number will not go into a price, so the whole message is refused.
	//   "T" is when the candle closes — without this field it lands in "t", the open,
	//       and nothing complains: every candle is simply stamped one interval late
	//       and merges into the wrong one.
	//
	// Claiming the keys exactly is what keeps them apart. The second one is the more
	// dangerous of the two precisely because it never says anything.
	CloseTimeMilliseconds int64 `json:"T"`
	LastTradeNumber       int64 `json:"L"`
}

// toLiveKCandleVo turns one live message into the shape the domain accepts.
// Nothing is judged here — the K candle rules are applied further in, and only to
// the candles that have closed.
func (kLine binanceLiveKLine) toLiveKCandleVo() (vo.LiveKCandleVo, error) {
	figures, figureError := kLine.figures()
	if figureError != nil {
		return vo.LiveKCandleVo{}, figureError
	}

	return vo.LiveKCandleVo{
		Symbol:              kLine.Symbol,
		OpenTime:            time.UnixMilli(kLine.OpenTimeMilliseconds).UTC(),
		Open:                figures[0],
		High:                figures[1],
		Low:                 figures[2],
		Close:               figures[3],
		Volume:              figures[4],
		QuoteVolume:         figures[5],
		TakerBuyBaseVolume:  figures[6],
		TakerBuyQuoteVolume: figures[7],
		Closed:              kLine.Closed,
	}, nil
}

// figures reads every quoted decimal in one pass so that a source sending something
// unreadable fails as one message rather than as a candle with a hole in it.
func (kLine binanceLiveKLine) figures() ([8]decimal.Decimal, error) {
	quoted := [8]string{
		kLine.Open, kLine.High, kLine.Low, kLine.Close,
		kLine.Volume, kLine.QuoteVolume, kLine.TakerBuyBaseVolume, kLine.TakerBuyQuoteVolume,
	}

	figures := [8]decimal.Decimal{}
	for index, text := range quoted {
		figure, parseError := decimal.NewFromString(text)
		if parseError != nil {
			return [8]decimal.Decimal{}, fmt.Errorf(
				"k candle figure from market source is not a number: %q", text)
		}
		figures[index] = figure
	}

	return figures, nil
}
