package persistence

import (
	"context"
	"errors"
	"fmt"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/jackc/pgx/v5/pgconn"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const ChartDrawingBoardOwnerMarketSymbolIndex = "idx_chart_drawing_boards_owner_market_symbol"

type ChartDrawingBoardRepository struct {
	database *gorm.DB
}

func NewChartDrawingBoardRepository(database *gorm.DB) *ChartDrawingBoardRepository {
	return &ChartDrawingBoardRepository{database: database}
}

func (chartDrawingBoardRepository *ChartDrawingBoardRepository) FindOne(
	executionContext context.Context, ownerID uint, market string, symbol string,
) (entities.ChartDrawingBoard, bool, error) {
	board := entities.ChartDrawingBoard{}

	result := chartDrawingBoardRepository.database.WithContext(executionContext).
		Where(clause.Eq{Column: "owner_id", Value: ownerID}).
		Where(clause.Eq{Column: "market", Value: market}).
		Where(clause.Eq{Column: "symbol", Value: symbol}).
		Limit(1).
		Find(&board)
	if result.Error != nil {
		return entities.ChartDrawingBoard{}, false, fmt.Errorf("find chart drawing board: %w", result.Error)
	}

	return board, result.RowsAffected > 0, nil
}

// Save inserts a first revision, where a concurrent first insert breaks the board index, and otherwise updates only the row still at expectedRevision.
func (chartDrawingBoardRepository *ChartDrawingBoardRepository) Save(
	executionContext context.Context, board entities.ChartDrawingBoard, expectedRevision int64,
) (entities.ChartDrawingBoard, error) {
	board.Owner = entities.User{}

	if expectedRevision == 0 {
		board.ID = 0
		createError := chartDrawingBoardRepository.database.WithContext(executionContext).
			Omit(clause.Associations).Create(&board).Error
		postgresError, isPostgresError := errors.AsType[*pgconn.PgError](createError)
		if isPostgresError && postgresError.Code == uniqueViolationCode &&
			postgresError.ConstraintName == ChartDrawingBoardOwnerMarketSymbolIndex {
			return entities.ChartDrawingBoard{}, domains.ErrChartDrawingBoardRevisionConflict
		}
		if createError != nil {
			return entities.ChartDrawingBoard{}, fmt.Errorf("create chart drawing board: %w", createError)
		}

		return board, nil
	}

	result := chartDrawingBoardRepository.database.WithContext(executionContext).
		Model(&entities.ChartDrawingBoard{}).
		Where(clause.Eq{Column: "id", Value: board.ID}).
		Where(clause.Eq{Column: "revision", Value: expectedRevision}).
		Select("revision", "last_drawing_id", "drawings", "updated_at").
		Updates(&board)
	if result.Error != nil {
		return entities.ChartDrawingBoard{}, fmt.Errorf("update chart drawing board: %w", result.Error)
	}
	if result.RowsAffected == 0 {
		return entities.ChartDrawingBoard{}, domains.ErrChartDrawingBoardRevisionConflict
	}

	return board, nil
}
