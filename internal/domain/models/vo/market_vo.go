package vo

// MarketVo is which market a symbol belongs to, decided explicitly rather than guessed from the symbol's spelling; see MarketDomain.
type MarketVo string

const (
	// MarketCrypto is the original round-the-clock market and also what an absent or unknown market means.
	MarketCrypto MarketVo = "crypto"
	// MarketTaiwanStock closes, follows only a few symbols at once, and reports fewer volume figures.
	MarketTaiwanStock MarketVo = "taiwanStock"
)
