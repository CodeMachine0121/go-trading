package service

import (
	"context"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
)

type SymbolScreeningProfileService struct {
	profileRepository     domaininterface.ISymbolScreeningProfileRepository
	runRepository         domaininterface.ISymbolScreeningRunRepository
	strategyScriptService *StrategyScriptService
}

func NewSymbolScreeningProfileService(
	profileRepository domaininterface.ISymbolScreeningProfileRepository,
	runRepository domaininterface.ISymbolScreeningRunRepository,
	strategyScriptService *StrategyScriptService,
) *SymbolScreeningProfileService {
	return &SymbolScreeningProfileService{
		profileRepository:     profileRepository,
		runRepository:         runRepository,
		strategyScriptService: strategyScriptService,
	}
}

func (profileService *SymbolScreeningProfileService) List(
	executionContext context.Context, ownerID uint,
) ([]dto.SymbolScreeningProfileDto, error) {
	profiles, findError := profileService.profileRepository.FindAllByOwner(executionContext, ownerID)
	if findError != nil {
		return nil, findError
	}

	profileDtos := make([]dto.SymbolScreeningProfileDto, 0, len(profiles))
	for _, profile := range profiles {
		profileDtos = append(profileDtos, profile.ToDto())
	}

	return profileDtos, nil
}

func (profileService *SymbolScreeningProfileService) Get(
	executionContext context.Context, ownerID uint, id uint,
) (dto.SymbolScreeningProfileDto, error) {
	profile, found, findError := profileService.profileRepository.FindOneOwned(executionContext, ownerID, id)
	if findError != nil {
		return dto.SymbolScreeningProfileDto{}, findError
	}
	if !found {
		return dto.SymbolScreeningProfileDto{}, domains.ErrSymbolScreeningProfileNotFound
	}

	return profile.ToDto(), nil
}

func (profileService *SymbolScreeningProfileService) Create(
	executionContext context.Context, ownerID uint, writeDto dto.SymbolScreeningProfileWriteDto,
) (dto.SymbolScreeningProfileDto, error) {
	profileDomain, _, prepareError := profileService.acceptedProfile(executionContext, ownerID, writeDto, 0)
	if prepareError != nil {
		return dto.SymbolScreeningProfileDto{}, prepareError
	}

	saved, saveError := profileService.profileRepository.Save(executionContext, profileDomain.ToEntity())
	if saveError != nil {
		return dto.SymbolScreeningProfileDto{}, saveError
	}

	return saved.ToDto(), nil
}

func (profileService *SymbolScreeningProfileService) Update(
	executionContext context.Context, ownerID uint, id uint, writeDto dto.SymbolScreeningProfileWriteDto,
) (dto.SymbolScreeningProfileDto, error) {
	stored, found, findError := profileService.profileRepository.FindOneOwned(executionContext, ownerID, id)
	if findError != nil {
		return dto.SymbolScreeningProfileDto{}, findError
	}
	if !found {
		return dto.SymbolScreeningProfileDto{}, domains.ErrSymbolScreeningProfileNotFound
	}

	profileDomain, _, prepareError := profileService.acceptedProfile(executionContext, ownerID, writeDto, id)
	if prepareError != nil {
		return dto.SymbolScreeningProfileDto{}, prepareError
	}

	saved, saveError := profileService.profileRepository.Save(executionContext, profileDomain.Replacing(stored).ToEntity())
	if saveError != nil {
		return dto.SymbolScreeningProfileDto{}, saveError
	}

	return saved.ToDto(), nil
}

func (profileService *SymbolScreeningProfileService) Delete(
	executionContext context.Context, ownerID uint, id uint,
) error {
	_, found, findError := profileService.profileRepository.FindOneOwned(executionContext, ownerID, id)
	if findError != nil {
		return findError
	}
	if !found {
		return domains.ErrSymbolScreeningProfileNotFound
	}

	activeCount, countError := profileService.runRepository.CountActiveByProfile(executionContext, id)
	if countError != nil {
		return countError
	}
	if activeCount > 0 {
		return domains.ErrSymbolScreeningProfileInUse
	}

	return profileService.profileRepository.DeleteOwned(executionContext, ownerID, id)
}

// SnapshotFor freezes the profile and its script as they are now, re-checking the script since it may have changed since the profile was saved.
func (profileService *SymbolScreeningProfileService) SnapshotFor(
	executionContext context.Context, ownerID uint, id uint,
) (dto.SymbolScreeningSnapshotDto, error) {
	stored, found, findError := profileService.profileRepository.FindOneOwned(executionContext, ownerID, id)
	if findError != nil {
		return dto.SymbolScreeningSnapshotDto{}, findError
	}
	if !found {
		return dto.SymbolScreeningSnapshotDto{}, domains.ErrSymbolScreeningProfileNotFound
	}

	storedDto := stored.ToDto()
	profileDomain, runnableScript, prepareError := profileService.acceptedProfile(executionContext, ownerID, dto.SymbolScreeningProfileWriteDto{
		Name:                           storedDto.Name,
		StrategyScriptID:               storedDto.StrategyScriptID,
		ParameterValues:                storedDto.ParameterValues,
		AggregationInterval:            storedDto.AggregationInterval,
		TradingMode:                    storedDto.TradingMode,
		Leverage:                       &storedDto.Leverage,
		EntryCostPercentage:            &storedDto.EntryCostPercentage,
		SlippagePercentage:             &storedDto.SlippagePercentage,
		HistoryDays:                    &storedDto.HistoryDays,
		SegmentCount:                   &storedDto.SegmentCount,
		MinimumTradeCount:              &storedDto.MinimumTradeCount,
		MinimumAverageDailyQuoteVolume: &storedDto.MinimumAverageDailyQuoteVolume,
		BaselineSymbols:                storedDto.BaselineSymbols,
	}, id)
	if prepareError != nil {
		return dto.SymbolScreeningSnapshotDto{}, prepareError
	}

	return profileDomain.ToSnapshot(runnableScript), nil
}

// acceptedProfile validates the figures, the script and the name together, since saving and snapshotting must refuse the same things.
func (profileService *SymbolScreeningProfileService) acceptedProfile(
	executionContext context.Context, ownerID uint, writeDto dto.SymbolScreeningProfileWriteDto, exceptID uint,
) (domains.SymbolScreeningProfileDomain, dto.RunnableStrategyScriptDto, error) {
	profileDomain, validationError := domains.NewSymbolScreeningProfileDomain(ownerID, writeDto)
	if validationError != nil {
		return domains.SymbolScreeningProfileDomain{}, dto.RunnableStrategyScriptDto{}, validationError
	}

	runnableScript, resolveError := profileService.strategyScriptService.ResolveOwnedStrategyScript(
		executionContext, ownerID, writeDto.StrategyScriptID)
	if resolveError != nil {
		return domains.SymbolScreeningProfileDomain{}, dto.RunnableStrategyScriptDto{}, resolveError
	}
	if acceptError := profileDomain.AcceptScript(runnableScript); acceptError != nil {
		return domains.SymbolScreeningProfileDomain{}, dto.RunnableStrategyScriptDto{}, acceptError
	}

	nameTaken, nameError := profileService.profileRepository.ExistsName(
		executionContext, ownerID, profileDomain.ToEntity().Name, exceptID)
	if nameError != nil {
		return domains.SymbolScreeningProfileDomain{}, dto.RunnableStrategyScriptDto{}, nameError
	}
	if nameTaken {
		return domains.SymbolScreeningProfileDomain{}, dto.RunnableStrategyScriptDto{},
			domains.ErrSymbolScreeningProfileNameTaken
	}

	return profileDomain, runnableScript, nil
}
