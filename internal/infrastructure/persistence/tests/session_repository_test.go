package persistence_test

import (
	"reflect"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/infrastructure/persistence"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
)

// aSessionOwner stores a user for sessions to belong to, because a session with no
// owner is a row the foreign key will not accept.
func aSessionOwner(t *testing.T, database *gorm.DB, email string) entities.User {
	t.Helper()

	user, saveError := persistence.NewUserRepository(database).Save(
		t.Context(), entities.User{Email: email, PasswordProof: "a-password-proof"})
	require.NoError(t, saveError)

	return user
}

func sessionOf(userID uint, chainID string, digest string) entities.Session {
	return entities.Session{
		UserID:             userID,
		ChainID:            chainID,
		RefreshTokenDigest: digest,
		ExpiresAt:          time.Now().Add(30 * 24 * time.Hour).UTC(),
	}
}

func TestSessionRepositorySaveHandsBackTheSessionAsStored(t *testing.T) {
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")

	savedSession, saveError := persistence.NewSessionRepository(database).Save(
		t.Context(), sessionOf(owner.ID, "a-chain", "a-digest"))

	require.NoError(t, saveError)
	assert.Positive(t, savedSession.ID)
	assert.False(t, savedSession.CreatedAt.IsZero())
	assert.Nil(t, savedSession.RevokedAt, "剛開的一段還沒有被撤掉")
}

func TestSessionRepositoryFindOneByDigest(t *testing.T) {
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")
	sessionRepository := persistence.NewSessionRepository(database)
	savedSession, saveError := sessionRepository.Save(
		t.Context(), sessionOf(owner.ID, "a-chain", "a-digest"))
	require.NoError(t, saveError)

	foundSession, findError := sessionRepository.FindOneByDigest(t.Context(), "a-digest")

	require.NoError(t, findError)
	assert.Equal(t, savedSession.ID, foundSession.ID)
	assert.Equal(t, "a-chain", foundSession.ChainID)
	assert.Equal(t, owner.ID, foundSession.UserID)
}

func TestSessionRepositoryFindOneByDigestSaysNothingIsThere(t *testing.T) {
	sessionRepository := persistence.NewSessionRepository(newTestDatabase(t))

	_, findError := sessionRepository.FindOneByDigest(t.Context(), "a-digest-nobody-has")

	require.ErrorIs(t, findError, domains.ErrSessionNotFound)
}

func TestSessionRepositoryRefusesTwoSessionsHoldingTheSameDigest(t *testing.T) {
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")
	sessionRepository := persistence.NewSessionRepository(database)
	_, saveError := sessionRepository.Save(t.Context(), sessionOf(owner.ID, "a-chain", "a-digest"))
	require.NoError(t, saveError)

	_, clashError := sessionRepository.Save(
		t.Context(), sessionOf(owner.ID, "another-chain", "a-digest"))

	require.Error(t, clashError, "同一份留存樣指向兩段登入階段，續用就不知道該撤哪一段")
}

func TestSessionRepositoryRotateEndsTheOldAndOpensTheNew(t *testing.T) {
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")
	sessionRepository := persistence.NewSessionRepository(database)
	previousSession, saveError := sessionRepository.Save(
		t.Context(), sessionOf(owner.ID, "a-chain", "a-digest"))
	require.NoError(t, saveError)

	rotatedSession, rotateError := sessionRepository.Rotate(
		t.Context(), previousSession.ID, sessionOf(owner.ID, "a-chain", "a-newer-digest"))

	require.NoError(t, rotateError)
	assert.Positive(t, rotatedSession.ID)
	assert.NotEqual(t, previousSession.ID, rotatedSession.ID)

	endedSession, findError := sessionRepository.FindOneByDigest(t.Context(), "a-digest")
	require.NoError(t, findError,
		"舊的那一段要留著——它正是盜用偵測要撞上的東西，刪掉就等於沒有偵測")
	assert.NotNil(t, endedSession.RevokedAt, "換發過的那一份必須當場作廢，否則它還能再用一次")
}

func TestSessionRepositoryRotateLeavesTheOldAloneWhenTheNewCannotBeWritten(t *testing.T) {
	// Ending the old outside the transaction that writes the new would leave the
	// holder with two proofs that do nothing and no way to find out why.
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")
	sessionRepository := persistence.NewSessionRepository(database)
	previousSession, saveError := sessionRepository.Save(
		t.Context(), sessionOf(owner.ID, "a-chain", "a-digest"))
	require.NoError(t, saveError)
	_, otherError := sessionRepository.Save(
		t.Context(), sessionOf(owner.ID, "another-chain", "a-taken-digest"))
	require.NoError(t, otherError)

	_, rotateError := sessionRepository.Rotate(
		t.Context(), previousSession.ID, sessionOf(owner.ID, "a-chain", "a-taken-digest"))

	require.Error(t, rotateError)
	unchangedSession, findError := sessionRepository.FindOneByDigest(t.Context(), "a-digest")
	require.NoError(t, findError)
	assert.Nil(t, unchangedSession.RevokedAt, "整個交易要一起回滾，舊的那一份不得被動到")
}

func TestSessionRepositoryRevokeChainEndsEverySessionOfOneSignIn(t *testing.T) {
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")
	sessionRepository := persistence.NewSessionRepository(database)
	for _, digest := range []string{"first-digest", "second-digest", "third-digest"} {
		_, saveError := sessionRepository.Save(t.Context(), sessionOf(owner.ID, "a-chain", digest))
		require.NoError(t, saveError)
	}

	require.NoError(t, sessionRepository.RevokeChain(t.Context(), "a-chain"))

	for _, digest := range []string{"first-digest", "second-digest", "third-digest"} {
		endedSession, findError := sessionRepository.FindOneByDigest(t.Context(), digest)
		require.NoError(t, findError)
		assert.NotNil(t, endedSession.RevokedAt,
			"%s 也要被撤掉——包含目前那一份還沒用過的", digest)
	}
}

func TestSessionRepositoryRevokeChainLeavesOtherChainsAlone(t *testing.T) {
	// Signing out one device is signing out that device, not that person.
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")
	sessionRepository := persistence.NewSessionRepository(database)
	_, thisDeviceError := sessionRepository.Save(
		t.Context(), sessionOf(owner.ID, "this-device", "this-digest"))
	require.NoError(t, thisDeviceError)
	_, otherDeviceError := sessionRepository.Save(
		t.Context(), sessionOf(owner.ID, "other-device", "other-digest"))
	require.NoError(t, otherDeviceError)

	require.NoError(t, sessionRepository.RevokeChain(t.Context(), "this-device"))

	otherSession, findError := sessionRepository.FindOneByDigest(t.Context(), "other-digest")
	require.NoError(t, findError)
	assert.Nil(t, otherSession.RevokedAt)
}

func TestSessionRepositoryRevokeChainOnNothingIsNotAFailure(t *testing.T) {
	sessionRepository := persistence.NewSessionRepository(newTestDatabase(t))

	assert.NoError(t, sessionRepository.RevokeChain(t.Context(), "a-chain-nobody-has"),
		"要達成的狀態已經成立了，回報失敗只會讓呼叫端重試一件已經完成的事")
}

func TestSessionRepositoryRevokeChainKeepsTheMomentASessionActuallyEnded(t *testing.T) {
	// The first time is the answer to "when did this stop". Overwriting it would
	// erase the trail somebody would follow to work out what happened.
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")
	sessionRepository := persistence.NewSessionRepository(database)
	_, saveError := sessionRepository.Save(t.Context(), sessionOf(owner.ID, "a-chain", "a-digest"))
	require.NoError(t, saveError)
	require.NoError(t, sessionRepository.RevokeChain(t.Context(), "a-chain"))
	firstlyEnded, findError := sessionRepository.FindOneByDigest(t.Context(), "a-digest")
	require.NoError(t, findError)

	require.NoError(t, sessionRepository.RevokeChain(t.Context(), "a-chain"))

	endedAgain, secondFindError := sessionRepository.FindOneByDigest(t.Context(), "a-digest")
	require.NoError(t, secondFindError)
	assert.Equal(t, firstlyEnded.RevokedAt.UnixNano(), endedAgain.RevokedAt.UnixNano())
}

func TestSessionRepositoryLosesEverySessionOfADeletedUser(t *testing.T) {
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")
	sessionRepository := persistence.NewSessionRepository(database)
	for _, digest := range []string{"first-digest", "second-digest"} {
		_, saveError := sessionRepository.Save(t.Context(), sessionOf(owner.ID, "a-chain", digest))
		require.NoError(t, saveError)
	}

	require.NoError(t, database.WithContext(t.Context()).Delete(&entities.User{}, owner.ID).Error)

	for _, digest := range []string{"first-digest", "second-digest"} {
		_, findError := sessionRepository.FindOneByDigest(t.Context(), digest)
		assert.ErrorIs(t, findError, domains.ErrSessionNotFound,
			"%s 應該跟著那位使用者一起消失", digest)
	}
}

func TestSessionRepositorySaysStorageBrokeRatherThanAnsweringWithNothing(t *testing.T) {
	sessionRepository := persistence.NewSessionRepository(closedDatabase(t))

	_, saveError := sessionRepository.Save(t.Context(), sessionOf(1, "a-chain", "a-digest"))
	_, findError := sessionRepository.FindOneByDigest(t.Context(), "a-digest")
	_, rotateError := sessionRepository.Rotate(t.Context(), 1, sessionOf(1, "a-chain", "a-newer"))
	revokeError := sessionRepository.RevokeChain(t.Context(), "a-chain")

	require.Error(t, saveError)
	require.Error(t, findError)
	assert.NotErrorIs(t, findError, domains.ErrSessionNotFound,
		"連不上資料庫不等於查無這段登入階段——那會讓人被登出而不是被告知系統壞了")
	require.Error(t, rotateError)
	require.Error(t, revokeError)
}

// The repository names the index it relies on in Go; the entity spells it in a
// struct tag, which cannot hold a constant. This test needs no database, so unlike
// the ones above it cannot skip.
func TestTheDigestIndexTheRepositoryReliesOnIsTheOneTheEntityDeclares(t *testing.T) {
	digestField, found := reflect.TypeFor[entities.Session]().FieldByName("RefreshTokenDigest")
	require.True(t, found, "the entity has no RefreshTokenDigest field to carry the index")

	assert.Contains(t, digestField.Tag.Get("gorm"),
		"uniqueIndex:"+persistence.SessionRefreshTokenDigestIndex)
}

func TestSessionRepositoryRotateRefusesASessionThatHasAlreadyEnded(t *testing.T) {
	// This is what makes "a renewal proof works once" true. Two renewals carrying the
	// same proof both read a session that is still good and both go on to write, so
	// the guarantee cannot come from the reading — only the write can establish it.
	// Without this, the second one succeeds and one chain ends up with two live
	// sessions, while the reuse detection that was supposed to catch it never fires.
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")
	sessionRepository := persistence.NewSessionRepository(database)
	previousSession, saveError := sessionRepository.Save(
		t.Context(), sessionOf(owner.ID, "a-chain", "a-digest"))
	require.NoError(t, saveError)
	_, firstRotateError := sessionRepository.Rotate(
		t.Context(), previousSession.ID, sessionOf(owner.ID, "a-chain", "a-second-digest"))
	require.NoError(t, firstRotateError)

	_, secondRotateError := sessionRepository.Rotate(
		t.Context(), previousSession.ID, sessionOf(owner.ID, "a-chain", "a-third-digest"))

	require.ErrorIs(t, secondRotateError, domains.ErrSessionAlreadyRotated)
	_, findError := sessionRepository.FindOneByDigest(t.Context(), "a-third-digest")
	assert.ErrorIs(t, findError, domains.ErrSessionNotFound,
		"被拒絕的那一次不得留下任何東西——留下的話，一份用過的憑證就換出了第二段有效的登入階段")
}

func TestSessionRepositoryRotateCannotUndoASignOut(t *testing.T) {
	// A renewal that read its session just before somebody signed out would otherwise
	// insert a brand-new working proof into a chain that had just been ended: the
	// person pressed sign out, was told it worked, and that device keeps going.
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")
	sessionRepository := persistence.NewSessionRepository(database)
	previousSession, saveError := sessionRepository.Save(
		t.Context(), sessionOf(owner.ID, "a-chain", "a-digest"))
	require.NoError(t, saveError)
	require.NoError(t, sessionRepository.RevokeChain(t.Context(), "a-chain"))

	_, rotateError := sessionRepository.Rotate(
		t.Context(), previousSession.ID, sessionOf(owner.ID, "a-chain", "a-newer-digest"))

	require.ErrorIs(t, rotateError, domains.ErrSessionAlreadyRotated)
	_, findError := sessionRepository.FindOneByDigest(t.Context(), "a-newer-digest")
	assert.ErrorIs(t, findError, domains.ErrSessionNotFound,
		"登出之後不得有任何東西再被放進這條鏈裡")
}

func TestSessionRepositoryFindOneByDigestRefusesToGuessWhenGivenNothing(t *testing.T) {
	// The condition is spelled out rather than given as a struct because GORM drops
	// zero-valued struct fields — and a lookup with no condition hands back whichever
	// row happens to be first, which here would be somebody else's session.
	database := newTestDatabase(t)
	owner := aSessionOwner(t, database, "james@example.com")
	sessionRepository := persistence.NewSessionRepository(database)
	_, saveError := sessionRepository.Save(t.Context(), sessionOf(owner.ID, "a-chain", "a-digest"))
	require.NoError(t, saveError)

	_, findError := sessionRepository.FindOneByDigest(t.Context(), "")

	require.ErrorIs(t, findError, domains.ErrSessionNotFound)
}
