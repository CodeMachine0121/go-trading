package marketdata

import "strings"

// fubonProductsAnswer is the shape this source answers "which contracts are listed"
// with. Only the parts needed to pick one are read; whatever else it sends stops here.
type fubonProductsAnswer struct {
	Data []fubonProduct `json:"data"`
}

// fubonProduct is one listed futures contract as this source states it: the code it
// trades under, and what the venue calls it.
type fubonProduct struct {
	Symbol string `json:"symbol"`
	Name   string `json:"name"`
}

// futuresMonthCodes are the letters this venue spells a contract's delivery month
// with, January first. They are the venue's own notation, which is why reading them
// belongs here and nowhere further in.
const futuresMonthCodes = "ABCDEFGHIJKL"

// monthsUntilDelivery is how far out this contract's delivery month is, counted in
// months from the year nought, and whether the symbol is a contract of the standing
// code at all.
//
// It exists so that "which of these expires next" can be answered by ordering, and
// the answer is the smallest. **That is not the same as working out settlement dates:**
// a contract stops being listed once it has settled, so the nearest listed month
// changes by itself, on the day the venue says so and not on a day this system
// calculated. That is exactly what "roll once settlement is done" asks for.
//
// The reference year resolves the single digit the venue writes years as. A digit
// stands for the year with that last digit nearest to now, which is unambiguous while
// contracts run a year or two out — and keeps reading right across the turn of a
// decade, where comparing digits alone would put 2030 before 2029.
func (fubonProduct fubonProduct) monthsUntilDelivery(
	standingSymbol string, referenceYear int,
) (int, bool) {
	if !strings.HasPrefix(fubonProduct.Symbol, standingSymbol) {
		return 0, false
	}

	deliveryCode := fubonProduct.Symbol[len(standingSymbol):]
	if len(deliveryCode) != 2 {
		return 0, false
	}

	monthIndex := strings.IndexByte(futuresMonthCodes, deliveryCode[0])
	if monthIndex < 0 {
		return 0, false
	}

	yearDigit := int(deliveryCode[1] - '0')
	if yearDigit < 0 || yearDigit > 9 {
		return 0, false
	}

	deliveryYear := referenceYear/10*10 + yearDigit
	if deliveryYear-referenceYear > 5 {
		deliveryYear -= 10
	}
	if referenceYear-deliveryYear > 5 {
		deliveryYear += 10
	}

	return deliveryYear*12 + monthIndex, true
}
