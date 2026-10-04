package domains

import (
	"fmt"
	"strings"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
)

const (
	// ContractAutoOrderDeadline is how long after its round an order not yet sent is still worth sending; a late order is the very delay auto orders exist to remove.
	ContractAutoOrderDeadline = 2 * time.Minute
	// contractAutoOrderRetryWait is how long an order waits after the venue could not be reached or did not say how a request went.
	contractAutoOrderRetryWait = 5 * time.Second
	// contractAutoOrderProtectionWindow is how long after the open fill a protective order that could not get through is still tried, before the owner is told to place it himself.
	contractAutoOrderProtectionWindow = 5 * time.Minute
	// contractAutoOrderClientIDPrefix marks the venue orders this system placed, so they can be found again by the same id.
	contractAutoOrderClientIDPrefix = "gt-ao-"
)

// ContractAutoOrderStepVo is what an auto order does next.
type ContractAutoOrderStepVo string

const (
	ContractAutoOrderStepClose   ContractAutoOrderStepVo = "close"
	ContractAutoOrderStepOpen    ContractAutoOrderStepVo = "open"
	ContractAutoOrderStepProtect ContractAutoOrderStepVo = "protect"
	ContractAutoOrderStepSettle  ContractAutoOrderStepVo = "settle"
)

// ContractAutoOrderDomain carries one auto order through its steps: close what the bot opened, open the new side, guard it, and settle.
// Every decision is made here from what the order has already done and what the bot holds; the caller only performs the calls it is told to.
type ContractAutoOrderDomain struct {
	order entities.ContractAutoOrder
}

func NewContractAutoOrderDomain(order entities.ContractAutoOrder) ContractAutoOrderDomain {
	return ContractAutoOrderDomain{order: order}
}

// NewQueuedContractAutoOrderDomain queues a round's intent, due at once and worth sending until the deadline after the round ran.
func NewQueuedContractAutoOrderDomain(
	strategyBotID uint, ownerUserID uint, symbol string, roundDueAt time.Time, runNumber int,
	intent dto.ContractAutoOrderIntentDto, ranAt time.Time,
) ContractAutoOrderDomain {
	order := entities.ContractAutoOrder{
		StrategyBotID:        strategyBotID,
		RoundDueAt:           roundDueAt.UTC(),
		RunNumber:            runNumber,
		OwnerUserID:          ownerUserID,
		Symbol:               symbol,
		TargetPosition:       intent.TargetPosition,
		NoOpenReason:         intent.NoOpenReason,
		Leverage:             intent.Leverage,
		StopLossPercentage:   intent.StopLossPercentage,
		TakeProfitPercentage: intent.TakeProfitPercentage,
		Status:               string(vo.ContractAutoOrderReady),
		NextAttemptAt:        ranAt.UTC(),
		ExpiresAt:            ranAt.UTC().Add(ContractAutoOrderDeadline),
		CreatedAt:            ranAt.UTC(),
	}
	if intent.HasOpenQuantity {
		order.OpenQuantity = decimal.NewNullDecimal(intent.OpenQuantity)
	}

	return ContractAutoOrderDomain{order: order}
}

func (orderDomain ContractAutoOrderDomain) ToEntity() entities.ContractAutoOrder {
	return orderDomain.order
}

func (orderDomain ContractAutoOrderDomain) ID() uint {
	return orderDomain.order.ID
}

func (orderDomain ContractAutoOrderDomain) StrategyBotID() uint {
	return orderDomain.order.StrategyBotID
}

func (orderDomain ContractAutoOrderDomain) OwnerUserID() uint {
	return orderDomain.order.OwnerUserID
}

func (orderDomain ContractAutoOrderDomain) Symbol() string {
	return orderDomain.order.Symbol
}

// IsDispatchableAt is a ready order whose wait is over, or one whose holder's claim ran out, which is how an order cut short by a dying replica is resumed.
func (orderDomain ContractAutoOrderDomain) IsDispatchableAt(now time.Time) bool {
	switch vo.ContractAutoOrderStatusVo(orderDomain.order.Status) {
	case vo.ContractAutoOrderReady:
		return !orderDomain.order.NextAttemptAt.After(now)
	case vo.ContractAutoOrderExecuting:
		claimedUntil := orderDomain.order.ClaimedUntil

		return claimedUntil == nil || !claimedUntil.After(now)
	}

	return false
}

// ClaimedBy is the order as the claim just left it.
func (orderDomain ContractAutoOrderDomain) ClaimedBy(claimant string, claimedUntil time.Time) ContractAutoOrderDomain {
	claimedUntilUtc := claimedUntil.UTC()
	orderDomain.order.Status = string(vo.ContractAutoOrderExecuting)
	orderDomain.order.ClaimedBy = claimant
	orderDomain.order.ClaimedUntil = &claimedUntilUtc

	return orderDomain
}

// NextStep reads what the bot holds now; a step already done is never asked for again, so an order resumed by another replica picks up where it stopped.
func (orderDomain ContractAutoOrderDomain) NextStep(position vo.AutoOrderPositionVo) ContractAutoOrderStepVo {
	order := orderDomain.order
	target := vo.TargetPositionVo(order.TargetPosition)
	holdsSomething := !orderDomain.isFlat(position)

	if !order.CloseDone && !order.OpenDone {
		if holdsSomething && position.Direction == target {
			return ContractAutoOrderStepSettle
		}
		if !holdsSomething && target == vo.TargetPositionFlat {
			return ContractAutoOrderStepSettle
		}
	}

	if !order.CloseDone && holdsSomething && position.Direction != target {
		return ContractAutoOrderStepClose
	}

	if target != vo.TargetPositionFlat && !order.OpenDone {
		if _, refused := orderDomain.openRefusal(); refused {
			return ContractAutoOrderStepSettle
		}

		return ContractAutoOrderStepOpen
	}

	if order.OpenDone && orderDomain.wantsProtection() && !order.ProtectionDone {
		return ContractAutoOrderStepProtect
	}

	return ContractAutoOrderStepSettle
}

// RefusalToSend names why no new market order may go out now; it is asked only before sending something the venue has no record of, so an order already sent is always followed up.
func (orderDomain ContractAutoOrderDomain) RefusalToSend(
	now time.Time, botRunning bool, autoOrderEnabled bool, hasContractTradingKey bool,
) (ContractAutoOrderDomain, bool) {
	reason := ""

	switch {
	case !autoOrderEnabled:
		reason = "自動下單已關閉，沒有下單"
	case !botRunning:
		reason = "機器人已停止，沒有下單"
	case !hasContractTradingKey:
		reason = "沒有可交易合約的幣安交易金鑰，沒有下單"
	case !now.Before(orderDomain.order.ExpiresAt):
		reason = "訊號已經過時，沒有下單"
	default:
		return orderDomain, false
	}

	// A reverse whose close already went through did something real; saying it gave up would hide that the bot is now flat.
	if orderDomain.order.CloseDone && !orderDomain.order.ClosePositionVanished {
		return orderDomain.withOpenFailure(reason, now), true
	}

	return orderDomain.settledAs(vo.ContractAutoOrderAbandoned, reason, now), true
}

// CloseOrderFor closes no more than the bot opened and no more than the venue still holds on that side; false means nothing is left to close.
func (orderDomain ContractAutoOrderDomain) CloseOrderFor(
	position vo.AutoOrderPositionVo, exchangePosition vo.ContractExchangePositionVo,
) (vo.ContractMarketOrderVo, bool) {
	held := exchangePosition.LongQuantity
	side := vo.ContractOrderSideSell
	if position.Direction == vo.TargetPositionShort {
		held = exchangePosition.ShortQuantity
		side = vo.ContractOrderSideBuy
	}

	quantity := decimal.Min(position.Quantity, held)
	if !quantity.IsPositive() {
		return vo.ContractMarketOrderVo{}, false
	}

	return vo.ContractMarketOrderVo{
		Symbol: orderDomain.order.Symbol, Side: side, Quantity: quantity, ReduceOnly: true,
		ClientOrderID: orderDomain.CloseClientOrderID(),
	}, true
}

// ProtectiveClientIDsOf are the orders guarding the bot's position, which are taken down before it is closed.
func (orderDomain ContractAutoOrderDomain) ProtectiveClientIDsOf(position vo.AutoOrderPositionVo) []string {
	clientIDs := []string{}
	for _, clientID := range []string{position.StopLossClientID, position.TakeProfitClientID} {
		if clientID != "" {
			clientIDs = append(clientIDs, clientID)
		}
	}

	return clientIDs
}

func (orderDomain ContractAutoOrderDomain) CloseClientOrderID() string {
	return orderDomain.clientOrderID("close")
}

func (orderDomain ContractAutoOrderDomain) OpenClientOrderID() string {
	return orderDomain.clientOrderID("open")
}

// AfterClose leaves the bot flat whatever was filled, since it closed everything of its own the venue still held.
func (orderDomain ContractAutoOrderDomain) AfterClose(
	position vo.AutoOrderPositionVo, fill vo.ContractOrderFillVo,
) (ContractAutoOrderDomain, vo.AutoOrderPositionVo) {
	orderDomain.order.CloseDone = true
	orderDomain.order.ClosedDirection = string(position.Direction)
	orderDomain.order.ClosedQuantity = decimal.NewNullDecimal(fill.ExecutedQuantity)
	orderDomain.order.CloseAveragePrice = decimal.NewNullDecimal(fill.AveragePrice)

	return orderDomain, vo.AutoOrderPositionVo{}
}

// AfterCloseVanished is a close that found nothing left at the venue, most likely because a protective order already fired.
func (orderDomain ContractAutoOrderDomain) AfterCloseVanished(
	position vo.AutoOrderPositionVo,
) (ContractAutoOrderDomain, vo.AutoOrderPositionVo) {
	orderDomain.order.CloseDone = true
	orderDomain.order.ClosedDirection = string(position.Direction)
	orderDomain.order.ClosePositionVanished = true

	return orderDomain, vo.AutoOrderPositionVo{}
}

// OpenOrder is the market order that opens the target side.
func (orderDomain ContractAutoOrderDomain) OpenOrder() vo.ContractMarketOrderVo {
	side := vo.ContractOrderSideBuy
	if vo.TargetPositionVo(orderDomain.order.TargetPosition) == vo.TargetPositionShort {
		side = vo.ContractOrderSideSell
	}

	return vo.ContractMarketOrderVo{
		Symbol: orderDomain.order.Symbol, Side: side, Quantity: orderDomain.order.OpenQuantity.Decimal,
		ClientOrderID: orderDomain.OpenClientOrderID(),
	}
}

// OpenLeverage is whole, since the open step is never reached with a fractional one (see NextStep).
func (orderDomain ContractAutoOrderDomain) OpenLeverage() int {
	return int(orderDomain.order.Leverage.IntPart())
}

// AfterOpen works the stop and target out from the actual fill, so the distances the owner set are measured from where he really got in.
func (orderDomain ContractAutoOrderDomain) AfterOpen(
	position vo.AutoOrderPositionVo, fill vo.ContractOrderFillVo, tickSize decimal.Decimal, now time.Time,
) (ContractAutoOrderDomain, vo.AutoOrderPositionVo) {
	order := orderDomain.order
	isShort := vo.TargetPositionVo(order.TargetPosition) == vo.TargetPositionShort
	hundred := decimal.NewFromInt(100)
	openedAt := now.UTC()

	order.OpenDone = true
	order.OpenedQuantity = decimal.NewNullDecimal(fill.ExecutedQuantity)
	order.OpenAveragePrice = decimal.NewNullDecimal(fill.AveragePrice)
	order.OpenedAt = &openedAt

	// Each exit lies on its own side of the fill; a short swaps them. Rounded to the nearest quotable price.
	exits := []struct {
		percentage decimal.Decimal
		below      bool
		price      *decimal.NullDecimal
	}{
		{order.StopLossPercentage, !isShort, &order.StopLossPrice},
		{order.TakeProfitPercentage, isShort, &order.TakeProfitPrice},
	}
	for _, exit := range exits {
		if !exit.percentage.IsPositive() {
			continue
		}

		distance := fill.AveragePrice.Mul(exit.percentage).Div(hundred)
		exitPrice := fill.AveragePrice.Add(distance)
		if exit.below {
			exitPrice = fill.AveragePrice.Sub(distance)
		}
		if tickSize.IsPositive() {
			exitPrice = exitPrice.Div(tickSize).Round(0).Mul(tickSize)
		}
		*exit.price = decimal.NewNullDecimal(exitPrice)
	}

	orderDomain.order = order

	return orderDomain, vo.AutoOrderPositionVo{
		Direction: vo.TargetPositionVo(order.TargetPosition), Quantity: fill.ExecutedQuantity,
		StopLossClientID: position.StopLossClientID, TakeProfitClientID: position.TakeProfitClientID,
	}
}

// ProtectiveOrders lists the stop and target to place, each only when its distance was set; they close the opened quantity and never open the other way.
func (orderDomain ContractAutoOrderDomain) ProtectiveOrders() []vo.ContractProtectiveOrderVo {
	order := orderDomain.order
	side := vo.ContractOrderSideSell
	if vo.TargetPositionVo(order.TargetPosition) == vo.TargetPositionShort {
		side = vo.ContractOrderSideBuy
	}

	protectiveOrders := []vo.ContractProtectiveOrderVo{}
	if order.StopLossPrice.Valid {
		protectiveOrders = append(protectiveOrders, vo.ContractProtectiveOrderVo{
			Symbol: order.Symbol, Side: side, Kind: vo.ContractProtectiveOrderStopLoss,
			TriggerPrice: order.StopLossPrice.Decimal, Quantity: order.OpenedQuantity.Decimal,
			ClientOrderID: orderDomain.clientOrderID("sl"),
		})
	}
	if order.TakeProfitPrice.Valid {
		protectiveOrders = append(protectiveOrders, vo.ContractProtectiveOrderVo{
			Symbol: order.Symbol, Side: side, Kind: vo.ContractProtectiveOrderTakeProfit,
			TriggerPrice: order.TakeProfitPrice.Decimal, Quantity: order.OpenedQuantity.Decimal,
			ClientOrderID: orderDomain.clientOrderID("tp"),
		})
	}

	return protectiveOrders
}

// MayKeepTryingProtectionAt is whether a protective order that could not get through is still worth another attempt rather than handing it to the owner.
func (orderDomain ContractAutoOrderDomain) MayKeepTryingProtectionAt(now time.Time) bool {
	openedAt := orderDomain.order.OpenedAt
	if openedAt == nil {
		return false
	}

	return now.Before(openedAt.Add(contractAutoOrderProtectionWindow))
}

// AfterProtection records which protective orders are in place and hands their ids to the bot's position, so its next close takes them down.
// An unconfirmed one counts as missing, since the owner cannot rely on it, but its id is kept too: if the venue did take it, the next close must still take it down.
func (orderDomain ContractAutoOrderDomain) AfterProtection(
	position vo.AutoOrderPositionVo, placed map[vo.ContractProtectiveOrderKindVo]bool,
	unconfirmed map[vo.ContractProtectiveOrderKindVo]bool,
) (ContractAutoOrderDomain, vo.AutoOrderPositionVo) {
	order := orderDomain.order
	order.StopLossPlaced = order.StopLossPrice.Valid && placed[vo.ContractProtectiveOrderStopLoss]
	order.TakeProfitPlaced = order.TakeProfitPrice.Valid && placed[vo.ContractProtectiveOrderTakeProfit]
	order.ProtectionMissing = (order.StopLossPrice.Valid && !order.StopLossPlaced) ||
		(order.TakeProfitPrice.Valid && !order.TakeProfitPlaced)
	order.ProtectionDone = true

	position.StopLossClientID = ""
	if order.StopLossPlaced || (order.StopLossPrice.Valid && unconfirmed[vo.ContractProtectiveOrderStopLoss]) {
		position.StopLossClientID = orderDomain.clientOrderID("sl")
	}
	position.TakeProfitClientID = ""
	if order.TakeProfitPlaced || (order.TakeProfitPrice.Valid && unconfirmed[vo.ContractProtectiveOrderTakeProfit]) {
		position.TakeProfitClientID = orderDomain.clientOrderID("tp")
	}

	orderDomain.order = order

	return orderDomain, position
}

// Rescheduled hands the order back to wait, so whichever replica takes it next asks the venue before sending anything again.
func (orderDomain ContractAutoOrderDomain) Rescheduled(now time.Time) ContractAutoOrderDomain {
	orderDomain.order.Status = string(vo.ContractAutoOrderReady)
	orderDomain.order.AttemptCount++
	orderDomain.order.NextAttemptAt = now.UTC().Add(contractAutoOrderRetryWait)
	orderDomain.order.ClaimedBy = ""
	orderDomain.order.ClaimedUntil = nil

	return orderDomain
}

// Settled ends an order that reached NextStep's settle: everything it set out to do is done, or there was nothing it could do.
func (orderDomain ContractAutoOrderDomain) Settled(position vo.AutoOrderPositionVo, now time.Time) ContractAutoOrderDomain {
	order := orderDomain.order
	target := vo.TargetPositionVo(order.TargetPosition)

	if !order.CloseDone && !order.OpenDone {
		if !orderDomain.isFlat(position) && position.Direction == target {
			return orderDomain.settledAs(vo.ContractAutoOrderNotPlaced,
				fmt.Sprintf("已經持有%s倉，不加倉", orderDomain.directionInWords(target)), now)
		}
		if orderDomain.isFlat(position) && target == vo.TargetPositionFlat {
			return orderDomain.settledAs(vo.ContractAutoOrderNotPlaced, "沒有自動開出的倉位，不需要平倉", now)
		}
	}

	if target != vo.TargetPositionFlat && !order.OpenDone {
		if reason, refused := orderDomain.openRefusal(); refused {
			return orderDomain.withOpenFailure(reason, now)
		}
	}

	vanishedNote := ""
	if order.ClosePositionVanished {
		vanishedNote = "幣安上已沒有這筆倉位（可能已觸發止損或止盈）"
	}
	if !order.OpenDone && order.ClosePositionVanished {
		return orderDomain.settledAs(vo.ContractAutoOrderNotPlaced, vanishedNote, now)
	}

	return orderDomain.settledAs(vo.ContractAutoOrderFilled, vanishedNote, now)
}

// SettledByFailure ends the order on the venue's refusal and says which of the owner's bots must have auto order switched off: all of them, only contract bots, or none.
func (orderDomain ContractAutoOrderDomain) SettledByFailure(
	call vo.ContractOrderCallVo, now time.Time,
) (ContractAutoOrderDomain, []string, bool) {
	switch call.Failure {
	case vo.ContractOrderFailureKeyRejected:
		return orderDomain.withOpenFailure(
				"幣安不接受你的交易金鑰，已關掉你所有機器人的自動下單，請重存金鑰", now),
			[]string{}, true
	case vo.ContractOrderFailureNoContractPermission:
		return orderDomain.withOpenFailure(
				"你的幣安交易金鑰沒有合約交易權限，已關掉你所有合約機器人的自動下單，請到幣安開權限後重存金鑰", now),
			[]string{string(vo.MarketDataKindContractKCandle)}, true
	case vo.ContractOrderFailureInsufficientBalance:
		return orderDomain.withOpenFailure("餘額不足", now), nil, false
	case vo.ContractOrderFailureVenueRefused:
		return orderDomain.withOpenFailure("交易所不收這一筆："+call.ExchangeMessage, now), nil, false
	case vo.ContractOrderFailureAccountSettingRefused:
		return orderDomain.withOpenFailure("幣安帳戶設定不符："+call.ExchangeMessage, now), nil, false
	}

	return orderDomain.withOpenFailure("幣安拒絕："+call.ExchangeMessage, now), nil, false
}

// SettledInHedgeMode ends an order on an account holding long and short separately, which the bot's single-position acts do not fit.
func (orderDomain ContractAutoOrderDomain) SettledInHedgeMode(now time.Time) ContractAutoOrderDomain {
	return orderDomain.withOpenFailure("請先在幣安把持倉模式改成單向持倉", now)
}

// SettledUnsealable ends an order whose trading key this system cannot open, which no retry would fix.
func (orderDomain ContractAutoOrderDomain) SettledUnsealable(now time.Time) ContractAutoOrderDomain {
	return orderDomain.withOpenFailure("系統目前無法開啟你的幣安交易金鑰", now)
}

// ToResultDto names the act from what was done, so a reverse reads as one act rather than two.
func (orderDomain ContractAutoOrderDomain) ToResultDto() dto.ContractAutoOrderResultDto {
	return orderDomain.order.ToResultDto(orderDomain.ActionInWords())
}

// ActionInWords is the act the order carried out, or the one it meant to when it did nothing.
func (orderDomain ContractAutoOrderDomain) ActionInWords() string {
	order := orderDomain.order
	target := vo.TargetPositionVo(order.TargetPosition)
	closedSomething := order.CloseDone && !order.ClosePositionVanished

	switch {
	case closedSomething && order.OpenDone:
		return "反手做" + orderDomain.directionInWords(target)
	case order.OpenDone:
		return "做" + orderDomain.directionInWords(target)
	case closedSomething:
		return "平" + orderDomain.directionInWords(vo.TargetPositionVo(order.ClosedDirection))
	case target == vo.TargetPositionFlat:
		return "平倉"
	}

	return "做" + orderDomain.directionInWords(target)
}

// MissingProtectionInWords names the protective orders that should be in place and are not; empty when nothing is missing.
func (orderDomain ContractAutoOrderDomain) MissingProtectionInWords() string {
	order := orderDomain.order
	missing := []string{}
	if order.StopLossPrice.Valid && !order.StopLossPlaced {
		missing = append(missing, "止損")
	}
	if order.TakeProfitPrice.Valid && !order.TakeProfitPlaced {
		missing = append(missing, "止盈")
	}

	return strings.Join(missing, "與")
}

func (orderDomain ContractAutoOrderDomain) Outcome() vo.ContractAutoOrderOutcomeVo {
	return vo.ContractAutoOrderOutcomeVo(orderDomain.order.Outcome)
}

// withOpenFailure keeps a reverse honest: when the close already went through, the order is partly done and says so.
func (orderDomain ContractAutoOrderDomain) withOpenFailure(reason string, now time.Time) ContractAutoOrderDomain {
	order := orderDomain.order
	if order.CloseDone && !order.ClosePositionVanished && !order.OpenDone {
		return orderDomain.settledAs(vo.ContractAutoOrderPartiallyDone,
			fmt.Sprintf("已平倉，但做%s沒開成：%s",
				orderDomain.directionInWords(vo.TargetPositionVo(order.TargetPosition)), reason), now)
	}

	return orderDomain.settledAs(vo.ContractAutoOrderNotPlaced, reason, now)
}

func (orderDomain ContractAutoOrderDomain) settledAs(
	outcome vo.ContractAutoOrderOutcomeVo, reason string, now time.Time,
) ContractAutoOrderDomain {
	settledAt := now.UTC()
	orderDomain.order.Status = string(vo.ContractAutoOrderSettled)
	orderDomain.order.Outcome = string(outcome)
	orderDomain.order.Reason = reason
	orderDomain.order.ClaimedBy = ""
	orderDomain.order.ClaimedUntil = nil
	orderDomain.order.SettledAt = &settledAt

	return orderDomain
}

// openRefusal is why the target side cannot be opened at all: nothing the venue would accept, or a leverage the venue cannot be set to.
func (orderDomain ContractAutoOrderDomain) openRefusal() (string, bool) {
	order := orderDomain.order
	if !order.OpenQuantity.Valid {
		return order.NoOpenReason, true
	}
	if !order.Leverage.Equal(order.Leverage.Truncate(0)) {
		return "交易所不收這一筆：幣安只接受整數槓桿", true
	}

	return "", false
}

func (orderDomain ContractAutoOrderDomain) wantsProtection() bool {
	return orderDomain.order.StopLossPrice.Valid || orderDomain.order.TakeProfitPrice.Valid
}

func (orderDomain ContractAutoOrderDomain) isFlat(position vo.AutoOrderPositionVo) bool {
	return (position.Direction != vo.TargetPositionLong && position.Direction != vo.TargetPositionShort) ||
		!position.Quantity.IsPositive()
}

func (orderDomain ContractAutoOrderDomain) clientOrderID(step string) string {
	return fmt.Sprintf("%s%d-%s", contractAutoOrderClientIDPrefix, orderDomain.order.ID, step)
}

func (orderDomain ContractAutoOrderDomain) directionInWords(direction vo.TargetPositionVo) string {
	if direction == vo.TargetPositionShort {
		return "空"
	}

	return "多"
}
