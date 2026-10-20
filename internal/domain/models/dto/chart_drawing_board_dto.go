package dto

// ChartDrawingBoardPlaceDto names one board: whose, which market, which symbol.
type ChartDrawingBoardPlaceDto struct {
	OwnerID uint
	Market  string
	Symbol  string
}

// ChartDrawingBoardDto lists drawings bottom first; a board never drawn on has revision zero and no drawings.
type ChartDrawingBoardDto struct {
	Market   string            `json:"market"`
	Symbol   string            `json:"symbol"`
	Revision int64             `json:"revision"`
	Drawings []ChartDrawingDto `json:"drawings"`
}

// ChartDrawingBoardReplaceDto carries the revision the drawings were based on, so a board changed since is never overwritten.
type ChartDrawingBoardReplaceDto struct {
	Place    ChartDrawingBoardPlaceDto
	Revision int64
	Drawings []ChartDrawingWriteDto
}
