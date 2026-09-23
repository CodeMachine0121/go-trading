package vo

// PriceLineVo is one line of prices as an indicator script sees it — an open, a
// high, a low and a close. A perpetual contract carries three of them beside its
// traded prices: the mark price, the index price and the premium index. Immutable
// plain data, no behavior.
type PriceLineVo struct {
	Open  float64
	High  float64
	Low   float64
	Close float64
}
