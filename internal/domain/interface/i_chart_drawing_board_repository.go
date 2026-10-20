package _interface

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
)

//go:generate go tool mockgen -source=i_chart_drawing_board_repository.go -destination=mocks/mock_i_chart_drawing_board_repository.go -package=mocks

// IChartDrawingBoardRepository keeps one row per owner, market and symbol; every write is conditional on the revision it was based on.
type IChartDrawingBoardRepository interface {
	// FindOne reports found=false for a board never drawn on.
	FindOne(executionContext context.Context, ownerID uint, market string, symbol string) (entities.ChartDrawingBoard, bool, error)
	// Save writes only if the stored revision is still expectedRevision (zero: no row yet), otherwise it reports a revision conflict and writes nothing.
	Save(executionContext context.Context, board entities.ChartDrawingBoard, expectedRevision int64) (entities.ChartDrawingBoard, error)
}
