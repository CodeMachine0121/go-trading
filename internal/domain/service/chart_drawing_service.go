package service

import (
	"context"
	"errors"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// assistantBoardWriteAttempts bounds the rereads when the person saves the same board while the assistant writes it.
const assistantBoardWriteAttempts = 3

// ChartDrawingService keeps each person's chart drawing boards and gives the assistant a pen that only reaches its own drawings.
type ChartDrawingService struct {
	chartDrawingBoardRepository domaininterface.IChartDrawingBoardRepository
	kCandleService              *KCandleService
	kCandleContractService      *KCandleContractService
	limits                      vo.ChartDrawingLimitsVo
}

func NewChartDrawingService(
	chartDrawingBoardRepository domaininterface.IChartDrawingBoardRepository,
	kCandleService *KCandleService,
	kCandleContractService *KCandleContractService,
	limits vo.ChartDrawingLimitsVo,
) *ChartDrawingService {
	return &ChartDrawingService{
		chartDrawingBoardRepository: chartDrawingBoardRepository,
		kCandleService:              kCandleService,
		kCandleContractService:      kCandleContractService,
		limits:                      limits,
	}
}

// GetBoard answers an empty board at revision zero for a place never drawn on.
func (chartDrawingService *ChartDrawingService) GetBoard(
	executionContext context.Context, placeDto dto.ChartDrawingBoardPlaceDto,
) (dto.ChartDrawingBoardDto, error) {
	place, placeError := domains.NewChartDrawingPlaceDomain(placeDto)
	if placeError != nil {
		return dto.ChartDrawingBoardDto{}, placeError
	}

	board, loadError := chartDrawingService.loadBoard(executionContext, place)
	if loadError != nil {
		return dto.ChartDrawingBoardDto{}, loadError
	}

	return board.ToDto(), nil
}

// ReplaceBoard never compares hand drawings with candles; a board changed since the given revision is refused with the board as it is now.
func (chartDrawingService *ChartDrawingService) ReplaceBoard(
	executionContext context.Context, replaceDto dto.ChartDrawingBoardReplaceDto,
) (dto.ChartDrawingBoardDto, error) {
	place, placeError := domains.NewChartDrawingPlaceDomain(replaceDto.Place)
	if placeError != nil {
		return dto.ChartDrawingBoardDto{}, placeError
	}

	board, loadError := chartDrawingService.loadBoard(executionContext, place)
	if loadError != nil {
		return dto.ChartDrawingBoardDto{}, loadError
	}

	replacedBoard, replaceError := board.Replace(replaceDto.Revision, replaceDto.Drawings)
	if replaceError != nil {
		return dto.ChartDrawingBoardDto{}, replaceError
	}

	if _, saveError := chartDrawingService.chartDrawingBoardRepository.Save(
		executionContext, replacedBoard.ToEntity(), board.Revision()); saveError != nil {
		return dto.ChartDrawingBoardDto{}, chartDrawingService.revisionConflictOr(executionContext, place, saveError)
	}

	return replacedBoard.ToDto(), nil
}

// AddAssistantDrawings reads each anchor's candle once, outside the rereads, since the market does not change when the board does.
func (chartDrawingService *ChartDrawingService) AddAssistantDrawings(
	executionContext context.Context, batchDto dto.AssistantChartDrawingBatchDto,
) (dto.AssistantChartDrawingAdditionDto, error) {
	batch, batchError := domains.NewAssistantChartDrawingBatchDomain(batchDto, chartDrawingService.limits)
	if batchError != nil {
		return dto.AssistantChartDrawingAdditionDto{}, batchError
	}

	anchorCandles := make([]dto.ChartDrawingAnchorCandleDto, 0)
	for _, seriesQueryDto := range batch.AnchorCandleQueries() {
		if batch.Place().IsContract() {
			seriesDto, seriesError := chartDrawingService.kCandleContractService.GetKCandleContractSeries(
				executionContext, seriesQueryDto)
			if seriesError != nil {
				return dto.AssistantChartDrawingAdditionDto{}, seriesError
			}
			for _, kCandleContractDto := range seriesDto.KCandles {
				anchorCandles = append(anchorCandles, kCandleContractDto.ToChartDrawingAnchorCandleDto())
			}
			continue
		}

		seriesDto, seriesError := chartDrawingService.kCandleService.GetKCandleSeries(executionContext, seriesQueryDto)
		if seriesError != nil {
			return dto.AssistantChartDrawingAdditionDto{}, seriesError
		}
		for _, kCandleDto := range seriesDto.KCandles {
			anchorCandles = append(anchorCandles, kCandleDto.ToChartDrawingAnchorCandleDto())
		}
	}

	drawings, drawingsError := batch.ToDrawings(anchorCandles)
	if drawingsError != nil {
		return dto.AssistantChartDrawingAdditionDto{}, drawingsError
	}

	additionDto := dto.AssistantChartDrawingAdditionDto{}
	writeError := chartDrawingService.rewriteBoardForAssistant(executionContext, batch.Place(),
		func(board domains.ChartDrawingBoardDomain) (domains.ChartDrawingBoardDomain, bool, error) {
			appendedBoard, addedDrawings, appendError := board.AppendAssistantDrawings(drawings)
			if appendError != nil {
				return domains.ChartDrawingBoardDomain{}, false, appendError
			}

			additionDto = dto.AssistantChartDrawingAdditionDto{
				Revision: appendedBoard.Revision(), AddedDrawings: addedDrawings,
			}

			return appendedBoard, true, nil
		})
	if writeError != nil {
		return dto.AssistantChartDrawingAdditionDto{}, writeError
	}

	return additionDto, nil
}

// RemoveAssistantDrawings writes nothing when there was nothing of the assistant's to remove.
func (chartDrawingService *ChartDrawingService) RemoveAssistantDrawings(
	executionContext context.Context, removalDto dto.AssistantChartDrawingRemovalDto,
) (dto.AssistantChartDrawingRemovalResultDto, error) {
	place, placeError := domains.NewChartDrawingPlaceDomain(removalDto.Place)
	if placeError != nil {
		return dto.AssistantChartDrawingRemovalResultDto{}, placeError
	}

	resultDto := dto.AssistantChartDrawingRemovalResultDto{}
	writeError := chartDrawingService.rewriteBoardForAssistant(executionContext, place,
		func(board domains.ChartDrawingBoardDomain) (domains.ChartDrawingBoardDomain, bool, error) {
			remainingBoard, removedDrawingIDs, removeError := board.RemoveAssistantDrawings(removalDto.DrawingIDs)
			if removeError != nil {
				return domains.ChartDrawingBoardDomain{}, false, removeError
			}

			resultDto = dto.AssistantChartDrawingRemovalResultDto{
				Revision:          remainingBoard.Revision(),
				RemovedCount:      len(removedDrawingIDs),
				RemovedDrawingIDs: removedDrawingIDs,
			}

			return remainingBoard, len(removedDrawingIDs) > 0, nil
		})
	if writeError != nil {
		return dto.AssistantChartDrawingRemovalResultDto{}, writeError
	}

	return resultDto, nil
}

func (chartDrawingService *ChartDrawingService) loadBoard(
	executionContext context.Context, place domains.ChartDrawingPlaceDomain,
) (domains.ChartDrawingBoardDomain, error) {
	storedBoard, _, findError := chartDrawingService.chartDrawingBoardRepository.FindOne(
		executionContext, place.OwnerID(), place.Market(), place.Symbol())
	if findError != nil {
		return domains.ChartDrawingBoardDomain{}, findError
	}

	return domains.NewChartDrawingBoardDomain(place, storedBoard, chartDrawingService.limits), nil
}

// rewriteBoardForAssistant rereads and reapplies the assistant's relative change when someone wrote the board in between.
func (chartDrawingService *ChartDrawingService) rewriteBoardForAssistant(
	executionContext context.Context,
	place domains.ChartDrawingPlaceDomain,
	change func(board domains.ChartDrawingBoardDomain) (domains.ChartDrawingBoardDomain, bool, error),
) error {
	for attempt := 1; ; attempt++ {
		board, loadError := chartDrawingService.loadBoard(executionContext, place)
		if loadError != nil {
			return loadError
		}

		changedBoard, needsSaving, changeError := change(board)
		if changeError != nil || !needsSaving {
			return changeError
		}

		_, saveError := chartDrawingService.chartDrawingBoardRepository.Save(
			executionContext, changedBoard.ToEntity(), board.Revision())
		if !errors.Is(saveError, domains.ErrChartDrawingBoardRevisionConflict) || attempt == assistantBoardWriteAttempts {
			if saveError != nil {
				return chartDrawingService.revisionConflictOr(executionContext, place, saveError)
			}

			return nil
		}
	}
}

// revisionConflictOr rereads the board for a revision conflict so the caller sees what it is now; other failures pass through.
func (chartDrawingService *ChartDrawingService) revisionConflictOr(
	executionContext context.Context, place domains.ChartDrawingPlaceDomain, saveError error,
) error {
	if !errors.Is(saveError, domains.ErrChartDrawingBoardRevisionConflict) {
		return saveError
	}

	currentBoard, loadError := chartDrawingService.loadBoard(executionContext, place)
	if loadError != nil {
		return loadError
	}

	return domains.ChartDrawingBoardRevisionConflictError{CurrentBoard: currentBoard.ToDto()}
}
