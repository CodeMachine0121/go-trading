package vo

// OrderFlowFootprintVo is one bar's trades tallied by price level, as a script reads it (named Footprint there); it is the zero value when the script declared no order flow price step.
type OrderFlowFootprintVo struct {
	// PriceStep is the width this run tallied at; zero means no order flow was asked for.
	PriceStep float64
	// Levels holds only levels that traded, lowest price first.
	Levels []OrderFlowPriceLevelVo
	// Delta is all buyer-initiated volume minus all seller-initiated volume; it may be negative.
	Delta float64
	// PointOfControl is the level with the most volume both ways, the lower one on a tie; zero when nothing traded.
	PointOfControl float64
}
