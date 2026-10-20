package persistence_test

import (
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func aStoredBoardDrawing(id uint, priceText string) dto.ChartDrawingDto {
	turnID := uint(34)

	return dto.ChartDrawingDto{
		ID: id, Kind: "horizontalLine",
		Anchors: []dto.ChartDrawingAnchorDto{{
			OpenTime: time.Date(2026, 10, 19, 8, 0, 0, 0, time.UTC), Price: decimal.RequireFromString(priceText),
		}},
		Color: "blue", LineWidth: 1, LineStyle: "solid", Visible: true, Source: "assistant",
		AssistantConversationID: &turnID, AssistantTurnID: &turnID,
	}
}

func TestChartDrawingBoardRepositoryStoresABoardWhole(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewChartDrawingBoardRepository(database)

	_, found, findError := repository.FindOne(t.Context(), owner.ID, "spot", "BTCUSDT")
	require.NoError(t, findError)
	assert.False(t, found, "a board never drawn on is not there")

	created, createError := repository.Save(t.Context(), entities.ChartDrawingBoard{
		OwnerID: owner.ID, Market: "spot", Symbol: "BTCUSDT", Revision: 1, LastDrawingID: 1,
		Drawings: []dto.ChartDrawingDto{aStoredBoardDrawing(1, "0.00012345")},
	}, 0)
	require.NoError(t, createError)
	assert.Positive(t, created.ID)

	stored, found, findError := repository.FindOne(t.Context(), owner.ID, "spot", "BTCUSDT")
	require.NoError(t, findError)
	require.True(t, found)
	assert.Equal(t, int64(1), stored.Revision)
	assert.Equal(t, uint(1), stored.LastDrawingID)
	require.Len(t, stored.Drawings, 1)
	assert.Equal(t, "0.00012345", stored.Drawings[0].Anchors[0].Price.String())
	assert.Equal(t, uint(34), *stored.Drawings[0].AssistantTurnID)

	_, contractFound, contractFindError := repository.FindOne(t.Context(), owner.ID, "contract", "BTCUSDT")
	require.NoError(t, contractFindError)
	assert.False(t, contractFound, "the contract board of the same symbol is another board")
}

func TestChartDrawingBoardRepositoryWritesOnlyOverTheExpectedRevision(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")
	repository := persistence.NewChartDrawingBoardRepository(database)
	created, createError := repository.Save(t.Context(), entities.ChartDrawingBoard{
		OwnerID: owner.ID, Market: "spot", Symbol: "BTCUSDT", Revision: 1, LastDrawingID: 1,
		Drawings: []dto.ChartDrawingDto{aStoredBoardDrawing(1, "100")},
	}, 0)
	require.NoError(t, createError)

	t.Run("a second first insert is a revision conflict", func(t *testing.T) {
		_, duplicateError := repository.Save(t.Context(), entities.ChartDrawingBoard{
			OwnerID: owner.ID, Market: "spot", Symbol: "BTCUSDT", Revision: 1, Drawings: []dto.ChartDrawingDto{},
		}, 0)

		assert.ErrorIs(t, duplicateError, domains.ErrChartDrawingBoardRevisionConflict)
	})

	t.Run("a write based on the stored revision lands", func(t *testing.T) {
		updated := created
		updated.Revision = 2
		updated.LastDrawingID = 2
		updated.Drawings = []dto.ChartDrawingDto{aStoredBoardDrawing(2, "200")}

		_, updateError := repository.Save(t.Context(), updated, 1)
		require.NoError(t, updateError)

		stored, _, findError := repository.FindOne(t.Context(), owner.ID, "spot", "BTCUSDT")
		require.NoError(t, findError)
		assert.Equal(t, int64(2), stored.Revision)
		assert.Equal(t, uint(2), stored.LastDrawingID)
		assert.Equal(t, "200", stored.Drawings[0].Anchors[0].Price.String())
	})

	t.Run("a write based on an old revision changes nothing", func(t *testing.T) {
		stale := created
		stale.Revision = 2
		stale.Drawings = []dto.ChartDrawingDto{}

		_, staleError := repository.Save(t.Context(), stale, 1)

		assert.ErrorIs(t, staleError, domains.ErrChartDrawingBoardRevisionConflict)
		stored, _, findError := repository.FindOne(t.Context(), owner.ID, "spot", "BTCUSDT")
		require.NoError(t, findError)
		assert.Len(t, stored.Drawings, 1)
	})

	t.Run("another person's board of the same symbol is their own", func(t *testing.T) {
		otherOwner := aDeliveryOwner(t, database, "someone@example.com")

		_, otherCreateError := repository.Save(t.Context(), entities.ChartDrawingBoard{
			OwnerID: otherOwner.ID, Market: "spot", Symbol: "BTCUSDT", Revision: 1, Drawings: []dto.ChartDrawingDto{},
		}, 0)

		require.NoError(t, otherCreateError)
		stored, _, findError := repository.FindOne(t.Context(), owner.ID, "spot", "BTCUSDT")
		require.NoError(t, findError)
		assert.Len(t, stored.Drawings, 1)
	})
}
