package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBinanceTradingKeyVerificationDomainRefusesWithTheNamedReason(t *testing.T) {
	testCases := []struct {
		name            string
		verification    vo.TradingKeyVerificationVo
		expectedReason  vo.TradingKeyVerificationFailureVo
		expectedMessage string
	}{
		{
			name:            "the key is rejected",
			verification:    vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureKeyRejected},
			expectedReason:  vo.TradingKeyVerificationFailureKeyRejected,
			expectedMessage: "幣安不接受這組金鑰",
		},
		{
			name:            "Binance cannot be reached",
			verification:    vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureUnreachable},
			expectedReason:  vo.TradingKeyVerificationFailureUnreachable,
			expectedMessage: "連不上幣安，請稍後再試",
		},
		{
			name:            "Binance takes too long",
			verification:    vo.TradingKeyVerificationVo{FailureReason: vo.TradingKeyVerificationFailureTimedOut},
			expectedReason:  vo.TradingKeyVerificationFailureTimedOut,
			expectedMessage: "等幣安回答等太久，這次沒有存成",
		},
		{
			name:            "an accepted key with neither trading permission",
			verification:    vo.TradingKeyVerificationVo{},
			expectedReason:  vo.TradingKeyVerificationFailureNoTradingPermission,
			expectedMessage: "這組幣安交易金鑰沒有任何交易權限",
		},
		{
			// A rejected key's permissions mean nothing.
			name: "a rejected key is refused even if permissions came along",
			verification: vo.TradingKeyVerificationVo{
				FailureReason: vo.TradingKeyVerificationFailureKeyRejected, SpotTradingEnabled: true,
			},
			expectedReason:  vo.TradingKeyVerificationFailureKeyRejected,
			expectedMessage: "幣安不接受這組金鑰",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := domains.NewBinanceTradingKeyVerificationDomain(testCase.verification).TradableMarkets()

			require.ErrorIs(t, err, domains.ErrBinanceTradingKeyVerificationFailed)
			verificationError, isVerificationError := err.(domains.BinanceTradingKeyVerificationError)
			require.True(t, isVerificationError)
			assert.Equal(t, testCase.expectedReason, verificationError.Reason)
			assert.Contains(t, err.Error(), testCase.expectedMessage)
		})
	}
}

func TestBinanceTradingKeyVerificationDomainRecordsTheGrantedMarkets(t *testing.T) {
	testCases := []struct {
		name             string
		verification     vo.TradingKeyVerificationVo
		expectedSpot     bool
		expectedContract bool
	}{
		{name: "spot only", verification: vo.TradingKeyVerificationVo{SpotTradingEnabled: true}, expectedSpot: true},
		{name: "contract only", verification: vo.TradingKeyVerificationVo{ContractTradingEnabled: true}, expectedContract: true},
		{name: "both", verification: vo.TradingKeyVerificationVo{SpotTradingEnabled: true, ContractTradingEnabled: true}, expectedSpot: true, expectedContract: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			tradableMarkets, err := domains.NewBinanceTradingKeyVerificationDomain(testCase.verification).TradableMarkets()

			require.NoError(t, err)
			assert.Equal(t, testCase.expectedSpot, tradableMarkets.Covers(vo.TradableMarketSpot))
			assert.Equal(t, testCase.expectedContract, tradableMarkets.Covers(vo.TradableMarketContract))
		})
	}
}
