package vo

// FollowTargetVo is one market being followed live: which symbol, and which market
// it belongs to.
//
// The market travels with the symbol for the same reason it travels with a fetch
// window — so that which source answers can be read off the request, and following a
// second venue costs the contract nothing.
//
// Immutable, no behavior.
type FollowTargetVo struct {
	Symbol string
	Market MarketVo
}
