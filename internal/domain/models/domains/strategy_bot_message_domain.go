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
// The headline is worded by the trading mode of the rules this round was judged by,
// because a conclusion is read as an instruction. To an account that cannot short,
// 賣出 is the act: sell what is held. To one that can, the act is 做空 — open a
// position that gains as the price falls — and a reader told to 賣出 would ask what
// they are meant to be selling.
type StrategyBotMessageDomain struct {
	round       dto.StrategyBotRoundDto
	tradingMode TradingModeDomain
}

// NewStrategyBotMessageDomain takes a round to be written out.
//
// A mode this cannot read is written as no mode at all: the zero value cannot short,
// so the message quotes the signal's own words and names nothing. That is the same
// rule the headline's coloured mark follows for a conclusion it does not recognise —
// a message is the last place to guess which way somebody should trade, and guessing
// 做空 here would be exactly that.
//
// Unreachable through the save gate, which refuses a mode it cannot read before it is
// ever stored. It is written down because the alternative to a rule is an accident.
func NewStrategyBotMessageDomain(round dto.StrategyBotRoundDto) StrategyBotMessageDomain {
	tradingMode, tradingModeError := NewTradingModeDomain(round.TradingMode)
	if tradingModeError != nil {
		return StrategyBotMessageDomain{round: round}
	}

	return StrategyBotMessageDomain{round: round, tradingMode: tradingMode}
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

	// The verb beside the mark, and the one thing the trading mode decides about a
	// message: a conclusion is read as an instruction. It starts as the signal's own
	// word, which is already the act for an account that cannot short, and is replaced
	// only where the two part company. An opinion nobody can read keeps its own
	// wording — a message is the last place to invent a direction.
	headlineVerb := strategyBotMessageDomain.signalInWords(strategyBotMessageDomain.round.Verdict)

	if strategyBotMessageDomain.tradingMode.CanGoShort() {
		switch vo.SignalVo(strategyBotMessageDomain.round.Verdict) {
		case vo.SignalBuy:
			headlineVerb = "做多"
		case vo.SignalSell:
			headlineVerb = "做空"
		}
	}

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

	// Named only where these rules can short, which is the one case a reader needs it:
	// the headline may be telling them to open a position rather than to sell one, and
	// which account these rules were written for is what makes that the right act. An
	// account that cannot short reads 買入／賣出 — its own signals, needing no
	// translator — so the line would be a sentence about the system, not the market.
	if strategyBotMessageDomain.tradingMode.CanGoShort() {
		lines = append(lines,
			fmt.Sprintf("⚙️ 交易模式 %s", strategyBotMessageDomain.tradingMode.InWords()))
	}

	// What to put down, before the working. Somebody skimming this on a phone is
	// deciding whether to act; the evidence is for whoever then wants to check.
	lines = append(lines, strategyBotMessageDomain.positionPlanLines()...)

	// Always the signals, whichever mode asked. These lines are the strategy scripts'
	// own testimony, and a script only ever says buy, sell or hold — rewriting them as
	// 做多／做空 would put words in their mouths and leave the reader unable to work
	// back from the conclusion to what produced it, which is the only reason these
	// lines are in the message at all.
	lines = append(lines, "", "📊 各來源怎麼說")

	for _, sourceSignal := range strategyBotMessageDomain.round.SourceSignals {
		lines = append(lines, fmt.Sprintf("　・%s（%s）：%s",
			sourceSignal.Label,
			sourceSignal.AggregationInterval,
			strategyBotMessageDomain.signalInWords(sourceSignal.Signal)))
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

	lines = append(lines, fmt.Sprintf("　・保證金 %s", positionPlan.Stake.String()))

	if positionPlan.Leveraged {
		lines = append(lines, fmt.Sprintf("　・名目 %s", positionPlan.Notional.String()))
	}

	// Which way each exit lies is written out in words. A short's stop sits above the
	// price, and 66105.915 reads like a perfectly ordinary price whichever side it was
	// meant for — so the side is never left for the reader to work out.
	if positionPlan.HasStopLoss {
		lines = append(lines, fmt.Sprintf("　・止損 %s（%s，虧 %s）",
			positionPlan.StopLossPrice.String(),
			exitDirectionInWords(positionPlan.SuggestsShort),
			positionPlan.LossAtStop.String()))
	}

	if positionPlan.HasTakeProfit {
		lines = append(lines, fmt.Sprintf("　・止盈 %s（%s，賺 %s）",
			positionPlan.TakeProfitPrice.String(),
			exitDirectionInWords(!positionPlan.SuggestsShort),
			positionPlan.GainAtTarget.String()))
	}

	// The one thing about this suggestion its reader cannot find out for themselves.
	// Every report card they have seen was produced without these exits, so a strategy
	// that looks profitable there has never been measured with the stop this message
	// is asking them to place.
	if positionPlan.HasStopLoss || positionPlan.HasTakeProfit {
		lines = append(lines, "　⚠️ 回測沒有把止損止盈算進去")
	}

	return lines
}

// exitDirectionInWords is which way an exit lies from the reference price.
//
// Both exits need it and they need it inverted from one another, which is exactly why
// it is one function: two copies would let a long's stop and a short's target drift
// into disagreeing about the same direction.
func exitDirectionInWords(above bool) string {
	if above {
		return "往上"
	}

	return "往下"
}

// signalInWords is a signal as a person reads it. Both the headline and every source
// line need it, which is what earns it a name of its own.
//
// It stays the signal's own vocabulary — buy, sell, hold — whatever mode is reading.
// What somebody has to go and do about one is the headline's business, above, and
// rewriting these words to match it would put them in the strategy scripts' mouths.
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
