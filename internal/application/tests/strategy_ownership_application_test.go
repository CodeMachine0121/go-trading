package application_test

import (
	"errors"
	"testing"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// strategyStrangerID is somebody who owns none of the strategies in this file.
const strategyStrangerID = uint(2)

// aStrangersStrategy is a stored strategy belonging to somebody who is not the
// caller.
func aStrangersStrategy(id uint) entities.Strategy {
	strategy := aStoredStrategy(id, "別人的")
	strategy.OwnerID = strategyStrangerID

	return strategy
}

func TestStrategyApplicationRefusesEverySortOfSomebodyElsesStrategyTheSameWay(t *testing.T) {
	// Four attempts on somebody else's strategy and one on a strategy that never
	// existed. All five have to say the same thing, or a caller holding a list of
	// identifiers could find out which of them exist by comparing the refusals.
	fixture := newStrategyApplicationUnderTest(t)
	fixture.strategyRepository.EXPECT().
		FindOne(gomock.Any(), uint(7)).Return(aStrangersStrategy(7), nil).Times(3)
	fixture.strategyRepository.EXPECT().
		FindOne(gomock.Any(), uint(8)).Return(entities.Strategy{}, domains.StrategyNotFound(8))
	fixture.publishedStrategyRepository.EXPECT().
		FindOne(gomock.Any(), uint(7)).Return(entities.PublishedStrategy{}, domains.ErrStrategyNotPublished)

	_, readError := fixture.strategyApplication.GetStrategy(t.Context(), strategyOwnerID, 7)
	deleteError := fixture.strategyApplication.DeleteStrategy(t.Context(), strategyOwnerID, 7)
	_, resolveError := fixture.strategyApplication.ResolveRunnableStrategy(t.Context(), strategyOwnerID, 7)
	_, missingError := fixture.strategyApplication.GetStrategy(t.Context(), strategyOwnerID, 8)

	require.ErrorIs(t, readError, domains.ErrStrategyNotFound)
	require.ErrorIs(t, deleteError, domains.ErrStrategyNotFound)
	require.ErrorIs(t, resolveError, domains.ErrStrategyNotFound)
	require.ErrorIs(t, missingError, domains.ErrStrategyNotFound)
	assert.Equal(t, readError.Error(), deleteError.Error())
	assert.Equal(t, readError.Error(), resolveError.Error())
}

func TestStrategyApplicationLeavesSomebodyElsesStrategyExactlyAsItWas(t *testing.T) {
	// Nothing is stubbed on the writing side: a rewrite that is refused must not
	// reach storage at all, and it must be refused before the content it carries is
	// even judged — otherwise the refusal tells a stranger their target exists.
	fixture := newStrategyApplicationUnderTest(t)
	fixture.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aStrangersStrategy(7), nil)

	writeDto := aStrategyWrite()
	writeDto.ID = 7
	writeDto.Name = ""

	_, err := fixture.strategyApplication.UpdateStrategy(t.Context(), writeDto)

	require.ErrorIs(t, err, domains.ErrStrategyNotFound)
	assert.NotContains(t, err.Error(), "必須給策略取一個名稱",
		"telling them what is wrong with what they sent tells them the strategy is there")
}

func TestStrategyApplicationResolvesAPublishedStrategyForAnybody(t *testing.T) {
	// Not having adopted it makes no difference: adoption fills a picker, and a
	// marketplace nobody can try before taking from is a marketplace of blind picks.
	fixture := newStrategyApplicationUnderTest(t)
	fixture.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aStrangersStrategy(7), nil)
	fixture.publishedStrategyRepository.EXPECT().
		FindOne(gomock.Any(), uint(7)).Return(entities.PublishedStrategy{StrategyID: 7}, nil)

	runnableStrategyDto, err := fixture.strategyApplication.ResolveRunnableStrategy(
		t.Context(), strategyOwnerID, 7)

	require.NoError(t, err)
	assert.Equal(t, aStrategyWrite().Script, runnableStrategyDto.Script)
	assert.Equal(t, "floatList", runnableStrategyDto.ResultType)
}

func TestStrategyApplicationWillNotResolveAnUnpublishedStrangersStrategy(t *testing.T) {
	fixture := newStrategyApplicationUnderTest(t)
	fixture.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aStrangersStrategy(7), nil)
	fixture.publishedStrategyRepository.EXPECT().
		FindOne(gomock.Any(), uint(7)).Return(entities.PublishedStrategy{}, domains.ErrStrategyNotPublished)

	_, err := fixture.strategyApplication.ResolveRunnableStrategy(t.Context(), strategyOwnerID, 7)

	require.ErrorIs(t, err, domains.ErrStrategyNotFound)
}

func TestStrategyApplicationNeverSavesAStrategyBelongingToNobody(t *testing.T) {
	// Nothing is stubbed on the repository: nothing may reach storage.
	fixture := newStrategyApplicationUnderTest(t)
	writeDto := aStrategyWrite()
	writeDto.OwnerID = 0

	_, err := fixture.strategyApplication.CreateStrategy(t.Context(), writeDto)

	require.ErrorIs(t, err, domains.ErrStrategyValidation)
	assert.Contains(t, err.Error(), "必須屬於一位使用者")
}

func TestStrategyApplicationDescription(t *testing.T) {
	t.Run("keeps what the owner wrote, without the blanks around it", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		writeDto := aStrategyWrite()
		writeDto.Description = "　抓短線轉折　"

		assertStoredDescription(t, fixture, writeDto, "抓短線轉折")
	})

	t.Run("a description of only blanks is no description at all", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		writeDto := aStrategyWrite()
		writeDto.Description = "   "

		assertStoredDescription(t, fixture, writeDto, "")
	})

	t.Run("no description at all is not a failure", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)

		assertStoredDescription(t, fixture, aStrategyWrite(), "")
	})

	t.Run("a description at the limit is kept", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		writeDto := aStrategyWrite()
		writeDto.Description = descriptionOfLength(512)

		assertStoredDescription(t, fixture, writeDto, writeDto.Description)
	})

	t.Run("a description past the limit is refused", func(t *testing.T) {
		// Nothing is stubbed on the repository: nothing may reach storage.
		fixture := newStrategyApplicationUnderTest(t)
		writeDto := aStrategyWrite()
		writeDto.Description = descriptionOfLength(513)

		_, err := fixture.strategyApplication.CreateStrategy(t.Context(), writeDto)

		require.ErrorIs(t, err, domains.ErrStrategyValidation)
		assert.Contains(t, err.Error(), "512")
	})

	t.Run("a rewrite past the limit leaves the strategy alone", func(t *testing.T) {
		fixture := newStrategyApplicationUnderTest(t)
		fixture.strategyRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategy(7, "二十根均線"), nil)

		writeDto := aStrategyWrite()
		writeDto.ID = 7
		writeDto.Description = descriptionOfLength(513)

		_, err := fixture.strategyApplication.UpdateStrategy(t.Context(), writeDto)

		require.ErrorIs(t, err, domains.ErrStrategyValidation)
	})
}

// descriptionOfLength is a description of exactly this many characters.
func descriptionOfLength(characterCount int) string {
	description := make([]rune, characterCount)
	for position := range description {
		description[position] = '轉'
	}

	return string(description)
}

// assertStoredDescription saves this write and checks the description that reached
// storage. What is asserted is the stored value rather than the returned one,
// because trimming is something the write does on the way in.
func assertStoredDescription(
	t *testing.T, fixture strategyApplicationUnderTest, writeDto dto.StrategyWriteDto, expected string,
) {
	t.Helper()

	storedDescription := "<nothing was stored>"
	fixture.strategyRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, strategy entities.Strategy) (entities.Strategy, error) {
			storedDescription = strategy.Description

			return strategy, nil
		})

	_, err := fixture.strategyApplication.CreateStrategy(t.Context(), writeDto)

	require.NoError(t, err)
	assert.Equal(t, expected, storedDescription)
}

func TestStrategyApplicationReportsAFailureToReadTheShelf(t *testing.T) {
	// The caller's own strategies come back fine and the shelf does not. Answering
	// with half a picker would look like "you adopted nothing".
	fixture := newStrategyApplicationUnderTest(t)
	storageFailure := errors.New("connection refused")
	fixture.strategyRepository.EXPECT().
		FindAllOwnedBy(gomock.Any(), strategyOwnerID).Return([]entities.Strategy{}, nil)
	fixture.strategyRepository.EXPECT().
		FindAllAdoptedBy(gomock.Any(), strategyOwnerID).Return(nil, storageFailure)

	_, err := fixture.strategyApplication.ListAvailableStrategies(t.Context(), strategyOwnerID)

	require.ErrorIs(t, err, storageFailure)
}

func TestStrategyApplicationReportsAFailureToAskWhetherSomethingIsPublished(t *testing.T) {
	// "There is no publication" is one of the two answers; "the store would not
	// say" is neither, and reading it as "not published" would turn an outage into
	// a refusal the caller cannot tell from a real one.
	fixture := newStrategyApplicationUnderTest(t)
	storageFailure := errors.New("connection refused")
	fixture.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aStrangersStrategy(7), nil)
	fixture.publishedStrategyRepository.EXPECT().
		FindOne(gomock.Any(), uint(7)).Return(entities.PublishedStrategy{}, storageFailure)

	_, err := fixture.strategyApplication.ResolveRunnableStrategy(t.Context(), strategyOwnerID, 7)

	require.ErrorIs(t, err, storageFailure)
	require.NotErrorIs(t, err, domains.ErrStrategyNotFound)
}

func TestResolvingAStrategyToRunItWritesNothingBack(t *testing.T) {
	// Running somebody else's strategy with your own numbers must change nothing
	// about it. Nothing is stubbed on the writing side of either store, so any
	// write at all — to the strategy, or to the shelf entry — fails this outright.
	fixture := newStrategyApplicationUnderTest(t)
	strangersStrategy := aStrangersStrategy(7)
	strangersStrategy.Parameters = []entities.StrategyParameter{
		{Name: "期數", Kind: "lookbackCount", DefaultValue: 20},
	}
	fixture.strategyRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(strangersStrategy, nil)
	fixture.publishedStrategyRepository.EXPECT().
		FindOne(gomock.Any(), uint(7)).Return(entities.PublishedStrategy{StrategyID: 7}, nil)

	runnableStrategyDto, err := fixture.strategyApplication.ResolveRunnableStrategy(
		t.Context(), strategyOwnerID, 7)

	require.NoError(t, err)
	require.Len(t, runnableStrategyDto.Parameters, 1)
	assert.InDelta(t, 20.0, runnableStrategyDto.Parameters[0].DefaultValue, 0,
		"解析交出來的是策略記著的預設值，這一次帶什麼是執行那一端的事")
}

func TestRunningAnAlgorithmNobodySavedNeverTouchesTheStrategyStore(t *testing.T) {
	// An algorithm the caller just wrote is their own text, hidden from nobody, so
	// it goes through no gate — and nothing is stubbed on either store here, which
	// is how "no gate" is proved rather than described.
	fixture := newIndicatorUnderTest(t)
	fixture.kCandleRepository.EXPECT().
		FindLatestBefore(gomock.Any(), "BTCUSDT", gomock.Any(), gomock.Any()).
		Return([]entities.KCandle{kCandleAt(at(9, 10), "100"), kCandleAt(at(9, 5), "100")}, nil)
	fixture.indicatorScriptProxy.EXPECT().
		Execute(gomock.Any(), "剛剛寫的那一段", gomock.Any(), gomock.Any(), gomock.Any()).
		Return(map[string]vo.IndicatorValueVo{"ma": {Numbers: []float64{110}}}, nil)

	_, err := fixture.indicatorCalculationApplication.CalculateIndicator(
		t.Context(), indicatorViewerID, carrying(t, "剛剛寫的那一段"), indicatorRequest(1))

	require.NoError(t, err)
}
