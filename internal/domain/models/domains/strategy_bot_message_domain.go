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
// because a conclusion is read as an instruction — and 賣出 is an instruction neither
// account can reliably carry out. To one that can short, the act is 做空: open a
// position that gains as the price falls, and a reader told to 賣出 asks what they
// are meant to be selling. To one that cannot, the act is 出場: close whatever is
// open, and a reader told to 賣出 asks the same question every time they are flat —
// which is most of the time, because a sell reaches them on the strength of the
// signal alone and the signal has never known what they hold.
//
// Only the headline speaks of acts. The source lines below keep quoting the scripts
// in 買入／賣出／持有, which is how a reader works back from the conclusion.
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
	// word and is replaced only where the word and the act part company. An opinion
	// nobody can read keeps its own wording — a message is the last place to invent a
	// direction.
	//
	// Both replacements are the same failure in two modes: 賣出 is an instruction the
	// reader may be unable to carry out, and which one they cannot carry out is what
	// the mode decides. Told 賣出 by rules that can short, they ask what they are
	// meant to be selling — the act is to open a position. Told it by rules that
	// cannot, they ask it whenever they are flat, which is most of the time: a sell
	// reaches them on the strength of the signal alone, and the signal has never
	// known what they hold. 出場 is the one wording both of those readers can act on.
	headlineVerb := strategyBotMessageDomain.signalInWords(strategyBotMessageDomain.round.Verdict)

	if strategyBotMessageDomain.tradingMode.CanGoShort() {
		switch vo.SignalVo(strategyBotMessageDomain.round.Verdict) {
		case vo.SignalBuy:
			headlineVerb = "做多"
		case vo.SignalSell:
			headlineVerb = "做空"
		}
	} else if strategyBotMessageDomain.tradingMode.TargetFor(
		NewSignalDomainOf(vo.SignalVo(strategyBotMessageDomain.round.Verdict)),
	) == vo.TargetPositionFlat {
		// Asked of the mode rather than matched against the signal here, because the
		// mode is what knows that its sell asks for nothing to be held. It also
		// settles the unreadable mode for free: that one asks for nothing at all
		// rather than for flat, so its message keeps quoting the signal, which is
		// the rule for a mode this cannot read.
		headlineVerb = "出場"
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

	// Named where the headline's verb does not already say everything the act
	// involves — which is every mode but one.
	//
	// Cash for goods is the exception: 買入 there means handing over money for a
	// thing, and there is no second reading, so the line would be a sentence about
	// the system rather than about the market. The other two each carry something
	// the verb cannot say. Shorting rules may be telling the reader to open a
	// position rather than close one. Borrowing rules turn the very same 買入 into
	// a position held on somebody else's money, which can be taken away from them at
	// a price — and a reader who executes that in a cash frame of mind has been
	// misled by a message that was technically correct.
	//
	// It opens with a blank line of its own, like every other block below the
	// headline. Without one it renders glued to the reference moment, and a reader
	// skimming a phone reads the two as one paragraph — as though the mode were
	// something about that price rather than about the rules that judged the round.
	if strategyBotMessageDomain.tradingMode.CanGoShort() ||
		strategyBotMessageDomain.tradingMode.CanUseLeverage() {
		lines = append(lines, "",
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
