package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// strategyBotMessageTimeLayout is how the reference moment is written. Universal
// time is named in the text because a person reading this on a phone is in some
// other zone, and a timestamp with no zone is one they have to guess at.
const strategyBotMessageTimeLayout = "2006-01-02 15:04 UTC"

// StrategyBotMessageDomain writes one round as the message its owner reads.
//
// The order is fixed by where it is read: a phone's notification list shows the
// first line and maybe the second. So the first line is the signal, the bot and the
// symbol — the three things that decide whether to open it — and the reasoning comes
// after, for whoever does.
//
// Below that first line the message is two labelled blocks, and the labels are the
// point. Written as a bare run of lines, the last thing a reader's eye lands on is a
// source's own reading — and a source saying 持有 under a headline saying 買入 reads
// as a contradiction rather than as the working. It is not one: a condition can
// perfectly well be met by a source that says hold. Saying "各來源怎麼說" out loud is
// what separates the conclusion from what it was concluded from.
type StrategyBotMessageDomain struct {
	round dto.StrategyBotRoundDto
}

// NewStrategyBotMessageDomain takes a round to be written out.
func NewStrategyBotMessageDomain(round dto.StrategyBotRoundDto) StrategyBotMessageDomain {
	return StrategyBotMessageDomain{round: round}
}

// Text is the message.
func (strategyBotMessageDomain StrategyBotMessageDomain) Text() string {
	// A coloured mark so the direction survives being skimmed. It opens the line
	// rather than replacing any of the words: a reader who cannot see colour, or
	// whose device draws these differently, still has 【買入】 written out — the mark
	// is faster to read, never the only thing that says it. Anything the system did
	// not recognise gets the neutral one, because a guess here is a guess about which
	// way somebody should trade.
	headlineMark := "⚪"

	switch vo.SignalVo(strategyBotMessageDomain.round.Verdict) {
	case vo.SignalBuy:
		headlineMark = "🟢"
	case vo.SignalSell:
		headlineMark = "🔴"
	}

	lines := []string{
		fmt.Sprintf("%s【%s】%s · %s",
			headlineMark,
			strategyBotMessageDomain.signalInWords(strategyBotMessageDomain.round.Verdict),
			strategyBotMessageDomain.round.BotName,
			strategyBotMessageDomain.round.Symbol),
		"",
	}

	// A round with no price to quote still says so. Silence in this slot would read
	// as a price of nothing, and the missing line is itself worth knowing: it means
	// the candles this bot judged by are older than the newest one stored.
	if strategyBotMessageDomain.round.HasReference {
		lines = append(lines,
			fmt.Sprintf("💰 參考價 %s", strategyBotMessageDomain.round.ReferencePrice.String()),
			// The moment sits under the number rather than beside it. Together they
			// make a line long enough to wrap on a phone, and a wrapped line breaks
			// wherever the screen happens to end — which is never where the meaning
			// does.
			fmt.Sprintf("　　%s 那一根一分鐘 K 線的收盤價",
				strategyBotMessageDomain.round.ReferenceTime.UTC().Format(strategyBotMessageTimeLayout)))
	} else {
		lines = append(lines, "💰 參考價 目前讀不到這個交易標的的最新 K 線")
	}

	lines = append(lines, "", "📊 各來源怎麼說")

	for _, sourceSignal := range strategyBotMessageDomain.round.SourceSignals {
		lines = append(lines, fmt.Sprintf("　・%s（%s）：%s",
			sourceSignal.Label,
			sourceSignal.AggregationInterval,
			strategyBotMessageDomain.signalInWords(sourceSignal.Signal)))
	}

	return strings.Join(lines, "\n")
}

// signalInWords is a signal as a person reads it. Both the headline and every source
// line need it, which is what earns it a name of its own.
//
// An unrecognised value is written out as it stands rather than replaced with a
// guess: a message is the last place to quietly turn something the system did not
// understand into one of the three things it did.
func (strategyBotMessageDomain StrategyBotMessageDomain) signalInWords(signal string) string {
	switch vo.SignalVo(signal) {
	case vo.SignalBuy:
		return "買入"
	case vo.SignalSell:
		return "賣出"
	case vo.SignalHold:
		return "持有"
	default:
		return signal
	}
}
