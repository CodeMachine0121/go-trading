package vo

// MarketVo is which market a trading symbol belongs to. It is what decides where
// its candles are fetched from and which rules it runs by — never the spelling of
// the symbol itself, because Taiwan stock has letter-and-digit codes and crypto
// puts no shape on its names at all, so guessing from the name is a rule that will
// one day be wrong without anybody noticing.
//
// Immutable, no behavior — reading one, defaulting it, and everything it implies
// about opening hours and follow ceilings lives in MarketDomain.
type MarketVo string

const (
	// MarketCrypto is the round-the-clock market this system started with. It is
	// also what an unrecognised or absent market means, so rows written before a
	// market was ever recorded keep behaving exactly as they did.
	MarketCrypto MarketVo = "crypto"
	// MarketTaiwanStock is the Taiwan listed-share market: it closes, its source
	// follows only a few symbols at once, and it reports fewer volume figures.
	MarketTaiwanStock MarketVo = "taiwanStock"
)
