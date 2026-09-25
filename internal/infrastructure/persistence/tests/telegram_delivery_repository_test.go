package persistence_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// aDeliveryOwner stores the user the delivery foreign key requires.
func aDeliveryOwner(t *testing.T, database *gorm.DB, email string) entities.User {
	owner, saveError := persistence.NewUserRepository(database).Save(
		t.Context(), entities.User{Email: email, PasswordProof: "a-password-proof"})
	require.NoError(t, saveError)

	return owner
}

func deliveryOf(userID uint, sealedBotToken string, tail string, chatID string) entities.TelegramDelivery {
	return entities.TelegramDelivery{
		UserID:         userID,
		SealedBotToken: sealedBotToken,
		BotTokenTail:   tail,
		ChatID:         chatID,
	}
}

func TestTelegramDeliveryRepositoryUpsertHandsBackTheSettingAsStored(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")

	savedDelivery, saveError := persistence.NewTelegramDeliveryRepository(database).Upsert(
		t.Context(), deliveryOf(owner.ID, "sealed-one", "1234", "987654"))

	require.NoError(t, saveError)
	assert.Positive(t, savedDelivery.ID)
	assert.False(t, savedDelivery.CreatedAt.IsZero())
	assert.Equal(t, "987654", savedDelivery.ChatID)
}

// The unique index, not a read-then-write, keeps one setting per person.
func TestTelegramDeliveryRepositoryUpsertReplacesRatherThanAdds(t *testing.T) {
	database := newTestDatabase(t)
	telegramDeliveryRepository := persistence.NewTelegramDeliveryRepository(database)
	owner := aDeliveryOwner(t, database, "james@example.com")
	_, firstError := telegramDeliveryRepository.Upsert(
		t.Context(), deliveryOf(owner.ID, "sealed-one", "1234", "987654"))
	require.NoError(t, firstError)

	_, secondError := telegramDeliveryRepository.Upsert(
		t.Context(), deliveryOf(owner.ID, "sealed-two", "5678", "111222"))
	require.NoError(t, secondError)

	storedDelivery, findError := telegramDeliveryRepository.FindOneByUser(t.Context(), owner.ID)
	require.NoError(t, findError)
	assert.Equal(t, "sealed-two", storedDelivery.SealedBotToken)
	assert.Equal(t, "5678", storedDelivery.BotTokenTail)
	assert.Equal(t, "111222", storedDelivery.ChatID)

	var settingCount int64
	require.NoError(t, database.Model(&entities.TelegramDelivery{}).
		Where("user_id = ?", owner.ID).Count(&settingCount).Error)
	assert.Equal(t, int64(1), settingCount, "同一個人只能有一份設定")
}

func TestTelegramDeliveryRepositoryFindOneByUserSaysNothingIsThere(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")

	_, findError := persistence.NewTelegramDeliveryRepository(database).FindOneByUser(
		t.Context(), owner.ID)

	require.ErrorIs(t, findError, domains.ErrTelegramDeliveryNotConfigured)
}

func TestTelegramDeliveryRepositoryFindOneByUserRefusesToGuessWhenGivenNobody(t *testing.T) {
	database := newTestDatabase(t)
	telegramDeliveryRepository := persistence.NewTelegramDeliveryRepository(database)
	owner := aDeliveryOwner(t, database, "james@example.com")
	_, saveError := telegramDeliveryRepository.Upsert(
		t.Context(), deliveryOf(owner.ID, "sealed-one", "1234", "987654"))
	require.NoError(t, saveError)

	_, findError := telegramDeliveryRepository.FindOneByUser(t.Context(), 0)

	require.ErrorIs(t, findError, domains.ErrTelegramDeliveryNotConfigured)
}

func TestTelegramDeliveryRepositoryKeepsEachPersonsSettingApart(t *testing.T) {
	database := newTestDatabase(t)
	telegramDeliveryRepository := persistence.NewTelegramDeliveryRepository(database)
	first := aDeliveryOwner(t, database, "james@example.com")
	second := aDeliveryOwner(t, database, "someone@example.com")
	_, firstError := telegramDeliveryRepository.Upsert(
		t.Context(), deliveryOf(first.ID, "sealed-one", "1234", "111"))
	require.NoError(t, firstError)

	_, secondError := telegramDeliveryRepository.Upsert(
		t.Context(), deliveryOf(second.ID, "sealed-two", "5678", "222"))
	require.NoError(t, secondError)

	firstDelivery, firstFindError := telegramDeliveryRepository.FindOneByUser(t.Context(), first.ID)
	require.NoError(t, firstFindError)
	assert.Equal(t, "111", firstDelivery.ChatID)
}

func TestTelegramDeliveryRepositoryDeleteByUser(t *testing.T) {
	database := newTestDatabase(t)
	telegramDeliveryRepository := persistence.NewTelegramDeliveryRepository(database)
	owner := aDeliveryOwner(t, database, "james@example.com")
	_, saveError := telegramDeliveryRepository.Upsert(
		t.Context(), deliveryOf(owner.ID, "sealed-one", "1234", "987654"))
	require.NoError(t, saveError)

	require.NoError(t, telegramDeliveryRepository.DeleteByUser(t.Context(), owner.ID))

	_, findError := telegramDeliveryRepository.FindOneByUser(t.Context(), owner.ID)
	assert.ErrorIs(t, findError, domains.ErrTelegramDeliveryNotConfigured)
}

func TestTelegramDeliveryRepositoryDeleteByUserSucceedsWithNothingToDelete(t *testing.T) {
	database := newTestDatabase(t)
	owner := aDeliveryOwner(t, database, "james@example.com")

	deleteError := persistence.NewTelegramDeliveryRepository(database).DeleteByUser(
		t.Context(), owner.ID)

	assert.NoError(t, deleteError)
}

func TestTelegramDeliveryRepositoryDeleteByUserLeavesEverybodyElseAlone(t *testing.T) {
	database := newTestDatabase(t)
	telegramDeliveryRepository := persistence.NewTelegramDeliveryRepository(database)
	remover := aDeliveryOwner(t, database, "james@example.com")
	bystander := aDeliveryOwner(t, database, "someone@example.com")
	_, removerError := telegramDeliveryRepository.Upsert(
		t.Context(), deliveryOf(remover.ID, "sealed-one", "1234", "111"))
	require.NoError(t, removerError)
	_, bystanderError := telegramDeliveryRepository.Upsert(
		t.Context(), deliveryOf(bystander.ID, "sealed-two", "5678", "222"))
	require.NoError(t, bystanderError)

	require.NoError(t, telegramDeliveryRepository.DeleteByUser(t.Context(), remover.ID))

	bystanderDelivery, findError := telegramDeliveryRepository.FindOneByUser(
		t.Context(), bystander.ID)
	require.NoError(t, findError)
	assert.Equal(t, "222", bystanderDelivery.ChatID)
}

func TestTelegramDeliveryRepositoryLosesTheSettingWithTheUser(t *testing.T) {
	database := newTestDatabase(t)
	telegramDeliveryRepository := persistence.NewTelegramDeliveryRepository(database)
	owner := aDeliveryOwner(t, database, "james@example.com")
	_, saveError := telegramDeliveryRepository.Upsert(
		t.Context(), deliveryOf(owner.ID, "sealed-one", "1234", "987654"))
	require.NoError(t, saveError)

	require.NoError(t, database.Delete(&entities.User{}, owner.ID).Error)

	_, findError := telegramDeliveryRepository.FindOneByUser(t.Context(), owner.ID)
	assert.ErrorIs(t, findError, domains.ErrTelegramDeliveryNotConfigured)
}

func TestTelegramDeliveryRepositorySaysStorageBrokeRatherThanAnsweringWithNothing(t *testing.T) {
	telegramDeliveryRepository := persistence.NewTelegramDeliveryRepository(closedDatabase(t))

	_, findError := telegramDeliveryRepository.FindOneByUser(t.Context(), 1)
	_, upsertError := telegramDeliveryRepository.Upsert(
		t.Context(), deliveryOf(1, "sealed-one", "1234", "987654"))
	deleteError := telegramDeliveryRepository.DeleteByUser(t.Context(), 1)

	require.Error(t, findError)
	assert.NotErrorIs(t, findError, domains.ErrTelegramDeliveryNotConfigured)
	require.Error(t, upsertError)
	require.Error(t, deleteError)
}
