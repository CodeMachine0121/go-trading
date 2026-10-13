package domains

import "errors"

// ErrContractOrderFlowRefused is a request for order flow that can never be met as asked (too long a stretch, too many price levels); retrying the same thing will not help.
var ErrContractOrderFlowRefused = errors.New("contract order flow refused")

// ErrContractOrderFlowUnavailable means the venue's trade history could not be read; trying again later may work.
var ErrContractOrderFlowUnavailable = errors.New("讀不到訂單流")

// ErrContractOrderFlowNotReady means the order flow was not gathered within the time allowed; what was gathered is kept, so a later attempt continues from there.
var ErrContractOrderFlowNotReady = errors.New("訂單流還沒備齊")
