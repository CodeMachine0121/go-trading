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
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const tradingKeyOwnerID = uint(7)

var tradingKeyConfiguredAt = time.Date(2026, 10, 1, 8, 0, 0, 0, time.UTC)

type binanceTradingKeyApplicationUnderTest struct {
	binanceTradingKeyApplication *application.BinanceTradingKeyApplication
	binanceTradingKeyRepository  *mocks.MockIBinanceTradingKeyRepository
	secretSealProxy              *mocks.MockISecretSealProxy
	tradingKeyVerificationProxy  *mocks.MockITradingKeyVerificationProxy
}

// newBinanceTradingKeyApplicationUnderTest wires the real domain service and models, mocking only the store, the lock and Binance.
func newBinanceTradingKeyApplicationUnderTest(t *testing.T) binanceTradingKeyApplicationUnderTest {
	mockController := gomock.NewController(t)
	binanceTradingKeyRepository := mocks.NewMockIBinanceTradingKeyRepository(mockController)
	secretSealProxy := mocks.NewMockISecretSealProxy(mockController)
	tradingKeyVerificationProxy := mocks.NewMockITradingKeyVerificationProxy(mockController)

	return binanceTradingKeyApplicationUnderTest{
		binanceTradingKeyApplication: application.NewBinanceTradingKeyApplication(
			service.NewBinanceTradingKeyService(
				binanceTradingKeyRepository, secretSealProxy, tradingKeyVerificationProxy)),
		binanceTradingKeyRepository: binanceTradingKeyRepository,
		secretSealProxy:             secretSealProxy,
		tradingKeyVerificationProxy: tradingKeyVerificationProxy,
	}
}

// expectSealing seals each string into a recognisable stand-in, so the stored row can be checked for plain text.
func (fixture binanceTradingKeyApplicationUnderTest) expectSealing() {
	fixture.secretSealProxy.EXPECT().Seal(gomock.Any()).
		DoAndReturn(func(plaintext string) (string, error) { return "sealed(" + plaintext + ")", nil }).
		Times(2)
}

func (fixture binanceTradingKeyApplicationUnderTest) expectBinanceAnswers(verification vo.TradingKeyVerificationVo) {
	fixture.tradingKeyVerificationProxy.EXPECT().
		VerifyTradingKey(gomock.Any(), gomock.Any()).
		Return(verification, nil)
}

// expectReplaceHandingBackWhatWasStored records what was stored and which bot kinds were switched off.
func (fixture binanceTradingKeyApplicationUnderTest) expectReplaceHandingBackWhatWasStored(
	stored *entities.BinanceTradingKey, switchedOffKinds *[]string,
) {
	fixture.binanceTradingKeyRepository.EXPECT().
		Replace(gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(
			_ context.Context, binanceTradingKey entities.BinanceTradingKey, uncoveredBotMarketDataKinds []string,
		) (entities.BinanceTradingKey, error) {
			*stored = binanceTradingKey
			*switchedOffKinds = uncoveredBotMarketDataKinds
			binanceTradingKey.ID = 1
			binanceTradingKey.UpdatedAt = tradingKeyConfiguredAt

			return binanceTradingKey, nil
		})
}

func TestBinanceTradingKeyApplicationSaveTradingKey(t *testing.T) {
	t.Run("a first key Binance accepts for spot is stored with its tail and markets", func(t *testing.T) {
		fixture := newBinanceTradingKeyApplicationUnderTest(t)
		fixture.expectSealing()
		fixture.expectBinanceAnswers(vo.TradingKeyVerificationVo{SpotTradingEnabled: true})
		stored := entities.BinanceTradingKey{}
		switchedOffKinds := []string{}
		fixture.expectReplaceHandingBackWhatWasStored(&stored, &switchedOffKinds)

		binanceTradingKeyDto, err := fixture.binanceTradingKeyApplication.SaveTradingKey(
			context.Background(), tradingKeyOwnerID,
			dto.BinanceTradingKeyWriteDto{ApiKey: "the-api-key-a1b2", SecretKey: "the-secret-key"})

		require.NoError(t, err)
		assert.True(t, binanceTradingKeyDto.Configured)
		assert.Equal(t, "a1b2", binanceTradingKeyDto.ApiKeyTail)
		assert.Equal(t, []string{"spot"}, binanceTradingKeyDto.TradableMarkets)
		assert.Equal(t, tradingKeyConfiguredAt, binanceTradingKeyDto.ConfiguredAt)
		assert.Equal(t, tradingKeyOwnerID, stored.UserID)
		assert.Equal(t, "sealed(the-api-key-a1b2)", stored.SealedApiKey)
		assert.Equal(t, "sealed(the-secret-key)", stored.SealedSecretKey)
		assert.ElementsMatch(t, []string{"contractKCandle"}, switchedOffKinds)
	})

	t.Run("a key with both markets switches no bot off", func(t *testing.T) {
		fixture := newBinanceTradingKeyApplicationUnderTest(t)
		fixture.expectSealing()
		fixture.expectBinanceAnswers(vo.TradingKeyVerificationVo{SpotTradingEnabled: true, ContractTradingEnabled: true})
		stored := entities.BinanceTradingKey{}
		switchedOffKinds := []string{"untouched"}
		fixture.expectReplaceHandingBackWhatWasStored(&stored, &switchedOffKinds)

		binanceTradingKeyDto, err := fixture.binanceTradingKeyApplication.SaveTradingKey(
			context.Background(), tradingKeyOwnerID,
			dto.BinanceTradingKeyWriteDto{ApiKey: "the-api-key-c3d4", SecretKey: "the-secret-key"})

		require.NoError(t, err)
		assert.Equal(t, []string{"spot", "contract"}, binanceTradingKeyDto.TradableMarkets)
		assert.Empty(t, switchedOffKinds)
	})

	t.Run("a contract-only key switches the spot bots off, old blank-kind ones included", func(t *testing.T) {
		fixture := newBinanceTradingKeyApplicationUnderTest(t)
		fixture.expectSealing()
		fixture.expectBinanceAnswers(vo.TradingKeyVerificationVo{ContractTradingEnabled: true})
		stored := entities.BinanceTradingKey{}
		switchedOffKinds := []string{}
		fixture.expectReplaceHandingBackWhatWasStored(&stored, &switchedOffKinds)

		binanceTradingKeyDto, err := fixture.binanceTradingKeyApplication.SaveTradingKey(
			context.Background(), tradingKeyOwnerID,
			dto.BinanceTradingKeyWriteDto{ApiKey: "the-api-key-e5f6", SecretKey: "the-secret-key"})

		require.NoError(t, err)
		assert.Equal(t, []string{"contract"}, binanceTradingKeyDto.TradableMarkets)
		assert.ElementsMatch(t, []string{"kCandle", ""}, switchedOffKinds)
	})

	t.Run("Binance is asked with both strings trimmed", func(t *testing.T) {
		fixture := newBinanceTradingKeyApplicationUnderTest(t)
		fixture.expectSealing()
		fixture.tradingKeyVerificationProxy.EXPECT().
			VerifyTradingKey(gomock.Any(), vo.TradingKeyCredentialVo{ApiKey: "the-api-key-a1b2", SecretKey: "the-secret-key"}).
			Return(vo.TradingKeyVerificationVo{SpotTradingEnabled: true}, nil)
		stored := entities.BinanceTradingKey{}
		switchedOffKinds := []string{}
		fixture.expectReplaceHandingBackWhatWasStored(&stored, &switchedOffKinds)

		binanceTradingKeyDto, err := fixture.binanceTradingKeyApplication.SaveTradingKey(
			context.Background(), tradingKeyOwnerID,
			dto.BinanceTradingKeyWriteDto{ApiKey: "  the-api-key-a1b2 \n", SecretKey: "\tthe-secret-key  "})

		require.NoError(t, err)
		assert.Equal(t, "a1b2", binanceTradingKeyDto.ApiKeyTail)
		assert.Equal(t, "sealed(the-secret-key)", stored.SealedSecretKey)
	})

	// Neither blank string reaches Binance, the lock or the store: none of them is set up.
	t.Run("blank strings are refused before anything else is asked", func(t *testing.T) {
		testCases := []struct {
			name            string
			writeDto        dto.BinanceTradingKeyWriteDto
			expectedMessage string
		}{
			{name: "blank API key", writeDto: dto.BinanceTradingKeyWriteDto{ApiKey: "  ", SecretKey: "the-secret-key"}, expectedMessage: "必須給 API Key"},
			{name: "blank secret key", writeDto: dto.BinanceTradingKeyWriteDto{ApiKey: "the-api-key", SecretKey: " "}, expectedMessage: "必須給 Secret Key"},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				fixture := newBinanceTradingKeyApplicationUnderTest(t)

				_, err := fixture.binanceTradingKeyApplication.SaveTradingKey(
					context.Background(), tradingKeyOwnerID, testCase.writeDto)

				require.ErrorIs(t, err, domains.ErrBinanceTradingKeyValidation)
				assert.Contains(t, err.Error(), testCase.expectedMessage)
			})
		}
	})

	t.Run("without a sealing key it refuses before asking Binance and stores nothing", func(t *testing.T) {
		fixture := newBinanceTradingKeyApplicationUnderTest(t)
		fixture.secretSealProxy.EXPECT().Seal(gomock.Any()).
			Return("", domains.ErrSecretSealUnavailable).AnyTimes()

		_, err := fixture.binanceTradingKeyApplication.SaveTradingKey(
			context.Background(), tradingKeyOwnerID,
			dto.BinanceTradingKeyWriteDto{ApiKey: "the-api-key", SecretKey: "the-secret-key"})

		require.ErrorIs(t, err, domains.ErrBinanceTradingKeySealUnavailable)
		assert.Equal(t, "系統目前無法安全保存幣安交易金鑰", err.Error())
	})

	t.Run("a sealing fault other than a missing key is reported as itself", func(t *testing.T) {
		fixture := newBinanceTradingKeyApplicationUnderTest(t)
		sealFault := errors.New("no randomness left")
		fixture.secretSealProxy.EXPECT().Seal(gomock.Any()).Return("", sealFault).AnyTimes()

		_, err := fixture.binanceTradingKeyApplication.SaveTradingKey(
			context.Background(), tradingKeyOwnerID,
			dto.BinanceTradingKeyWriteDto{ApiKey: "the-api-key", SecretKey: "the-secret-key"})

		require.ErrorIs(t, err, sealFault)
		assert.NotErrorIs(t, err, domains.ErrBinanceTradingKeySealUnavailable)
	})

	// The store is not set up to be written, so any write fails the test: the old key and every switch stay as they were.
	t.Run("every Binance refusal stores nothing and names its reason", func(t *testing.T) {
		testCases := []struct {
			name           string
			verification   vo.TradingKeyVerificationVo
			expectedReason vo.TradingKeyVerificationFailureVo
		}{
			{name: "key rejected", verification: vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureKeyRejected}, expectedReason: vo.TradingKeyVerificationFailureKeyRejected},
			{name: "unreachable", verification: vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureUnreachable}, expectedReason: vo.TradingKeyVerificationFailureUnreachable},
			{name: "timed out", verification: vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureTimedOut}, expectedReason: vo.TradingKeyVerificationFailureTimedOut},
			{name: "no trading permission", verification: vo.TradingKeyVerificationVo{}, expectedReason: vo.TradingKeyVerificationFailureNoTradingPermission},
		}

		for _, testCase := range testCases {
			t.Run(testCase.name, func(t *testing.T) {
				fixture := newBinanceTradingKeyApplicationUnderTest(t)
				fixture.expectSealing()
				fixture.expectBinanceAnswers(testCase.verification)

				_, err := fixture.binanceTradingKeyApplication.SaveTradingKey(
					context.Background(), tradingKeyOwnerID,
					dto.BinanceTradingKeyWriteDto{ApiKey: "the-api-key-c3d4", SecretKey: "the-secret-key"})

				verificationError, isVerificationError := errors.AsType[domains.BinanceTradingKeyVerificationError](err)
				require.True(t, isVerificationError)
				assert.Equal(t, testCase.expectedReason, verificationError.Reason)
			})
		}
	})

	t.Run("a local fault while asking Binance stores nothing", func(t *testing.T) {
		fixture := newBinanceTradingKeyApplicationUnderTest(t)
		fixture.expectSealing()
		localFault := errors.New("cannot build the request")
		fixture.tradingKeyVerificationProxy.EXPECT().VerifyTradingKey(gomock.Any(), gomock.Any()).
			Return(vo.TradingKeyVerificationVo{}, localFault)

		_, err := fixture.binanceTradingKeyApplication.SaveTradingKey(
			context.Background(), tradingKeyOwnerID,
			dto.BinanceTradingKeyWriteDto{ApiKey: "the-api-key", SecretKey: "the-secret-key"})

		require.ErrorIs(t, err, localFault)
	})

	t.Run("a store failure is reported as itself", func(t *testing.T) {
		fixture := newBinanceTradingKeyApplicationUnderTest(t)
		fixture.expectSealing()
		fixture.expectBinanceAnswers(vo.TradingKeyVerificationVo{SpotTradingEnabled: true})
		storageFailure := errors.New("the database is not there")
		fixture.binanceTradingKeyRepository.EXPECT().Replace(gomock.Any(), gomock.Any(), gomock.Any()).
			Return(entities.BinanceTradingKey{}, storageFailure)

		_, err := fixture.binanceTradingKeyApplication.SaveTradingKey(
			context.Background(), tradingKeyOwnerID,
			dto.BinanceTradingKeyWriteDto{ApiKey: "the-api-key", SecretKey: "the-secret-key"})

		require.ErrorIs(t, err, storageFailure)
	})
}

func aStoredBinanceTradingKeyOf(userID uint) entities.BinanceTradingKey {
	return entities.BinanceTradingKey{
		ID: 1, UserID: userID,
		SealedApiKey: "sealed-api", SealedSecretKey: "sealed-secret",
		ApiKeyTail: "a1b2", SpotTradingEnabled: true,
		UpdatedAt: tradingKeyConfiguredAt,
	}
}

// Reading never opens anything: the lock is not set up, so touching it fails the test.
func TestBinanceTradingKeyApplicationReads(t *testing.T) {
	t.Run("the owner reads the tail, markets and time", func(t *testing.T) {
		fixture := newBinanceTradingKeyApplicationUnderTest(t)
		fixture.binanceTradingKeyRepository.EXPECT().FindOneByUser(gomock.Any(), tradingKeyOwnerID).
			Return(aStoredBinanceTradingKeyOf(tradingKeyOwnerID), nil)

		binanceTradingKeyDto, err := fixture.binanceTradingKeyApplication.GetTradingKey(
			context.Background(), tradingKeyOwnerID)

		require.NoError(t, err)
		assert.Equal(t, dto.BinanceTradingKeyDto{
			Configured: true, ApiKeyTail: "a1b2", TradableMarkets: []string{"spot"},
			ConfiguredAt: tradingKeyConfiguredAt,
		}, binanceTradingKeyDto)
	})

	t.Run("never having stored one is not a failure", func(t *testing.T) {
		fixture := newBinanceTradingKeyApplicationUnderTest(t)
		fixture.binanceTradingKeyRepository.EXPECT().FindOneByUser(gomock.Any(), tradingKeyOwnerID).
			Return(entities.BinanceTradingKey{}, domains.ErrBinanceTradingKeyNotConfigured)

		binanceTradingKeyDto, err := fixture.binanceTradingKeyApplication.GetTradingKey(
			context.Background(), tradingKeyOwnerID)

		require.NoError(t, err)
		assert.False(t, binanceTradingKeyDto.Configured)
		assert.Empty(t, binanceTradingKeyDto.ApiKeyTail)
		assert.Equal(t, []string{}, binanceTradingKeyDto.TradableMarkets)
	})

	t.Run("the status shows markets and no part of the key", func(t *testing.T) {
		fixture := newBinanceTradingKeyApplicationUnderTest(t)
		fixture.binanceTradingKeyRepository.EXPECT().FindOneByUser(gomock.Any(), tradingKeyOwnerID).
			Return(aStoredBinanceTradingKeyOf(tradingKeyOwnerID), nil)

		statusDto, err := fixture.binanceTradingKeyApplication.GetTradingKeyStatus(
			context.Background(), tradingKeyOwnerID)

		require.NoError(t, err)
		assert.Equal(t, dto.BinanceTradingKeyStatusDto{
			Configured: true, TradableMarkets: []string{"spot"}, ConfiguredAt: tradingKeyConfiguredAt,
		}, statusDto)
	})

	t.Run("the status of never having stored one is not a failure", func(t *testing.T) {
		fixture := newBinanceTradingKeyApplicationUnderTest(t)
		fixture.binanceTradingKeyRepository.EXPECT().FindOneByUser(gomock.Any(), tradingKeyOwnerID).
			Return(entities.BinanceTradingKey{}, domains.ErrBinanceTradingKeyNotConfigured)

		statusDto, err := fixture.binanceTradingKeyApplication.GetTradingKeyStatus(
			context.Background(), tradingKeyOwnerID)

		require.NoError(t, err)
		assert.False(t, statusDto.Configured)
		assert.Equal(t, []string{}, statusDto.TradableMarkets)
	})

	t.Run("storage failing is reported, not read as not configured", func(t *testing.T) {
		storageFailure := errors.New("the database is not there")
		fixture := newBinanceTradingKeyApplicationUnderTest(t)
		fixture.binanceTradingKeyRepository.EXPECT().FindOneByUser(gomock.Any(), tradingKeyOwnerID).
			Return(entities.BinanceTradingKey{}, storageFailure).Times(2)

		_, getError := fixture.binanceTradingKeyApplication.GetTradingKey(context.Background(), tradingKeyOwnerID)
		_, statusError := fixture.binanceTradingKeyApplication.GetTradingKeyStatus(context.Background(), tradingKeyOwnerID)

		require.ErrorIs(t, getError, storageFailure)
		require.ErrorIs(t, statusError, storageFailure)
	})
}

func TestBinanceTradingKeyApplicationRemoveTradingKey(t *testing.T) {
	t.Run("removing asks the store to drop the key and switch every bot off", func(t *testing.T) {
		fixture := newBinanceTradingKeyApplicationUnderTest(t)
		fixture.binanceTradingKeyRepository.EXPECT().DeleteByUser(gomock.Any(), tradingKeyOwnerID).Return(nil)

		require.NoError(t, fixture.binanceTradingKeyApplication.RemoveTradingKey(
			context.Background(), tradingKeyOwnerID))
	})

	t.Run("a store failure is reported", func(t *testing.T) {
		fixture := newBinanceTradingKeyApplicationUnderTest(t)
		storageFailure := errors.New("the database is not there")
		fixture.binanceTradingKeyRepository.EXPECT().DeleteByUser(gomock.Any(), tradingKeyOwnerID).
			Return(storageFailure)

		require.ErrorIs(t, fixture.binanceTradingKeyApplication.RemoveTradingKey(
			context.Background(), tradingKeyOwnerID), storageFailure)
	})
}
