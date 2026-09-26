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

// strategyScriptStrangerID is somebody who owns none of the strategy scripts in this file.
const strategyScriptStrangerID = uint(2)

func aStrangersStrategyScript(id uint) entities.StrategyScript {
	strategyScript := aStoredStrategyScript(id, "別人的")
	strategyScript.OwnerID = strategyScriptStrangerID

	return strategyScript
}

func TestStrategyScriptApplicationRefusesEverySortOfSomebodyElsesStrategyScriptTheSameWay(t *testing.T) {
	// All five refusals must read the same, or comparing them would reveal which identifiers exist.
	fixture := newStrategyScriptApplicationUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().
		FindOne(gomock.Any(), uint(7)).Return(aStrangersStrategyScript(7), nil).Times(3)
	fixture.strategyScriptRepository.EXPECT().
		FindOne(gomock.Any(), uint(8)).Return(entities.StrategyScript{}, domains.StrategyScriptNotFound(8))
	fixture.publishedStrategyScriptRepository.EXPECT().
		FindOne(gomock.Any(), uint(7)).Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished)

	_, readError := fixture.strategyScriptApplication.GetStrategyScript(t.Context(), strategyScriptOwnerID, 7)
	deleteError := fixture.strategyScriptApplication.DeleteStrategyScript(t.Context(), strategyScriptOwnerID, 7)
	_, resolveError := fixture.strategyScriptApplication.ResolveRunnableStrategyScript(t.Context(), strategyScriptOwnerID, 7)
	_, missingError := fixture.strategyScriptApplication.GetStrategyScript(t.Context(), strategyScriptOwnerID, 8)

	require.ErrorIs(t, readError, domains.ErrStrategyScriptNotFound)
	require.ErrorIs(t, deleteError, domains.ErrStrategyScriptNotFound)
	require.ErrorIs(t, resolveError, domains.ErrStrategyScriptNotFound)
	require.ErrorIs(t, missingError, domains.ErrStrategyScriptNotFound)
	assert.Equal(t, readError.Error(), deleteError.Error())
	assert.Equal(t, readError.Error(), resolveError.Error())
}

func TestStrategyScriptApplicationLeavesSomebodyElsesStrategyScriptExactlyAsItWas(t *testing.T) {
	// Nothing is stubbed on the writing side: the refusal must come before storage and before the
	// content is judged, or it reveals the target exists.
	fixture := newStrategyScriptApplicationUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aStrangersStrategyScript(7), nil)

	writeDto := aStrategyScriptWrite()
	writeDto.ID = 7
	writeDto.Name = ""

	_, err := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

	require.ErrorIs(t, err, domains.ErrStrategyScriptNotFound)
	assert.NotContains(t, err.Error(), "必須給策略腳本取一個名稱",
		"telling them what is wrong with what they sent tells them the strategy script is there")
}

func TestStrategyScriptApplicationResolvesAPublishedStrategyScriptForAnybody(t *testing.T) {
	// Not having adopted it makes no difference: a marketplace script can be tried before it is
	// taken.
	fixture := newStrategyScriptApplicationUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aStrangersStrategyScript(7), nil)
	fixture.publishedStrategyScriptRepository.EXPECT().
		FindOne(gomock.Any(), uint(7)).Return(entities.PublishedStrategyScript{StrategyScriptID: 7}, nil)

	runnableStrategyScriptDto, err := fixture.strategyScriptApplication.ResolveRunnableStrategyScript(
		t.Context(), strategyScriptOwnerID, 7)

	require.NoError(t, err)
	assert.Equal(t, aStrategyScriptWrite().Script, runnableStrategyScriptDto.Script)
	assert.Equal(t, "floatList", runnableStrategyScriptDto.ResultType)
}

func TestStrategyScriptApplicationWillNotResolveAnUnpublishedStrangersStrategyScript(t *testing.T) {
	fixture := newStrategyScriptApplicationUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aStrangersStrategyScript(7), nil)
	fixture.publishedStrategyScriptRepository.EXPECT().
		FindOne(gomock.Any(), uint(7)).Return(entities.PublishedStrategyScript{}, domains.ErrStrategyScriptNotPublished)

	_, err := fixture.strategyScriptApplication.ResolveRunnableStrategyScript(t.Context(), strategyScriptOwnerID, 7)

	require.ErrorIs(t, err, domains.ErrStrategyScriptNotFound)
}

func TestStrategyScriptApplicationNeverSavesAStrategyScriptBelongingToNobody(t *testing.T) {
	// Nothing is stubbed on the repository: nothing may reach storage.
	fixture := newStrategyScriptApplicationUnderTest(t)
	writeDto := aStrategyScriptWrite()
	writeDto.OwnerID = 0

	_, err := fixture.strategyScriptApplication.CreateStrategyScript(t.Context(), writeDto)

	require.ErrorIs(t, err, domains.ErrStrategyScriptValidation)
	assert.Contains(t, err.Error(), "必須屬於一位使用者")
}

func TestStrategyScriptApplicationDescription(t *testing.T) {
	t.Run("keeps what the owner wrote, without the blanks around it", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		writeDto := aStrategyScriptWrite()
		writeDto.Description = "　抓短線轉折　"

		assertStoredDescription(t, fixture, writeDto, "抓短線轉折")
	})

	t.Run("a description of only blanks is no description at all", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		writeDto := aStrategyScriptWrite()
		writeDto.Description = "   "

		assertStoredDescription(t, fixture, writeDto, "")
	})

	t.Run("no description at all is not a failure", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)

		assertStoredDescription(t, fixture, aStrategyScriptWrite(), "")
	})

	t.Run("a description at the limit is kept", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		writeDto := aStrategyScriptWrite()
		writeDto.Description = descriptionOfLength(512)

		assertStoredDescription(t, fixture, writeDto, writeDto.Description)
	})

	t.Run("a description past the limit is refused", func(t *testing.T) {
		// Nothing is stubbed on the repository: nothing may reach storage.
		fixture := newStrategyScriptApplicationUnderTest(t)
		writeDto := aStrategyScriptWrite()
		writeDto.Description = descriptionOfLength(513)

		_, err := fixture.strategyScriptApplication.CreateStrategyScript(t.Context(), writeDto)

		require.ErrorIs(t, err, domains.ErrStrategyScriptValidation)
		assert.Contains(t, err.Error(), "512")
	})

	t.Run("a rewrite past the limit leaves the strategy script alone", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScript(7, "二十根均線"), nil).AnyTimes()

		writeDto := aStrategyScriptWrite()
		writeDto.ID = 7
		writeDto.Description = descriptionOfLength(513)

		_, err := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

		require.ErrorIs(t, err, domains.ErrStrategyScriptValidation)
	})
}

func descriptionOfLength(characterCount int) string {
	description := make([]rune, characterCount)
	for position := range description {
		description[position] = '轉'
	}

	return string(description)
}

// assertStoredDescription checks the stored description rather than the returned one, because
// trimming happens on the way in.
func assertStoredDescription(
	t *testing.T, fixture strategyScriptApplicationUnderTest, writeDto dto.StrategyScriptWriteDto, expected string,
) {
	t.Helper()

	storedDescription := "<nothing was stored>"
	fixture.strategyScriptRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ any, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
			storedDescription = strategyScript.Description

			return strategyScript, nil
		})

	_, err := fixture.strategyScriptApplication.CreateStrategyScript(t.Context(), writeDto)

	require.NoError(t, err)
	assert.Equal(t, expected, storedDescription)
}

func TestStrategyScriptApplicationReportsAFailureToAskWhetherSomethingIsPublished(t *testing.T) {
	// A store outage must not be read as "not published", or it would look like a genuine refusal.
	fixture := newStrategyScriptApplicationUnderTest(t)
	storageFailure := errors.New("connection refused")
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(aStrangersStrategyScript(7), nil)
	fixture.publishedStrategyScriptRepository.EXPECT().
		FindOne(gomock.Any(), uint(7)).Return(entities.PublishedStrategyScript{}, storageFailure)

	_, err := fixture.strategyScriptApplication.ResolveRunnableStrategyScript(t.Context(), strategyScriptOwnerID, 7)

	require.ErrorIs(t, err, storageFailure)
	require.NotErrorIs(t, err, domains.ErrStrategyScriptNotFound)
}

func TestResolvingAStrategyScriptToRunItWritesNothingBack(t *testing.T) {
	// Running somebody else's script with your own numbers must not write to either store; nothing
	// is stubbed on the writing side.
	fixture := newStrategyScriptApplicationUnderTest(t)
	strangersStrategyScript := aStrangersStrategyScript(7)
	strangersStrategyScript.Parameters = []entities.StrategyScriptParameter{
		{Name: "期數", Kind: "lookbackCount", DefaultValue: 20},
	}
	fixture.strategyScriptRepository.EXPECT().FindOne(gomock.Any(), uint(7)).Return(strangersStrategyScript, nil)
	fixture.publishedStrategyScriptRepository.EXPECT().
		FindOne(gomock.Any(), uint(7)).Return(entities.PublishedStrategyScript{StrategyScriptID: 7}, nil)

	runnableStrategyScriptDto, err := fixture.strategyScriptApplication.ResolveRunnableStrategyScript(
		t.Context(), strategyScriptOwnerID, 7)

	require.NoError(t, err)
	require.Len(t, runnableStrategyScriptDto.Parameters, 1)
	assert.InDelta(t, 20.0, runnableStrategyScriptDto.Parameters[0].DefaultValue, 0,
		"解析交出來的是策略腳本記著的預設值，這一次帶什麼是執行那一端的事")
}

func TestRunningAnAlgorithmNobodySavedNeverTouchesTheStrategyScriptStore(t *testing.T) {
	// An unsaved algorithm goes through no gate; nothing is stubbed on either store to prove it.
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

func TestStrategyScriptApplicationDoesNotAskTheMarketplaceAboutTheCallersOwnStrategyScript(t *testing.T) {
	// 自己的策略腳本先短路、不問市集；常駐機器人每輪都解析每個來源，白問會一直累積。
	fixture := newStrategyScriptApplicationUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().
		FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScript(7, "我的"), nil)
	fixture.publishedStrategyScriptRepository.EXPECT().
		FindOne(gomock.Any(), gomock.Any()).Times(0)

	_, runError := fixture.strategyScriptApplication.ResolveRunnableStrategyScript(
		t.Context(), strategyScriptOwnerID, 7)

	require.NoError(t, runError)
}

func TestStrategyScriptApplicationStillAsksTheMarketplaceAboutSomebodyElsesStrategyScript(t *testing.T) {
	// 別人的那一支就非問不可：第二道關卡沒開，第三道才是答案。
	fixture := newStrategyScriptApplicationUnderTest(t)
	fixture.strategyScriptRepository.EXPECT().
		FindOne(gomock.Any(), uint(7)).Return(aStrangersStrategyScript(7), nil)
	fixture.publishedStrategyScriptRepository.EXPECT().
		FindOne(gomock.Any(), uint(7)).
		Return(entities.PublishedStrategyScript{StrategyScriptID: 7}, nil)

	_, runError := fixture.strategyScriptApplication.ResolveRunnableStrategyScript(
		t.Context(), strategyScriptOwnerID, 7)

	require.NoError(t, runError)
}
