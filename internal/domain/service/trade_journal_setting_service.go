package service

import (
	"context"
	"fmt"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
)

// TradeJournalSettingService owns a person's fee rates and trade tags.
type TradeJournalSettingService struct {
	tradeJournalSettingRepository domaininterface.ITradeJournalSettingRepository
	tradeTagRepository            domaininterface.ITradeTagRepository
	contractTradeRecordRepository domaininterface.IContractTradeRecordRepository
	clockProxy                    domaininterface.IClockProxy
}

func NewTradeJournalSettingService(
	tradeJournalSettingRepository domaininterface.ITradeJournalSettingRepository,
	tradeTagRepository domaininterface.ITradeTagRepository,
	contractTradeRecordRepository domaininterface.IContractTradeRecordRepository,
	clockProxy domaininterface.IClockProxy,
) *TradeJournalSettingService {
	return &TradeJournalSettingService{
		tradeJournalSettingRepository: tradeJournalSettingRepository,
		tradeTagRepository:            tradeTagRepository,
		contractTradeRecordRepository: contractTradeRecordRepository,
		clockProxy:                    clockProxy,
	}
}

// GetSetting answers null rates for a person who never set any, which is their ordinary state.
func (tradeJournalSettingService *TradeJournalSettingService) GetSetting(
	executionContext context.Context, userID uint,
) (dto.TradeJournalSettingDto, error) {
	setting, _, findError := tradeJournalSettingService.tradeJournalSettingRepository.FindOneByUser(
		executionContext, userID)
	if findError != nil {
		return dto.TradeJournalSettingDto{}, findError
	}

	return setting.ToDto(), nil
}

func (tradeJournalSettingService *TradeJournalSettingService) SaveFeeRates(
	executionContext context.Context, userID uint, writeDto dto.TradeJournalSettingWriteDto,
) (dto.TradeJournalSettingDto, error) {
	setting, _, findError := tradeJournalSettingService.tradeJournalSettingRepository.FindOneByUser(
		executionContext, userID)
	if findError != nil {
		return dto.TradeJournalSettingDto{}, findError
	}

	updatedSetting, validationError := domains.NewTradeJournalSettingDomain(setting).WithFeeRates(writeDto)
	if validationError != nil {
		return dto.TradeJournalSettingDto{}, validationError
	}

	savedSetting, saveError := tradeJournalSettingService.tradeJournalSettingRepository.SaveFeeRates(
		executionContext, updatedSetting.ToEntity(userID))
	if saveError != nil {
		return dto.TradeJournalSettingDto{}, saveError
	}

	return savedSetting.ToDto(), nil
}

// ListTags hands out the default mistake tags the first time only, so tags the person deleted stay deleted.
func (tradeJournalSettingService *TradeJournalSettingService) ListTags(
	executionContext context.Context, ownerID uint,
) ([]dto.TradeTagDto, error) {
	setting, _, findError := tradeJournalSettingService.tradeJournalSettingRepository.FindOneByUser(
		executionContext, ownerID)
	if findError != nil {
		return nil, findError
	}

	if domains.NewTradeJournalSettingDomain(setting).NeedsDefaultMistakeTags() {
		defaultTags := make([]entities.TradeTag, 0, len(domains.DefaultMistakeTagNames))
		for _, name := range domains.DefaultMistakeTagNames {
			defaultTags = append(defaultTags, entities.TradeTag{
				OwnerID: ownerID, Kind: string(vo.TradeTagKindMistake), Name: name,
			})
		}

		if seedError := tradeJournalSettingService.tradeTagRepository.CreateIfAbsent(
			executionContext, defaultTags); seedError != nil {
			return nil, seedError
		}

		if markError := tradeJournalSettingService.tradeJournalSettingRepository.MarkDefaultMistakeTagsSeeded(
			executionContext, ownerID, tradeJournalSettingService.clockProxy.Now()); markError != nil {
			return nil, markError
		}
	}

	tags, listError := tradeJournalSettingService.tradeTagRepository.FindAllByOwner(executionContext, ownerID)
	if listError != nil {
		return nil, listError
	}

	tagDtos := make([]dto.TradeTagDto, 0, len(tags))
	for _, tag := range tags {
		tagDtos = append(tagDtos, tag.ToDto())
	}

	return tagDtos, nil
}

func (tradeJournalSettingService *TradeJournalSettingService) CreateTag(
	executionContext context.Context, ownerID uint, writeDto dto.TradeTagWriteDto,
) (dto.TradeTagDto, error) {
	tagDomain, validationError := domains.NewTradeTagDomain(writeDto)
	if validationError != nil {
		return dto.TradeTagDto{}, validationError
	}

	createdTag, createError := tradeJournalSettingService.tradeTagRepository.Create(
		executionContext, tagDomain.ToEntity(ownerID))
	if createError != nil {
		return dto.TradeTagDto{}, createError
	}

	return createdTag.ToDto(), nil
}

// RenameTag keeps the tag's kind; the trades carrying it follow the new name because they point at the tag.
func (tradeJournalSettingService *TradeJournalSettingService) RenameTag(
	executionContext context.Context, ownerID uint, id uint, name string,
) (dto.TradeTagDto, error) {
	tag, findError := tradeJournalSettingService.findOwnedTag(executionContext, ownerID, id)
	if findError != nil {
		return dto.TradeTagDto{}, findError
	}

	tagDomain, validationError := domains.NewTradeTagDomain(dto.TradeTagWriteDto{Kind: tag.Kind, Name: name})
	if validationError != nil {
		return dto.TradeTagDto{}, validationError
	}

	renamedTag, renameError := tradeJournalSettingService.tradeTagRepository.Rename(
		executionContext, id, tagDomain.Name())
	if renameError != nil {
		return dto.TradeTagDto{}, renameError
	}

	return renamedTag.ToDto(), nil
}

func (tradeJournalSettingService *TradeJournalSettingService) DeleteTag(
	executionContext context.Context, ownerID uint, id uint,
) error {
	if _, findError := tradeJournalSettingService.findOwnedTag(executionContext, ownerID, id); findError != nil {
		return findError
	}

	carryingCount, countError := tradeJournalSettingService.contractTradeRecordRepository.CountByTag(
		executionContext, id)
	if countError != nil {
		return countError
	}
	if carryingCount > 0 {
		return fmt.Errorf("%w: 還有 %d 筆交易貼著它，請先從交易上移除或改名",
			domains.ErrTradeTagInUse, carryingCount)
	}

	return tradeJournalSettingService.tradeTagRepository.Delete(executionContext, id)
}

func (tradeJournalSettingService *TradeJournalSettingService) findOwnedTag(
	executionContext context.Context, ownerID uint, id uint,
) (entities.TradeTag, error) {
	tag, findError := tradeJournalSettingService.tradeTagRepository.FindOne(executionContext, id)
	if findError != nil {
		return entities.TradeTag{}, findError
	}
	if tag.OwnerID != ownerID {
		return entities.TradeTag{}, domains.TradeTagNotFound(id)
	}

	return tag, nil
}
