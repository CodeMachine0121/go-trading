package domains

import (
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

var ErrChartDrawingValidation = errors.New("chart drawing validation failed")

// ErrChartDrawingNotFound is scoped to one board; another person's board is never read.
var ErrChartDrawingNotFound = errors.New("chart drawing not found")

// ErrChartDrawingHandDrawn keeps the assistant from removing anything the person drew.
var ErrChartDrawingHandDrawn = errors.New("chart drawing drawn by hand")

var ErrChartDrawingBoardRevisionConflict = errors.New("chart drawing board revision conflict")

// ChartDrawingBoardRevisionConflictError carries the board as it is now, so the caller can reread and reapply without another request.
type ChartDrawingBoardRevisionConflictError struct {
	CurrentBoard dto.ChartDrawingBoardDto
}

func (revisionConflictError ChartDrawingBoardRevisionConflictError) Error() string {
	return fmt.Sprintf("%s: 繪圖板在你讀取之後已經變了（現在是第 %d 版），請重新讀取後再存",
		ErrChartDrawingBoardRevisionConflict, revisionConflictError.CurrentBoard.Revision)
}

func (revisionConflictError ChartDrawingBoardRevisionConflictError) Unwrap() error {
	return ErrChartDrawingBoardRevisionConflict
}
