package domains

import (
	"fmt"
	"strings"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// strategyBotMessageTimeLayout names UTC explicitly because readers are in other timezones.
const strategyBotMessageTimeLayout = "2006-01-02 15:04 UTC"

// StrategyBotMessageDomain writes one round as a message: the first line (signal, bot, symbol) is what a phone notification shows, and the labelled "各來源怎麼說" block keeps sources' own buy/sell/hold from reading as a contradiction of the headline.
type StrategyBotMessageDomain struct {
	round  dto.StrategyBotRoundDto
	market StrategyBotMarketDomain
}

func NewStrategyBotMessageDomain(round dto.StrategyBotRoundDto) StrategyBotMessageDomain {
	return StrategyBotMessageDomain{
		round:  round,
		market: NewStrategyBotMarketDomain(round.MarketDataKind, round.ContractTradingMode),
	}
}

func (strategyBotMessageDomain StrategyBotMessageDomain) Text() string {
	verdict := NewSignalDomainOf(vo.SignalVo(strategyBotMessageDomain.round.Verdict))

	market := strategyBotMessageDomain.market

	// The coloured mark supplements, never replaces, the written act.
	lines := []string{
		fmt.Sprintf("%s【%s】%s · %s",
			market.HeadlineMark(verdict),
			market.HeadlineVerb(verdict),
			strategyBotMessageDomain.round.BotName,
			market.SymbolLabel(strategyBotMessageDomain.round.Symbol)),
		"",
	}

	// Contract bots state the trading mode because the same sell is a reversal under one mode and a close under another.
	if tradingModeInWords := market.TradingModeInWords(); tradingModeInWords != "" {
		lines = append(lines, fmt.Sprintf("⚙️ 交易模式 %s", tradingModeInWords))
	}

	// A missing price is stated explicitly: it means the judged candles are older than the newest stored one.
	candleWords := market.ReferenceCandleWords()

	if strategyBotMessageDomain.round.HasReference {
		lines = append(lines,
			fmt.Sprintf("💰 參考價 %s", strategyBotMessageDomain.round.ReferencePrice.String()),
			// On its own line so a phone wrap doesn't split it mid-meaning.
			fmt.Sprintf("　　%s 那一根一分鐘%s的收盤價",
				strategyBotMessageDomain.round.ReferenceTime.UTC().Format(strategyBotMessageTimeLayout),
				candleWords))
	} else {
		lines = append(lines, fmt.Sprintf("💰 參考價 目前讀不到這個交易標的的最新%s", candleWords))
	}

	lines = append(lines, strategyBotMessageDomain.positionPlanLines()...)

	// Sources are always quoted in the scripts' own words so readers can trace the conclusion.
	lines = append(lines, "", "📊 各來源怎麼說")

	for _, sourceSignal := range strategyBotMessageDomain.round.SourceSignals {
		lines = append(lines, fmt.Sprintf("　・%s（%s）：%s",
			sourceSignal.Label,
			sourceSignal.AggregationInterval,
			NewSignalDomainOf(vo.SignalVo(sourceSignal.Signal)).InWords()))
	}

	return strings.Join(lines, "\n")
}

// positionPlanLines returns nothing when no position plan is set (keeping the message unchanged) and only the lines for figures that were asked for.
func (strategyBotMessageDomain StrategyBotMessageDomain) positionPlanLines() []string {
	if !strategyBotMessageDomain.round.HasPositionPlan {
		return nil
	}

	positionPlan := strategyBotMessageDomain.round.PositionPlan

	// Labelled a suggestion in the heading so nobody reads the figures as a placed order.
	lines := []string{"", "📐 建議部位（這個系統不下單）"}

	if !positionPlan.Affordable {
		return append(lines, fmt.Sprintf(
			"　・部位資金不足，押不下 %s", positionPlan.Stake.String()))
	}

	// Contract orders show both margin and notional: one is typed, the other is the exposure.
	if strategyBotMessageDomain.market.IsContract() {
		lines = append(lines, fmt.Sprintf("　・保證金 %s（%s 倍槓桿，名目 %s）",
			positionPlan.Stake.String(), positionPlan.Leverage.String(), positionPlan.Notional.String()))
	} else {
		lines = append(lines, fmt.Sprintf("　・開倉金額 %s", positionPlan.Stake.String()))
	}

	// A venue-refused order prints only the refusal, so no one tries to place its exits.
	if positionPlan.HasVenueRefusal {
		refusal := positionPlan.VenueRefusal

		switch vo.ContractOrderRefusalReasonVo(refusal.Reason) {
		case vo.ContractOrderRefusalBelowMinimumQuantity:
			return append(lines, fmt.Sprintf("　・交易所不收這一筆：數量 %s 低於最小下單量 %s",
				refusal.Quantity.String(), refusal.MinimumQuantity.String()))
		case vo.ContractOrderRefusalBelowMinimumNotional:
			return append(lines, fmt.Sprintf("　・交易所不收這一筆：名目 %s 低於最小名目 %s",
				refusal.Notional.String(), refusal.MinimumNotional.String()))
		}

		return append(lines, fmt.Sprintf("　・交易所不收這一筆：名目 %s 那一級最高只能開 %d 倍",
			refusal.Notional.String(), refusal.TierMaximumLeverage))
	}

	if positionPlan.HasQuantity {
		lines = append(lines, fmt.Sprintf("　・數量 %s", positionPlan.Quantity.String()))
	}

	// Exit sides are spelled out because a bare price doesn't say which side it's on, and shorts swap them.
	stopSide, targetSide := "往下", "往上"
	if positionPlan.Direction == string(vo.PositionDirectionShort) {
		stopSide, targetSide = "往上", "往下"
	}

	if positionPlan.HasStopLoss {
		lines = append(lines, fmt.Sprintf("　・止損 %s（%s，虧 %s）",
			positionPlan.StopLossPrice.String(), stopSide,
			positionPlan.LossAtStop.String()))
	}

	if positionPlan.HasTakeProfit {
		lines = append(lines, fmt.Sprintf("　・止盈 %s（%s，賺 %s）",
			positionPlan.TakeProfitPrice.String(), targetSide,
			positionPlan.GainAtTarget.String()))
	}

	if positionPlan.HasLiquidationPrice {
		basis := ""
		if positionPlan.LiquidationFromSmallestTier {
			basis = "，用最小那一級估算"
		}

		lines = append(lines, fmt.Sprintf("　・預估強平價 %s（%s%s）",
			positionPlan.LiquidationPrice.String(), stopSide, basis))
	} else if positionPlan.CannotBeLiquidated {
		lines = append(lines, "　・這個槓桿下不會被強制平倉")
	}

	// Funding is an estimate from the last settled rate; the venue sets the next one.
	if positionPlan.ForContract && !positionPlan.HasFundingRate {
		lines = append(lines, "　・資金費率：還沒有資金費率紀錄")
	} else if positionPlan.ForContract {
		every := "每次結算"
		if positionPlan.FundingIntervalHours > 0 {
			every = fmt.Sprintf("每 %d 小時", positionPlan.FundingIntervalHours)
		}

		settlement := "不付也不收"
		if positionPlan.FundingPayment.IsPositive() {
			settlement = fmt.Sprintf("約付 %s", positionPlan.FundingPayment.String())
		} else if positionPlan.FundingPayment.IsNegative() {
			settlement = fmt.Sprintf("約收 %s", positionPlan.FundingPayment.Neg().String())
		}

		lines = append(lines, fmt.Sprintf("　・資金費率 %s%%（最近一次結算）：%s%s（估算）",
			positionPlan.FundingRate.Mul(oneHundredPercent).String(), every, settlement))
	}

	if positionPlan.LacksTradingSpecification {
		lines = append(lines, "　⚠️ 這個合約標的還沒有交易規格：數字未照交易所規則取整，也估不出強平價")
	}

	// Warn when the position would be liquidated before reaching its stop; without a liquidation price the rough rule ignores maintenance margin.
	if positionPlan.LiquidatesBeforeStop && positionPlan.HasLiquidationPrice {
		lines = append(lines, "　⚠️ 止損比預估強平價還遠：還沒到止損就會先被強制平倉")
	} else if positionPlan.LiquidatesBeforeStop {
		lines = append(lines,
			"　⚠️ 止損距離乘上槓桿已達 100%：還沒到止損就會先被強制平倉（未計維持保證金）")
	}

	// Replays only honour exits when given the distances, so remind the reader to fill them in.
	if positionPlan.HasStopLoss || positionPlan.HasTakeProfit {
		lines = append(lines, "　⚠️ 回測要算進止損止盈，重演時把這兩個距離填上")
	}

	return lines
}
