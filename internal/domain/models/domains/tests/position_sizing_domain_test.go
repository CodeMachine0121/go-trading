package domains_test

import (
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewPositionSizingDomain(t *testing.T) {
	testCases := []struct {
		name          string
		declaredMode  string
		declaredValue decimal.Decimal
		expectedMode  vo.PositionSizingModeVo
		expectsError  bool
	}{
		{
			name:         "declaring nothing stakes everything",
			declaredMode: "",
			expectedMode: vo.PositionSizingModeAllIn,
		},
		{
			name:         "all in is accepted",
			declaredMode: "allIn",
			expectedMode: vo.PositionSizingModeAllIn,
		},
		{
			name:          "a percentage inside the range is accepted",
			declaredMode:  "percentage",
			declaredValue: decimal.NewFromInt(50),
			expectedMode:  vo.PositionSizingModePercentage,
		},
		{
			name:          "a percentage of exactly a hundred is accepted",
			declaredMode:  "percentage",
			declaredValue: decimal.NewFromInt(100),
			expectedMode:  vo.PositionSizingModePercentage,
		},
		{
			name:          "a percentage of zero is refused",
			declaredMode:  "percentage",
			declaredValue: decimal.Zero,
			expectsError:  true,
		},
		{
			name:          "a negative percentage is refused",
			declaredMode:  "percentage",
			declaredValue: decimal.NewFromInt(-1),
			expectsError:  true,
		},
		{
			name:          "a percentage above a hundred is refused",
			declaredMode:  "percentage",
			declaredValue: decimal.NewFromInt(150),
			expectsError:  true,
		},
		{
			name:          "a fixed amount above zero is accepted",
			declaredMode:  "fixedAmount",
			declaredValue: decimal.NewFromInt(3000),
			expectedMode:  vo.PositionSizingModeFixedAmount,
		},
		{
			name:          "a fixed amount of zero is refused",
			declaredMode:  "fixedAmount",
			declaredValue: decimal.Zero,
			expectsError:  true,
		},
		{
			name:          "a negative fixed amount is refused",
			declaredMode:  "fixedAmount",
			declaredValue: decimal.NewFromInt(-3000),
			expectsError:  true,
		},
		{
			name:          "a fixed amount larger than any account is still accepted",
			declaredMode:  "fixedAmount",
			declaredValue: decimal.NewFromInt(30000),
			expectedMode:  vo.PositionSizingModeFixedAmount,
		},
		{name: "an unrecognised mode is refused", declaredMode: "half", expectsError: true},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			positionSizing, err := domains.NewPositionSizingDomain(
				testCase.declaredMode, testCase.declaredValue)

			if testCase.expectsError {
				assert.ErrorIs(t, err, domains.ErrBacktestValidation)
				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.expectedMode, positionSizing.Mode())
		})
	}
}

func TestPositionSizingDomainStakeFor(t *testing.T) {
	testCases := []struct {
		name           string
		declaredMode   string
		declaredValue  decimal.Decimal
		availableCash  decimal.Decimal
		expectedStake  decimal.Decimal
		expectsCanOpen bool
	}{
		{
			name:           "all in stakes every unit of cash on hand",
			declaredMode:   "allIn",
			availableCash:  decimal.NewFromInt(10000),
			expectedStake:  decimal.NewFromInt(10000),
			expectsCanOpen: true,
		},
		{
			name:           "a percentage stakes that share of the cash on hand",
			declaredMode:   "percentage",
			declaredValue:  decimal.NewFromInt(50),
			availableCash:  decimal.NewFromInt(10000),
			expectedStake:  decimal.NewFromInt(5000),
			expectsCanOpen: true,
		},
		{
			name:           "a hundred percent is the same as staking everything",
			declaredMode:   "percentage",
			declaredValue:  decimal.NewFromInt(100),
			availableCash:  decimal.NewFromInt(10000),
			expectedStake:  decimal.NewFromInt(10000),
			expectsCanOpen: true,
		},
		{
			name:           "a fixed amount stakes the same figure every time",
			declaredMode:   "fixedAmount",
			declaredValue:  decimal.NewFromInt(3000),
			availableCash:  decimal.NewFromInt(10000),
			expectedStake:  decimal.NewFromInt(3000),
			expectsCanOpen: true,
		},
		{
			name:           "a fixed amount the account cannot cover stakes nothing",
			declaredMode:   "fixedAmount",
			declaredValue:  decimal.NewFromInt(3000),
			availableCash:  decimal.NewFromInt(2000),
			expectsCanOpen: false,
		},
		{
			name:           "an empty account stakes nothing even when staking everything",
			declaredMode:   "allIn",
			availableCash:  decimal.Zero,
			expectsCanOpen: false,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			positionSizing, err := domains.NewPositionSizingDomain(
				testCase.declaredMode, testCase.declaredValue)
			require.NoError(t, err)

			stake, canOpen := positionSizing.StakeFor(testCase.availableCash)

			assert.Equal(t, testCase.expectsCanOpen, canOpen)
			if testCase.expectsCanOpen {
				assert.True(t, testCase.expectedStake.Equal(stake), "staked %s", stake)
			}
		})
	}
}
