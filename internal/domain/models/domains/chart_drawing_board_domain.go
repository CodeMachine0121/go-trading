package domains

import (
	"fmt"
	"slices"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// ChartDrawingBoardDomain is one person's drawings on one market and symbol; it alone decides who may change which drawing.
type ChartDrawingBoardDomain struct {
	place         ChartDrawingPlaceDomain
	boardID       uint
	revision      int64
	lastDrawingID uint
	drawings      []dto.ChartDrawingDto
	limits        vo.ChartDrawingLimitsVo
}

// NewChartDrawingBoardDomain takes the stored row, or a zero row for a board never drawn on.
func NewChartDrawingBoardDomain(
	place ChartDrawingPlaceDomain, storedBoard entities.ChartDrawingBoard, limits vo.ChartDrawingLimitsVo,
) ChartDrawingBoardDomain {
	return ChartDrawingBoardDomain{
		place:         place,
		boardID:       storedBoard.ID,
		revision:      storedBoard.Revision,
		lastDrawingID: storedBoard.LastDrawingID,
		drawings:      append([]dto.ChartDrawingDto{}, storedBoard.Drawings...),
		limits:        limits,
	}
}

func (chartDrawingBoardDomain ChartDrawingBoardDomain) Revision() int64 {
	return chartDrawingBoardDomain.revision
}

// Replace takes the whole board the person saved; a drawing keeping a known number keeps who drew it, and every other one is newly hand drawn.
func (chartDrawingBoardDomain ChartDrawingBoardDomain) Replace(
	basedOnRevision int64, writeDtos []dto.ChartDrawingWriteDto,
) (ChartDrawingBoardDomain, error) {
	if basedOnRevision != chartDrawingBoardDomain.revision {
		return ChartDrawingBoardDomain{}, ChartDrawingBoardRevisionConflictError{
			CurrentBoard: chartDrawingBoardDomain.ToDto(),
		}
	}

	if len(writeDtos) > chartDrawingBoardDomain.limits.BoardCapacity {
		return ChartDrawingBoardDomain{}, fmt.Errorf("%w: 一份繪圖板最多 %d 個繪圖，收到 %d 個",
			ErrChartDrawingValidation, chartDrawingBoardDomain.limits.BoardCapacity, len(writeDtos))
	}

	provenanceByID := make(map[uint]vo.ChartDrawingProvenanceVo, len(chartDrawingBoardDomain.drawings))
	for _, storedDrawing := range chartDrawingBoardDomain.drawings {
		provenanceByID[storedDrawing.ID] = vo.ChartDrawingProvenanceVo{
			Source:                  vo.ChartDrawingSourceVo(storedDrawing.Source),
			AssistantConversationID: storedDrawing.AssistantConversationID,
			AssistantTurnID:         storedDrawing.AssistantTurnID,
		}
	}

	replaced := chartDrawingBoardDomain
	replaced.drawings = make([]dto.ChartDrawingDto, 0, len(writeDtos))
	for position, writeDto := range writeDtos {
		provenance, keepsKnownDrawing := provenanceByID[writeDto.ID]
		if !keepsKnownDrawing {
			provenance = vo.ChartDrawingProvenanceVo{Source: vo.ChartDrawingSourceManual}
		}

		drawing, drawingError := NewChartDrawingDomain(writeDto.Kind, writeDto.Anchors, writeDto.Style, provenance)
		if drawingError != nil {
			return ChartDrawingBoardDomain{}, fmt.Errorf(
				"%w: 第 %d 個繪圖：%w", ErrChartDrawingValidation, position+1, drawingError)
		}

		drawingID := writeDto.ID
		if !keepsKnownDrawing {
			replaced.lastDrawingID++
			drawingID = replaced.lastDrawingID
		}
		// A number listed twice keeps its drawing only the first time.
		delete(provenanceByID, writeDto.ID)

		replaced.drawings = append(replaced.drawings, drawing.ToDto(drawingID))
	}

	if anchorCount := replaced.anchorCount(); anchorCount > replaced.limits.BoardAnchorCapacity {
		return ChartDrawingBoardDomain{}, fmt.Errorf("%w: 一份繪圖板的繪圖最多共 %d 個錨點，收到 %d 個",
			ErrChartDrawingValidation, replaced.limits.BoardAnchorCapacity, anchorCount)
	}
	replaced.revision++

	return replaced, nil
}

// AppendAssistantDrawings puts the whole batch on top or none of it, naming the free places left when it does not fit.
func (chartDrawingBoardDomain ChartDrawingBoardDomain) AppendAssistantDrawings(
	drawings []ChartDrawingDomain,
) (ChartDrawingBoardDomain, []dto.ChartDrawingDto, error) {
	freePlaces := chartDrawingBoardDomain.limits.BoardCapacity - len(chartDrawingBoardDomain.drawings)
	if len(drawings) > freePlaces {
		return ChartDrawingBoardDomain{}, nil, fmt.Errorf(
			"%w: 一份繪圖板最多 %d 個繪圖，這份只剩 %d 個空位，這一批有 %d 個",
			ErrChartDrawingValidation, chartDrawingBoardDomain.limits.BoardCapacity, max(freePlaces, 0), len(drawings))
	}

	freeAnchors := chartDrawingBoardDomain.limits.BoardAnchorCapacity - chartDrawingBoardDomain.anchorCount()
	batchAnchorCount := 0
	for _, drawing := range drawings {
		batchAnchorCount += len(drawing.anchors)
	}
	if batchAnchorCount > freeAnchors {
		return ChartDrawingBoardDomain{}, nil, fmt.Errorf(
			"%w: 一份繪圖板的繪圖最多共 %d 個錨點，這份只剩 %d 個錨點的空間，這一批有 %d 個",
			ErrChartDrawingValidation, chartDrawingBoardDomain.limits.BoardAnchorCapacity, max(freeAnchors, 0), batchAnchorCount)
	}

	appended := chartDrawingBoardDomain
	appended.drawings = slices.Clone(chartDrawingBoardDomain.drawings)
	addedDrawings := make([]dto.ChartDrawingDto, 0, len(drawings))
	for _, drawing := range drawings {
		appended.lastDrawingID++
		addedDrawing := drawing.ToDto(appended.lastDrawingID)
		appended.drawings = append(appended.drawings, addedDrawing)
		addedDrawings = append(addedDrawings, addedDrawing)
	}
	appended.revision++

	return appended, addedDrawings, nil
}

// RemoveAssistantDrawings removes every assistant drawing when none are named; naming a hand drawing or an unknown one removes nothing.
// Nothing removed leaves the revision where it was.
func (chartDrawingBoardDomain ChartDrawingBoardDomain) RemoveAssistantDrawings(
	drawingIDs []uint,
) (ChartDrawingBoardDomain, []uint, error) {
	drawingsByID := make(map[uint]dto.ChartDrawingDto, len(chartDrawingBoardDomain.drawings))
	for _, drawing := range chartDrawingBoardDomain.drawings {
		drawingsByID[drawing.ID] = drawing
	}

	for _, drawingID := range drawingIDs {
		drawing, exists := drawingsByID[drawingID]
		if !exists {
			return ChartDrawingBoardDomain{}, nil, fmt.Errorf(
				"%w: 這份繪圖板上沒有編號 %d 的繪圖", ErrChartDrawingNotFound, drawingID)
		}
		if drawing.Source != string(vo.ChartDrawingSourceAssistant) {
			return ChartDrawingBoardDomain{}, nil, fmt.Errorf(
				"%w: 編號 %d 的繪圖是使用者手畫的，助手不能移除", ErrChartDrawingHandDrawn, drawingID)
		}
	}

	remaining := chartDrawingBoardDomain
	remaining.drawings = make([]dto.ChartDrawingDto, 0, len(chartDrawingBoardDomain.drawings))
	removedDrawingIDs := make([]uint, 0)
	for _, drawing := range chartDrawingBoardDomain.drawings {
		isAssistantDrawing := drawing.Source == string(vo.ChartDrawingSourceAssistant)
		if isAssistantDrawing && (len(drawingIDs) == 0 || slices.Contains(drawingIDs, drawing.ID)) {
			removedDrawingIDs = append(removedDrawingIDs, drawing.ID)
			continue
		}

		remaining.drawings = append(remaining.drawings, drawing)
	}

	if len(removedDrawingIDs) > 0 {
		remaining.revision++
	}

	return remaining, removedDrawingIDs, nil
}

func (chartDrawingBoardDomain ChartDrawingBoardDomain) anchorCount() int {
	anchorCount := 0
	for _, drawing := range chartDrawingBoardDomain.drawings {
		anchorCount += len(drawing.Anchors)
	}

	return anchorCount
}

func (chartDrawingBoardDomain ChartDrawingBoardDomain) ToDto() dto.ChartDrawingBoardDto {
	return dto.ChartDrawingBoardDto{
		Market:   chartDrawingBoardDomain.place.Market(),
		Symbol:   chartDrawingBoardDomain.place.Symbol(),
		Revision: chartDrawingBoardDomain.revision,
		Drawings: slices.Clone(chartDrawingBoardDomain.drawings),
	}
}

func (chartDrawingBoardDomain ChartDrawingBoardDomain) ToEntity() entities.ChartDrawingBoard {
	return entities.ChartDrawingBoard{
		ID:            chartDrawingBoardDomain.boardID,
		OwnerID:       chartDrawingBoardDomain.place.OwnerID(),
		Market:        chartDrawingBoardDomain.place.Market(),
		Symbol:        chartDrawingBoardDomain.place.Symbol(),
		Revision:      chartDrawingBoardDomain.revision,
		LastDrawingID: chartDrawingBoardDomain.lastDrawingID,
		Drawings:      slices.Clone(chartDrawingBoardDomain.drawings),
	}
}
