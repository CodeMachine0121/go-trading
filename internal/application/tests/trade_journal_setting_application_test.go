package application_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const journalOwnerID = uint(7)

var journalMoment = time.Date(2026, 9, 27, 10, 0, 0, 0, time.UTC)

type tradeJournalSettingApplicationUnderTest struct {
	application                   *application.TradeJournalSettingApplication
	tradeJournalSettingRepository *mocks.MockITradeJournalSettingRepository
	tradeTagRepository            *mocks.MockITradeTagRepository
	contractTradeRecordRepository *mocks.MockIContractTradeRecordRepository
}

func newTradeJournalSettingApplicationUnderTest(t *testing.T) tradeJournalSettingApplicationUnderTest {
	mockController := gomock.NewController(t)
	tradeJournalSettingRepository := mocks.NewMockITradeJournalSettingRepository(mockController)
	tradeTagRepository := mocks.NewMockITradeTagRepository(mockController)
	contractTradeRecordRepository := mocks.NewMockIContractTradeRecordRepository(mockController)
	clockProxy := mocks.NewMockIClockProxy(mockController)
	clockProxy.EXPECT().Now().Return(journalMoment).AnyTimes()

	return tradeJournalSettingApplicationUnderTest{
		application: application.NewTradeJournalSettingApplication(service.NewTradeJournalSettingService(
			tradeJournalSettingRepository, tradeTagRepository, contractTradeRecordRepository, clockProxy)),
		tradeJournalSettingRepository: tradeJournalSettingRepository,
		tradeTagRepository:            tradeTagRepository,
		contractTradeRecordRepository: contractTradeRecordRepository,
	}
}

func percentage(value string) decimal.NullDecimal {
	return decimal.NullDecimal{Decimal: decimal.RequireFromString(value), Valid: true}
}

var errStorageDown = errors.New("storage down")

func TestTradeJournalSettingApplicationFeeRates(t *testing.T) {
	t.Run("a person who never set rates gets none", func(t *testing.T) {
		fixture := newTradeJournalSettingApplicationUnderTest(t)
		fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), journalOwnerID).
			Return(entities.TradeJournalSetting{}, false, nil)

		settingDto, err := fixture.application.GetSetting(context.Background(), journalOwnerID)

		require.NoError(t, err)
		assert.False(t, settingDto.MakerFeeRate.Valid)
		assert.False(t, settingDto.TakerFeeRate.Valid)
	})

	t.Run("rates are saved for the person", func(t *testing.T) {
		fixture := newTradeJournalSettingApplicationUnderTest(t)
		fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), journalOwnerID).
			Return(entities.TradeJournalSetting{}, false, nil)
		fixture.tradeJournalSettingRepository.EXPECT().SaveFeeRates(gomock.Any(), entities.TradeJournalSetting{
			UserID: journalOwnerID, MakerFeeRate: percentage("0.02"), TakerFeeRate: percentage("0.05"),
		}).DoAndReturn(func(_ context.Context, setting entities.TradeJournalSetting) (entities.TradeJournalSetting, error) {
			return setting, nil
		})

		settingDto, err := fixture.application.SaveFeeRates(context.Background(), journalOwnerID,
			dto.TradeJournalSettingWriteDto{MakerFeeRate: percentage("0.02"), TakerFeeRate: percentage("0.05")})

		require.NoError(t, err)
		assert.Equal(t, "0.05", settingDto.TakerFeeRate.Decimal.String())
	})

	t.Run("a negative rate is refused before anything is written", func(t *testing.T) {
		fixture := newTradeJournalSettingApplicationUnderTest(t)
		fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), journalOwnerID).
			Return(entities.TradeJournalSetting{}, false, nil)

		_, err := fixture.application.SaveFeeRates(context.Background(), journalOwnerID,
			dto.TradeJournalSettingWriteDto{TakerFeeRate: percentage("-0.01")})

		require.ErrorIs(t, err, domains.ErrTradeJournalSettingValidation)
		assert.Contains(t, err.Error(), "手續費率不得為負")
	})

	t.Run("storage failures come back as they are", func(t *testing.T) {
		fixture := newTradeJournalSettingApplicationUnderTest(t)
		fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), journalOwnerID).
			Return(entities.TradeJournalSetting{}, false, errStorageDown).Times(2)

		_, getError := fixture.application.GetSetting(context.Background(), journalOwnerID)
		_, saveError := fixture.application.SaveFeeRates(context.Background(), journalOwnerID, dto.TradeJournalSettingWriteDto{})

		require.ErrorIs(t, getError, errStorageDown)
		require.ErrorIs(t, saveError, errStorageDown)
	})

	t.Run("a failed save comes back as it is", func(t *testing.T) {
		fixture := newTradeJournalSettingApplicationUnderTest(t)
		fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), journalOwnerID).
			Return(entities.TradeJournalSetting{}, false, nil)
		fixture.tradeJournalSettingRepository.EXPECT().SaveFeeRates(gomock.Any(), gomock.Any()).
			Return(entities.TradeJournalSetting{}, errStorageDown)

		_, err := fixture.application.SaveFeeRates(context.Background(), journalOwnerID, dto.TradeJournalSettingWriteDto{})

		require.ErrorIs(t, err, errStorageDown)
	})
}

func TestTradeJournalSettingApplicationListTags(t *testing.T) {
	t.Run("the first listing hands out the five default mistake tags", func(t *testing.T) {
		fixture := newTradeJournalSettingApplicationUnderTest(t)
		fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), journalOwnerID).
			Return(entities.TradeJournalSetting{}, false, nil)
		seeded := []entities.TradeTag{}
		fixture.tradeTagRepository.EXPECT().CreateIfAbsent(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, tags []entities.TradeTag) error {
				seeded = tags
				return nil
			})
		fixture.tradeJournalSettingRepository.EXPECT().MarkDefaultMistakeTagsSeeded(gomock.Any(), journalOwnerID, journalMoment)
		fixture.tradeTagRepository.EXPECT().FindAllByOwner(gomock.Any(), journalOwnerID).
			DoAndReturn(func(context.Context, uint) ([]entities.TradeTag, error) { return seeded, nil })

		tagDtos, err := fixture.application.ListTags(context.Background(), journalOwnerID)

		require.NoError(t, err)
		names := []string{}
		for _, tagDto := range tagDtos {
			names = append(names, tagDto.Name)
			assert.Equal(t, "mistake", tagDto.Kind)
		}
		assert.Equal(t, []string{"追價進場", "移動止損", "提早出場", "部位過大", "報復性交易"}, names)
	})

	t.Run("deleted defaults do not come back", func(t *testing.T) {
		fixture := newTradeJournalSettingApplicationUnderTest(t)
		seededAt := journalMoment.Add(-time.Hour)
		fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), journalOwnerID).
			Return(entities.TradeJournalSetting{DefaultMistakeTagsSeededAt: &seededAt}, true, nil)
		fixture.tradeTagRepository.EXPECT().FindAllByOwner(gomock.Any(), journalOwnerID).Return([]entities.TradeTag{}, nil)

		tagDtos, err := fixture.application.ListTags(context.Background(), journalOwnerID)

		require.NoError(t, err)
		assert.Empty(t, tagDtos)
	})

	t.Run("each failure along the way comes back", func(t *testing.T) {
		seededAt := journalMoment
		for name, arrange := range map[string]func(fixture tradeJournalSettingApplicationUnderTest){
			"reading the setting": func(fixture tradeJournalSettingApplicationUnderTest) {
				fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), journalOwnerID).
					Return(entities.TradeJournalSetting{}, false, errStorageDown)
			},
			"seeding": func(fixture tradeJournalSettingApplicationUnderTest) {
				fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), journalOwnerID).
					Return(entities.TradeJournalSetting{}, false, nil)
				fixture.tradeTagRepository.EXPECT().CreateIfAbsent(gomock.Any(), gomock.Any()).Return(errStorageDown)
			},
			"marking": func(fixture tradeJournalSettingApplicationUnderTest) {
				fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), journalOwnerID).
					Return(entities.TradeJournalSetting{}, false, nil)
				fixture.tradeTagRepository.EXPECT().CreateIfAbsent(gomock.Any(), gomock.Any()).Return(nil)
				fixture.tradeJournalSettingRepository.EXPECT().MarkDefaultMistakeTagsSeeded(
					gomock.Any(), journalOwnerID, journalMoment).Return(errStorageDown)
			},
			"listing": func(fixture tradeJournalSettingApplicationUnderTest) {
				fixture.tradeJournalSettingRepository.EXPECT().FindOneByUser(gomock.Any(), journalOwnerID).
					Return(entities.TradeJournalSetting{DefaultMistakeTagsSeededAt: &seededAt}, true, nil)
				fixture.tradeTagRepository.EXPECT().FindAllByOwner(gomock.Any(), journalOwnerID).Return(nil, errStorageDown)
			},
		} {
			t.Run(name, func(t *testing.T) {
				fixture := newTradeJournalSettingApplicationUnderTest(t)
				arrange(fixture)

				_, err := fixture.application.ListTags(context.Background(), journalOwnerID)

				require.ErrorIs(t, err, errStorageDown)
			})
		}
	})
}

func TestTradeJournalSettingApplicationTags(t *testing.T) {
	ownTag := entities.TradeTag{ID: 5, OwnerID: journalOwnerID, Kind: "mistake", Name: "移動止損"}
	strangersTag := entities.TradeTag{ID: 6, OwnerID: 99, Kind: "setup", Name: "突破"}

	t.Run("a setup tag is created", func(t *testing.T) {
		fixture := newTradeJournalSettingApplicationUnderTest(t)
		fixture.tradeTagRepository.EXPECT().Create(gomock.Any(),
			entities.TradeTag{OwnerID: journalOwnerID, Kind: "setup", Name: "突破"}).
			Return(entities.TradeTag{ID: 8, OwnerID: journalOwnerID, Kind: "setup", Name: "突破"}, nil)

		tagDto, err := fixture.application.CreateTag(context.Background(), journalOwnerID,
			dto.TradeTagWriteDto{Kind: "setup", Name: " 突破 "})

		require.NoError(t, err)
		assert.Equal(t, dto.TradeTagDto{ID: 8, Kind: "setup", Name: "突破"}, tagDto)
	})

	t.Run("a nameless tag or a duplicate is refused", func(t *testing.T) {
		fixture := newTradeJournalSettingApplicationUnderTest(t)
		fixture.tradeTagRepository.EXPECT().Create(gomock.Any(), gomock.Any()).
			Return(entities.TradeTag{}, domains.ErrTradeTagNameConflict)

		_, blankError := fixture.application.CreateTag(context.Background(), journalOwnerID,
			dto.TradeTagWriteDto{Kind: "setup", Name: ""})
		_, duplicateError := fixture.application.CreateTag(context.Background(), journalOwnerID,
			dto.TradeTagWriteDto{Kind: "setup", Name: "突破"})

		require.ErrorIs(t, blankError, domains.ErrTradeTagValidation)
		require.ErrorIs(t, duplicateError, domains.ErrTradeTagNameConflict)
	})

	t.Run("a tag is renamed within its kind", func(t *testing.T) {
		fixture := newTradeJournalSettingApplicationUnderTest(t)
		fixture.tradeTagRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(ownTag, nil)
		fixture.tradeTagRepository.EXPECT().Rename(gomock.Any(), uint(5), "放寬止損").
			Return(entities.TradeTag{ID: 5, OwnerID: journalOwnerID, Kind: "mistake", Name: "放寬止損"}, nil)

		tagDto, err := fixture.application.RenameTag(context.Background(), journalOwnerID, 5, "放寬止損")

		require.NoError(t, err)
		assert.Equal(t, "放寬止損", tagDto.Name)
	})

	t.Run("renaming refuses a blank name, somebody else's tag and a failed write", func(t *testing.T) {
		fixture := newTradeJournalSettingApplicationUnderTest(t)
		fixture.tradeTagRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(ownTag, nil).Times(2)
		fixture.tradeTagRepository.EXPECT().FindOne(gomock.Any(), uint(6)).Return(strangersTag, nil)
		fixture.tradeTagRepository.EXPECT().Rename(gomock.Any(), uint(5), "追價進場").
			Return(entities.TradeTag{}, domains.ErrTradeTagNameConflict)

		_, blankError := fixture.application.RenameTag(context.Background(), journalOwnerID, 5, " ")
		_, strangerError := fixture.application.RenameTag(context.Background(), journalOwnerID, 6, "回踩")
		_, conflictError := fixture.application.RenameTag(context.Background(), journalOwnerID, 5, "追價進場")

		require.ErrorIs(t, blankError, domains.ErrTradeTagValidation)
		require.ErrorIs(t, strangerError, domains.ErrTradeTagNotFound)
		require.ErrorIs(t, conflictError, domains.ErrTradeTagNameConflict)
	})

	t.Run("a tag still carried by trades cannot be deleted", func(t *testing.T) {
		fixture := newTradeJournalSettingApplicationUnderTest(t)
		fixture.tradeTagRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(ownTag, nil)
		fixture.contractTradeRecordRepository.EXPECT().CountByTag(gomock.Any(), uint(5)).Return(int64(4), nil)

		err := fixture.application.DeleteTag(context.Background(), journalOwnerID, 5)

		require.ErrorIs(t, err, domains.ErrTradeTagInUse)
		assert.Contains(t, err.Error(), "還有 4 筆交易貼著它，請先從交易上移除或改名")
	})

	t.Run("an unused tag is deleted", func(t *testing.T) {
		fixture := newTradeJournalSettingApplicationUnderTest(t)
		fixture.tradeTagRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(ownTag, nil)
		fixture.contractTradeRecordRepository.EXPECT().CountByTag(gomock.Any(), uint(5)).Return(int64(0), nil)
		fixture.tradeTagRepository.EXPECT().Delete(gomock.Any(), uint(5)).Return(nil)

		require.NoError(t, fixture.application.DeleteTag(context.Background(), journalOwnerID, 5))
	})

	t.Run("deleting refuses somebody else's or a missing tag and reports a failed count", func(t *testing.T) {
		fixture := newTradeJournalSettingApplicationUnderTest(t)
		fixture.tradeTagRepository.EXPECT().FindOne(gomock.Any(), uint(6)).Return(strangersTag, nil)
		fixture.tradeTagRepository.EXPECT().FindOne(gomock.Any(), uint(9)).Return(entities.TradeTag{}, domains.TradeTagNotFound(9))
		fixture.tradeTagRepository.EXPECT().FindOne(gomock.Any(), uint(5)).Return(ownTag, nil)
		fixture.contractTradeRecordRepository.EXPECT().CountByTag(gomock.Any(), uint(5)).Return(int64(0), errStorageDown)

		require.ErrorIs(t, fixture.application.DeleteTag(context.Background(), journalOwnerID, 6), domains.ErrTradeTagNotFound)
		require.ErrorIs(t, fixture.application.DeleteTag(context.Background(), journalOwnerID, 9), domains.ErrTradeTagNotFound)
		require.ErrorIs(t, fixture.application.DeleteTag(context.Background(), journalOwnerID, 5), errStorageDown)
	})
}
