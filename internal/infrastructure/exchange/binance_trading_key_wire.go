package exchange

// binanceApiRestrictionsResponse keeps only the two trading permissions this system records; withdrawal and other permissions are deliberately not read.
type binanceApiRestrictionsResponse struct {
	EnableSpotAndMarginTrading bool `json:"enableSpotAndMarginTrading"`
	EnableFutures              bool `json:"enableFutures"`
}

// binanceErrorResponse is Binance's error body; Code is the only reliable way to tell a rejected key from a refused request.
type binanceErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"msg"`
}

// rejectedKeyCodes blame the key itself: malformed key, unknown key or missing permission for the call, bad signature, unknown key id.
var rejectedKeyCodes = map[int]bool{
	-2014: true,
	-2015: true,
	-1022: true,
	-2008: true,
}

// BlamesTheKey returns false for unfamiliar codes, so the caller reports unreachable rather than sending someone to regenerate a working key.
func (binanceErrorResponse binanceErrorResponse) BlamesTheKey() bool {
	return rejectedKeyCodes[binanceErrorResponse.Code]
}
