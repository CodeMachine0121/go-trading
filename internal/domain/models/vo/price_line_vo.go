package vo

// PriceLineVo is one OHLC line; a perpetual contract has three besides its traded prices: mark, index and premium index.
type PriceLineVo struct {
	Open  float64
	High  float64
	Low   float64
	Close float64
}
