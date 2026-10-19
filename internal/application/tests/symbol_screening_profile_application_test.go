package application_test

import (
	"context"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func aProfileWriteNaming(scriptID uint) dto.SymbolScreeningProfileWriteDto {
	return dto.SymbolScreeningProfileWriteDto{Name: "趨勢 V3", StrategyScriptID: scriptID, AggregationInterval: "4h"}
}

func TestCreateSymbolScreeningProfile(t *testing.T) {
	t.Run("only the essentials are needed, the rest take the defaults", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.theScriptIsOwned()
		underTest.profileRepository.EXPECT().ExistsName(gomock.Any(), screeningAdministratorID, "趨勢 V3", uint(0)).
			Return(false, nil)
		underTest.profileRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, profile entities.SymbolScreeningProfile) (entities.SymbolScreeningProfile, error) {
				profile.ID = screeningProfileID
				return profile, nil
			})

		profile, createError := underTest.application.CreateProfile(
			t.Context(), screeningAdministratorID, aProfileWriteNaming(screeningScriptID))

		require.NoError(t, createError)
		assert.Equal(t, 730, profile.HistoryDays)
		assert.Equal(t, 3, profile.SegmentCount)
		assert.Equal(t, 6, profile.MinimumTradeCount)
		assert.True(t, decimal.NewFromInt(20_000_000).Equal(profile.MinimumAverageDailyQuoteVolume))
		assert.True(t, decimal.NewFromInt(1).Equal(profile.Leverage))
		assert.True(t, decimal.RequireFromString("0.05").Equal(profile.EntryCostPercentage))
		assert.True(t, decimal.RequireFromString("0.05").Equal(profile.SlippagePercentage))
		assert.Equal(t, "longOnly", profile.TradingMode)
		assert.Empty(t, profile.BaselineSymbols)
	})

	t.Run("someone else's script cannot be found", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		foreignScript := anOwnedContractSignalScript()
		foreignScript.ID, foreignScript.OwnerID = 40, 99
		underTest.strategyScriptRepo.EXPECT().FindOne(gomock.Any(), uint(40)).Return(foreignScript, nil)

		_, createError := underTest.application.CreateProfile(t.Context(), screeningAdministratorID, aProfileWriteNaming(40))

		assert.ErrorIs(t, createError, domains.ErrStrategyScriptNotFound)
	})

	t.Run("a name already used by another of my profiles", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.theScriptIsOwned()
		underTest.profileRepository.EXPECT().ExistsName(gomock.Any(), screeningAdministratorID, "趨勢 V3", uint(0)).
			Return(true, nil)

		_, createError := underTest.application.CreateProfile(
			t.Context(), screeningAdministratorID, aProfileWriteNaming(screeningScriptID))

		assert.ErrorIs(t, createError, domains.ErrSymbolScreeningProfileNameTaken)
	})
}

func TestReadAndChangeSymbolScreeningProfile(t *testing.T) {
	t.Run("another administrator's profile cannot be found", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), screeningAdministratorID, uint(5)).
			Return(entities.SymbolScreeningProfile{}, false, nil)

		_, getError := underTest.application.GetProfile(t.Context(), screeningAdministratorID, 5)

		assert.ErrorIs(t, getError, domains.ErrSymbolScreeningProfileNotFound)
	})

	t.Run("changing the history days keeps the same profile", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.theScriptIsOwned()
		underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), screeningAdministratorID, screeningProfileID).
			Return(aStoredScreeningProfile(), true, nil)
		underTest.profileRepository.EXPECT().ExistsName(gomock.Any(), screeningAdministratorID, "趨勢 V3", screeningProfileID).
			Return(false, nil)
		underTest.profileRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, profile entities.SymbolScreeningProfile) (entities.SymbolScreeningProfile, error) {
				return profile, nil
			})
		writeDto := aProfileWriteNaming(screeningScriptID)
		threeHundredSixtyFive := 365
		writeDto.HistoryDays = &threeHundredSixtyFive

		profile, updateError := underTest.application.UpdateProfile(
			t.Context(), screeningAdministratorID, screeningProfileID, writeDto)

		require.NoError(t, updateError)
		assert.Equal(t, screeningProfileID, profile.ID)
		assert.Equal(t, 365, profile.HistoryDays)
	})

	t.Run("a profile a queued run uses cannot be deleted", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), screeningAdministratorID, screeningProfileID).
			Return(aStoredScreeningProfile(), true, nil)
		underTest.runRepository.EXPECT().CountActiveByProfile(gomock.Any(), screeningProfileID).Return(1, nil)

		deleteError := underTest.application.DeleteProfile(t.Context(), screeningAdministratorID, screeningProfileID)

		assert.ErrorIs(t, deleteError, domains.ErrSymbolScreeningProfileInUse)
	})

	t.Run("a profile whose runs have all ended is deleted", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), screeningAdministratorID, screeningProfileID).
			Return(aStoredScreeningProfile(), true, nil)
		underTest.runRepository.EXPECT().CountActiveByProfile(gomock.Any(), screeningProfileID).Return(0, nil)
		underTest.profileRepository.EXPECT().DeleteOwned(gomock.Any(), screeningAdministratorID, screeningProfileID).
			Return(nil)

		assert.NoError(t, underTest.application.DeleteProfile(t.Context(), screeningAdministratorID, screeningProfileID))
	})
}

func TestSubmitSymbolScreeningRun(t *testing.T) {
	t.Run("the pasted list is tidied and the profile and script are frozen into the queued run", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.theScriptIsOwned()
		underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), screeningAdministratorID, screeningProfileID).
			Return(aStoredScreeningProfile(), true, nil)
		underTest.profileRepository.EXPECT().ExistsName(gomock.Any(), screeningAdministratorID, "趨勢 V3", screeningProfileID).
			Return(false, nil)
		underTest.runRepository.EXPECT().CountActiveByOwner(gomock.Any(), screeningAdministratorID).Return(0, nil)
		savedRun := entities.SymbolScreeningRun{}
		underTest.runRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, run entities.SymbolScreeningRun) (entities.SymbolScreeningRun, error) {
				run.ID = screeningRunID
				savedRun = run
				return run, nil
			})

		runDto, submitError := underTest.application.SubmitRun(t.Context(), screeningAdministratorID,
			dto.SymbolScreeningRunSubmissionDto{ProfileID: screeningProfileID, SymbolList: "xrpusdt, DOGEUSDT，linkusdt"})

		require.NoError(t, submitError)
		assert.Equal(t, "queued", runDto.Status)
		assert.Equal(t, 3, runDto.CandidateCount)
		symbols := []string{}
		for _, candidate := range savedRun.Candidates {
			symbols = append(symbols, candidate.Symbol)
			assert.Equal(t, "waiting", candidate.Status)
		}
		assert.Equal(t, []string{"XRPUSDT", "DOGEUSDT", "LINKUSDT"}, symbols)
		assert.Equal(t, "the script", savedRun.Snapshot.Script)
		assert.Equal(t, 730, savedRun.Snapshot.HistoryDays)
	})

	t.Run("one run of mine is already queued", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.theScriptIsOwned()
		underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), screeningAdministratorID, screeningProfileID).
			Return(aStoredScreeningProfile(), true, nil)
		underTest.profileRepository.EXPECT().ExistsName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(false, nil)
		underTest.runRepository.EXPECT().CountActiveByOwner(gomock.Any(), screeningAdministratorID).Return(1, nil)

		_, submitError := underTest.application.SubmitRun(t.Context(), screeningAdministratorID,
			dto.SymbolScreeningRunSubmissionDto{ProfileID: screeningProfileID, SymbolList: "XRPUSDT"})

		assert.ErrorIs(t, submitError, domains.ErrSymbolScreeningRunAlreadyActive)
	})

	t.Run("nothing left after tidying leaves no run behind", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.theScriptIsOwned()
		underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), screeningAdministratorID, screeningProfileID).
			Return(aStoredScreeningProfile(), true, nil)
		underTest.profileRepository.EXPECT().ExistsName(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
			Return(false, nil)

		_, submitError := underTest.application.SubmitRun(t.Context(), screeningAdministratorID,
			dto.SymbolScreeningRunSubmissionDto{ProfileID: screeningProfileID, SymbolList: " , ，"})

		require.ErrorIs(t, submitError, domains.ErrSymbolScreeningValidation)
		assert.Contains(t, submitError.Error(), "沒有任何標的")
	})

	t.Run("another administrator's profile cannot be found", func(t *testing.T) {
		underTest := newSymbolScreeningUnderTest(t, 0)
		underTest.profileRepository.EXPECT().FindOneOwned(gomock.Any(), screeningAdministratorID, uint(5)).
			Return(entities.SymbolScreeningProfile{}, false, nil)

		_, submitError := underTest.application.SubmitRun(t.Context(), screeningAdministratorID,
			dto.SymbolScreeningRunSubmissionDto{ProfileID: 5, SymbolList: "XRPUSDT"})

		assert.ErrorIs(t, submitError, domains.ErrSymbolScreeningProfileNotFound)
	})
}
