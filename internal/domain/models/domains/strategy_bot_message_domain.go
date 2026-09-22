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
//
// The headline is worded by the conclusion itself, because a conclusion is read as an
// instruction and the signal's own word is not always one its reader can carry out —
// see SignalDomain.HeadlineVerb.
//
// Only the headline speaks of acts. The source lines below keep quoting the scripts
// in 買入／賣出／持有, which is how a reader works back from the conclusion.
type StrategyBotMessageDomain struct {
	round dto.StrategyBotRoundDto
}

// NewStrategyBotMessageDomain takes a round to be written out.
func NewStrategyBotMessageDomain(round dto.StrategyBotRoundDto) StrategyBotMessageDomain {
	return StrategyBotMessageDomain{round: round}
}

// Text is the message.
func (strategyBotMessageDomain StrategyBotMessageDomain) Text() string {
	// What this round concluded, read once. Both halves of the headline ask about it
	// — the mark and the verb — and a round carries it as a string, so converting it
	// at each of them is the same conversion written twice.
	verdict := NewSignalDomainOf(vo.SignalVo(strategyBotMessageDomain.round.Verdict))

	// A coloured mark so the direction survives being skimmed. It opens the line
	// rather than replacing any of the words: a reader who cannot see colour, or
	// whose device draws these differently, still has 【買入】 written out — the mark
	// is faster to read, never the only thing that says it. Anything the system did
	// not recognise gets the neutral one, because a guess here is a guess about which
	// way somebody should trade.
	headlineMark := "⚪"

	switch verdict.Value() {
	case vo.SignalBuy:
		headlineMark = "🟢"
	case vo.SignalSell:
		headlineMark = "🔴"
	}

	// The verb beside the mark: a conclusion is read as an instruction, and the
	// signal's own word is not always one this reader can carry out. Asked of the
	// conclusion rather than worked out here — what an opinion asks somebody to go and
	// do is part of what that opinion means.
	headlineVerb := verdict.HeadlineVerb()

	lines := []string{
		fmt.Sprintf("%s【%s】%s · %s",
			headlineMark,
			headlineVerb,
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

	// What to put down, before the working. Somebody skimming this on a phone is
	// deciding whether to act; the evidence is for whoever then wants to check.
	lines = append(lines, strategyBotMessageDomain.positionPlanLines()...)

	// Always the signals, whatever the headline concluded. These lines are the scripts'
	// own testimony, and a script only ever says buy, sell or hold — rewriting them as
	// 做多／做空 would put words in their mouths and leave the reader unable to work
	// back from the conclusion to what produced it, which is the only reason these
	// lines are in the message at all.
	lines = append(lines, "", "📊 各來源怎麼說")

	for _, sourceSignal := range strategyBotMessageDomain.round.SourceSignals {
		lines = append(lines, fmt.Sprintf("　・%s（%s）：%s",
			sourceSignal.Label,
			sourceSignal.AggregationInterval,
			NewSignalDomainOf(vo.SignalVo(sourceSignal.Signal)).InWords()))
	}

	return strings.Join(lines, "\n")
}

// positionPlanLines are what this round suggests putting down, or nothing at all.
//
// Nothing at all is the ordinary case and it has to stay byte-for-byte quiet: a bot
// whose owner never filled the settings in sends the message it sent before any of
// this existed.
//
// Each figure's line appears only if it was asked for, which is why they are appended
// one at a time rather than formatted as one block. A bot sizing a spot purchase wants
// the amount and nothing about leverage; one that set a stop and no target wants one
// exit line, not two with a blank.
func (strategyBotMessageDomain StrategyBotMessageDomain) positionPlanLines() []string {
	if !strategyBotMessageDomain.round.HasPositionPlan {
		return nil
	}

	positionPlan := strategyBotMessageDomain.round.PositionPlan

	// Named as a suggestion in the heading itself, not in small print at the bottom.
	// Nothing here places an order, and a paragraph of prices and amounts is exactly
	// what somebody would otherwise read as confirmation that something had been.
	lines := []string{"", "📐 建議部位（這個系統不下單）"}

	if !positionPlan.Affordable {
		// The figure that could not be put down is named rather than printed as if it
		// could. Printing it plain would have somebody placing it.
		return append(lines, fmt.Sprintf(
			"　・部位資金不足，押不下 %s", positionPlan.Stake.String()))
	}

	lines = append(lines, fmt.Sprintf("　・開倉金額 %s", positionPlan.Stake.String()))

	// Which way each exit lies is written out in words. 66105.915 reads like a
	// perfectly ordinary price whichever side it was meant for, so the side is never
	// left for the reader to work out.
	if positionPlan.HasStopLoss {
		lines = append(lines, fmt.Sprintf("　・止損 %s（往下，虧 %s）",
			positionPlan.StopLossPrice.String(),
			positionPlan.LossAtStop.String()))
	}

	if positionPlan.HasTakeProfit {
		lines = append(lines, fmt.Sprintf("　・止盈 %s（往上，賺 %s）",
			positionPlan.TakeProfitPrice.String(),
			positionPlan.GainAtTarget.String()))
	}

	// A replay can now be asked to honour exits like these, but only when it is
	// asked: one given no distances still measures the strategy as though the bet
	// were carried to its closing signal. So the line stays, and says what to do
	// about it rather than only that something is missing — claiming the exits were
	// already counted would simply be a newer untruth.
	if positionPlan.HasStopLoss || positionPlan.HasTakeProfit {
		lines = append(lines, "　⚠️ 回測要算進止損止盈，重演時把這兩個距離填上")
	}

	return lines
}
