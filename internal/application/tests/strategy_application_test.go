package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// strategyMaxCandleCount is the ceiling these tests judge a candle count against.
const strategyMaxCandleCount = 1000

type strategyApplicationUnderTest struct {
	strategyApplication *application.StrategyApplication
	strategyRepository  *mocks.MockIStrategyRepository
}

// newStrategyApplicationUnderTest wires the real domain service and the real
// strategy model, mocking only the outermost boundary: storage.
func newStrategyApplicationUnderTest(t *testing.T) strategyApplicationUnderTest {
	controller := gomock.NewController(t)
	strategyRepository := mocks.NewMockIStrategyRepository(controller)

	return strategyApplicationUnderTest{
		strategyApplication: application.NewStrategyApplication(
			service.NewStrategyService(strategyRepository, strategyMaxCandleCount)),
		strategyRepository: strategyRepository,
	}
}

func aStrategyWrite() dto.StrategyWriteDto {
	return dto.StrategyWriteDto{
		Name:                "二十根均線",
		Script:              "func Calculate(candles []vo.KCandleVo) map[string][]float64 { return nil }",
		ResultType:          "floatList",
		AggregationInterval: "1h",
		CandleCount:         20,
	}
}

func aStoredStrategy(id uint, name string) entities.Strategy {
	return entities.Strategy{
		ID:                  id,
		Name:                name,
		Script:              aStrategyWrite().Script,
		ResultType:          "floatList",
		AggregationInterval: "1h",
		CandleCount:         20,
		CreatedAt:           time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC),
		UpdatedAt:           time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC),
	}
}

func TestStrategyApplicationCreateStrategy(t *testing.T) {
	t.Run("saves the strategy and hands back what was stored", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		fixture.strategyRepository.EXPECT().
			Save(gomock.Any()).
			DoAndReturn(func(strategy entities.Strategy) (entities.Strategy, error) {
				// A strategy that does not exist yet carries no identifier of its own.
				assert.Equal(t, uint(0), strategy.ID)
				assert.Equal(t, "二十根均線", strategy.Name)
				assert.Equal(t, "1h", strategy.AggregationInterval)
				assert.Equal(t, "floatList", strategy.ResultType)
				assert.Equal(t, 20, strategy.CandleCount)

				return aStoredStrategy(7, strategy.Name), nil
			})

		strategyDto, err := fixture.strategyApplication.CreateStrategy(aStrategyWrite())

		require.NoError(t, err)
		assert.Equal(t, uint(7), strategyDto.ID)
		assert.Equal(t, "二十根均線", strategyDto.Name)
		assert.Equal(t, "1h", strategyDto.AggregationInterval)
	})

	t.Run("saves a name without the blanks around it", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		fixture.strategyRepository.EXPECT().
			Save(gomock.Any()).
			DoAndReturn(func(strategy entities.Strategy) (entities.Strategy, error) {
				assert.Equal(t, "二十根均線", strategy.Name)

				return aStoredStrategy(7, strategy.Name), nil
			})

		writeDto := aStrategyWrite()
		writeDto.Name = "　二十根均線　"

		_, err := fixture.strategyApplication.CreateStrategy(writeDto)

		require.NoError(t, err)
	})

	t.Run("falls back on the same defaults the rest of the system uses", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		fixture.strategyRepository.EXPECT().
			Save(gomock.Any()).
			DoAndReturn(func(strategy entities.Strategy) (entities.Strategy, error) {
				assert.Equal(t, "5m", strategy.AggregationInterval)
				assert.Equal(t, "float", strategy.ResultType)

				return aStoredStrategy(7, strategy.Name), nil
			})

		writeDto := aStrategyWrite()
		writeDto.AggregationInterval = ""
		writeDto.ResultType = ""

		_, err := fixture.strategyApplication.CreateStrategy(writeDto)

		require.NoError(t, err)
	})

	t.Run("saves a script it cannot vouch for", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		fixture.strategyRepository.EXPECT().
			Save(gomock.Any()).Return(aStoredStrategy(7, "二十根均線"), nil)

		writeDto := aStrategyWrite()
		writeDto.Script = "這根本不是一段程式碼"

		_, err := fixture.strategyApplication.CreateStrategy(writeDto)

		require.NoError(t, err)
	})

	t.Run("reports a name another strategy already holds", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		fixture.strategyRepository.EXPECT().
			Save(gomock.Any()).Return(entities.Strategy{}, domains.ErrStrategyNameConflict)

		_, err := fixture.strategyApplication.CreateStrategy(aStrategyWrite())

		require.ErrorIs(t, err, domains.ErrStrategyNameConflict)
	})

	t.Run("reports a storage failure as it is", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		storageFailure := errors.New("connection refused")
		fixture.strategyRepository.EXPECT().
			Save(gomock.Any()).Return(entities.Strategy{}, storageFailure)

		_, err := fixture.strategyApplication.CreateStrategy(aStrategyWrite())

		require.ErrorIs(t, err, storageFailure)
	})
}

func TestStrategyApplicationRefusesContentBeforeAnythingIsWritten(t *testing.T) {
	// The repository is left with no expectation at all, so any call to it fails the
	// test: nothing may be stored, and nothing already stored may be touched.
	testCases := []struct {
		name            string
		breakIt         func(writeDto *dto.StrategyWriteDto)
		expectedMessage string
	}{
		{
			name:            "no name",
			breakIt:         func(writeDto *dto.StrategyWriteDto) { writeDto.Name = "" },
			expectedMessage: "必須給策略取一個名稱",
		},
		{
			name:            "no script",
			breakIt:         func(writeDto *dto.StrategyWriteDto) { writeDto.Script = "" },
			expectedMessage: "策略必須帶一段指標算式",
		},
		{
			name:            "an aggregation interval nobody offers",
			breakIt:         func(writeDto *dto.StrategyWriteDto) { writeDto.AggregationInterval = "7m" },
			expectedMessage: "彙總刻度只能是 5m、15m、1h、4h、1d 其中之一",
		},
		{
			name:            "a result type nobody offers",
			breakIt:         func(writeDto *dto.StrategyWriteDto) { writeDto.ResultType = "string" },
			expectedMessage: "指標值種類只能是 float、floatList、bool、boolList 其中之一",
		},
		{
			name:            "a candle count of zero",
			breakIt:         func(writeDto *dto.StrategyWriteDto) { writeDto.CandleCount = 0 },
			expectedMessage: "計算根數必須大於零",
		},
		{
			name:            "a candle count over the ceiling",
			breakIt:         func(writeDto *dto.StrategyWriteDto) { writeDto.CandleCount = 1001 },
			expectedMessage: "超過單次可用的最大根數",
		},
	}

	for _, testCase := range testCases {
		t.Run("creating with "+testCase.name, func(t *testing.T) {
			fixture := newStrategyApplicationUnderTest(t)
			writeDto := aStrategyWrite()
			testCase.breakIt(&writeDto)

			_, err := fixture.strategyApplication.CreateStrategy(writeDto)

			require.ErrorIs(t, err, domains.ErrStrategyValidation)
			assert.Contains(t, err.Error(), testCase.expectedMessage)
		})

		t.Run("rewriting with "+testCase.name, func(t *testing.T) {
			fixture := newStrategyApplicationUnderTest(t)
			writeDto := aStrategyWrite()
			writeDto.ID = 7
			testCase.breakIt(&writeDto)

			_, err := fixture.strategyApplication.UpdateStrategy(writeDto)

			require.ErrorIs(t, err, domains.ErrStrategyValidation)
			assert.Contains(t, err.Error(), testCase.expectedMessage)
		})
	}
}

func TestStrategyApplicationGetStrategy(t *testing.T) {
	t.Run("hands back the strategy carrying that identifier", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		fixture.strategyRepository.EXPECT().
			FindOne(uint(7)).Return(aStoredStrategy(7, "二十根均線"), nil)

		strategyDto, err := fixture.strategyApplication.GetStrategy(7)

		require.NoError(t, err)
		assert.Equal(t, uint(7), strategyDto.ID)
		assert.Equal(t, "二十根均線", strategyDto.Name)
		assert.Equal(t, aStrategyWrite().Script, strategyDto.Script)
		assert.Equal(t, "floatList", strategyDto.ResultType)
		assert.Equal(t, "1h", strategyDto.AggregationInterval)
		assert.Equal(t, 20, strategyDto.CandleCount)
		assert.False(t, strategyDto.CreatedAt.IsZero())
		assert.False(t, strategyDto.UpdatedAt.IsZero())
	})

	t.Run("reports a strategy that is not there", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		fixture.strategyRepository.EXPECT().
			FindOne(uint(7)).Return(entities.Strategy{}, domains.ErrStrategyNotFound)

		_, err := fixture.strategyApplication.GetStrategy(7)

		require.ErrorIs(t, err, domains.ErrStrategyNotFound)
	})
}

func TestStrategyApplicationListStrategies(t *testing.T) {
	t.Run("hands back every strategy in the order it was given them", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		fixture.strategyRepository.EXPECT().
			FindAll().Return([]entities.Strategy{
			aStoredStrategy(1, "二十根均線"),
			aStoredStrategy(2, "六十根均線"),
		}, nil)

		strategyDtos, err := fixture.strategyApplication.ListStrategies()

		require.NoError(t, err)
		require.Len(t, strategyDtos, 2)
		assert.Equal(t, "二十根均線", strategyDtos[0].Name)
		assert.Equal(t, "六十根均線", strategyDtos[1].Name)

		// Every one of them carries everything it remembers, not just its name —
		// a collection of names would send the reader back for each strategy again.
		for _, strategyDto := range strategyDtos {
			assert.NotZero(t, strategyDto.ID)
			assert.NotEmpty(t, strategyDto.Name)
			assert.Equal(t, aStrategyWrite().Script, strategyDto.Script)
			assert.Equal(t, "floatList", strategyDto.ResultType)
			assert.Equal(t, "1h", strategyDto.AggregationInterval)
			assert.Equal(t, 20, strategyDto.CandleCount)
			assert.False(t, strategyDto.CreatedAt.IsZero())
			assert.False(t, strategyDto.UpdatedAt.IsZero())
		}
	})

	t.Run("holding none is an answer, not a failure", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		fixture.strategyRepository.EXPECT().FindAll().Return([]entities.Strategy{}, nil)

		strategyDtos, err := fixture.strategyApplication.ListStrategies()

		require.NoError(t, err)
		assert.NotNil(t, strategyDtos)
		assert.Empty(t, strategyDtos)
	})

	t.Run("reports a storage failure", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		storageFailure := errors.New("connection refused")
		fixture.strategyRepository.EXPECT().FindAll().Return(nil, storageFailure)

		_, err := fixture.strategyApplication.ListStrategies()

		require.ErrorIs(t, err, storageFailure)
	})
}

func TestStrategyApplicationUpdateStrategy(t *testing.T) {
	t.Run("rewrites the strategy the write names", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		fixture.strategyRepository.EXPECT().
			Update(gomock.Any()).
			DoAndReturn(func(strategy entities.Strategy) (entities.Strategy, error) {
				assert.Equal(t, uint(7), strategy.ID)
				assert.Equal(t, "六十根均線", strategy.Name)
				assert.Equal(t, 60, strategy.CandleCount)

				return aStoredStrategy(7, strategy.Name), nil
			})

		writeDto := aStrategyWrite()
		writeDto.ID = 7
		writeDto.Name = "六十根均線"
		writeDto.CandleCount = 60

		strategyDto, err := fixture.strategyApplication.UpdateStrategy(writeDto)

		require.NoError(t, err)
		assert.Equal(t, uint(7), strategyDto.ID)
		assert.Equal(t, "六十根均線", strategyDto.Name)
	})

	t.Run("reports a strategy that is not there", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		fixture.strategyRepository.EXPECT().
			Update(gomock.Any()).Return(entities.Strategy{}, domains.ErrStrategyNotFound)

		writeDto := aStrategyWrite()
		writeDto.ID = 7

		_, err := fixture.strategyApplication.UpdateStrategy(writeDto)

		require.ErrorIs(t, err, domains.ErrStrategyNotFound)
	})

	t.Run("reports a name another strategy already holds", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		fixture.strategyRepository.EXPECT().
			Update(gomock.Any()).Return(entities.Strategy{}, domains.ErrStrategyNameConflict)

		writeDto := aStrategyWrite()
		writeDto.ID = 7

		_, err := fixture.strategyApplication.UpdateStrategy(writeDto)

		require.ErrorIs(t, err, domains.ErrStrategyNameConflict)
	})

	t.Run("refuses a rewrite that names no strategy without writing anything", func(t *testing.T) {
		// No strategy carries no identifier. Nothing is stubbed on the repository, so
		// a write that went out anyway — which names no row, and whose blast radius
		// is then the storage layer's decision — fails the test.
		fixture := newStrategyApplicationUnderTest(t)
		writeDto := aStrategyWrite()
		writeDto.ID = 0

		_, err := fixture.strategyApplication.UpdateStrategy(writeDto)

		require.ErrorIs(t, err, domains.ErrStrategyNotFound)
	})
}

func TestStrategyApplicationDeleteStrategy(t *testing.T) {
	t.Run("removes the strategy", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		fixture.strategyRepository.EXPECT().Delete(uint(7)).Return(nil)

		require.NoError(t, fixture.strategyApplication.DeleteStrategy(7))
	})

	t.Run("reports a strategy that is not there", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		fixture.strategyRepository.EXPECT().
			Delete(uint(7)).Return(domains.ErrStrategyNotFound)

		err := fixture.strategyApplication.DeleteStrategy(7)

		require.ErrorIs(t, err, domains.ErrStrategyNotFound)
	})
}
