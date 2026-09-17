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
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

// strategy scriptOwnerID is whoever these tests act as. Every strategy script they save belongs
// to them, and every strategy script they read back is their own — which is what makes
// these tests about saving and rewriting rather than about who may see what.
const strategyScriptOwnerID = uint(1)

type strategyScriptApplicationUnderTest struct {
	strategyScriptApplication         *application.StrategyScriptApplication
	strategyScriptRepository          *mocks.MockIStrategyScriptRepository
	publishedStrategyScriptRepository *mocks.MockIPublishedStrategyScriptRepository
}

// newStrategyScriptApplicationUnderTest wires the real domain service and the real
// strategy script model, mocking only the outermost boundary: storage.
func newStrategyScriptApplicationUnderTest(t *testing.T) strategyScriptApplicationUnderTest {
	controller := gomock.NewController(t)
	strategyScriptRepository := mocks.NewMockIStrategyScriptRepository(controller)
	publishedStrategyScriptRepository := mocks.NewMockIPublishedStrategyScriptRepository(controller)

	return strategyScriptApplicationUnderTest{
		strategyScriptApplication: application.NewStrategyScriptApplication(
			service.NewStrategyScriptService(strategyScriptRepository, publishedStrategyScriptRepository)),
		strategyScriptRepository:          strategyScriptRepository,
		publishedStrategyScriptRepository: publishedStrategyScriptRepository,
	}
}

func aStrategyScriptWrite() dto.StrategyScriptWriteDto {
	return dto.StrategyScriptWriteDto{
		OwnerID:    strategyScriptOwnerID,
		Name:       "二十根均線",
		Script:     "func Calculate(candles []vo.KCandleVo) map[string][]float64 { return nil }",
		ResultType: "floatList",
	}
}

func aStoredStrategyScript(id uint, name string) entities.StrategyScript {
	return entities.StrategyScript{
		ID:         id,
		OwnerID:    strategyScriptOwnerID,
		Name:       name,
		Script:     aStrategyScriptWrite().Script,
		ResultType: "floatList",
		CreatedAt:  time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC),
		UpdatedAt:  time.Date(2026, 9, 3, 8, 0, 0, 0, time.UTC),
	}
}

// aPublication is one strategy script sitting on the marketplace, owned by somebody who is
// not the caller.
func aPublication(id uint, name string, ownerID uint) entities.PublishedStrategyScript {
	strategyScript := aStoredStrategyScript(id, name)
	strategyScript.OwnerID = ownerID
	strategyScript.Owner = entities.User{ID: ownerID, Email: "someone@example.com"}

	return entities.PublishedStrategyScript{
		StrategyScriptID: id,
		PublishedAt:      time.Date(2026, 9, 10, 8, 0, 0, 0, time.UTC),
		StrategyScript:   strategyScript,
	}
}

func TestStrategyScriptApplicationCreateStrategyScript(t *testing.T) {
	t.Run("saves the strategy script and hands back what was stored", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
				// A strategy script that does not exist yet carries no identifier of its own.
				assert.Equal(t, uint(0), strategyScript.ID)
				assert.Equal(t, "二十根均線", strategyScript.Name)
				assert.Equal(t, "floatList", strategyScript.ResultType)

				return aStoredStrategyScript(7, strategyScript.Name), nil
			})

		strategyScriptDto, err := fixture.strategyScriptApplication.CreateStrategyScript(t.Context(), aStrategyScriptWrite())

		require.NoError(t, err)
		assert.Equal(t, uint(7), strategyScriptDto.ID)
		assert.Equal(t, "二十根均線", strategyScriptDto.Name)
	})

	t.Run("saves a name without the blanks around it", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
				assert.Equal(t, "二十根均線", strategyScript.Name)

				return aStoredStrategyScript(7, strategyScript.Name), nil
			})

		writeDto := aStrategyScriptWrite()
		writeDto.Name = "　二十根均線　"

		_, err := fixture.strategyScriptApplication.CreateStrategyScript(t.Context(), writeDto)

		require.NoError(t, err)
	})

	t.Run("a name differing only by its blanks reaches storage as the same name", func(t *testing.T) {
		// This is the half of "只差前後空白的名稱視為重複" that lives above storage:
		// two spellings arrive, one name is stored. The other half — one name twice
		// is a conflict — is asserted against the real index in the repository tests.
		fixture := newStrategyScriptApplicationUnderTest(t)
		storedNames := make([]string, 0, 2)
		fixture.strategyScriptRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
				storedNames = append(storedNames, strategyScript.Name)

				return aStoredStrategyScript(uint(len(storedNames)), strategyScript.Name), nil
			}).Times(2)

		plainly := aStrategyScriptWrite()
		plainly.Name = "二十根均線"
		padded := aStrategyScriptWrite()
		padded.Name = "　二十根均線　"

		_, plainError := fixture.strategyScriptApplication.CreateStrategyScript(t.Context(), plainly)
		_, paddedError := fixture.strategyScriptApplication.CreateStrategyScript(t.Context(), padded)

		require.NoError(t, plainError)
		require.NoError(t, paddedError)
		assert.Equal(t, []string{"二十根均線", "二十根均線"}, storedNames)
	})

	t.Run("falls back on the same default the rest of the system uses", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
				assert.Equal(t, "float", strategyScript.ResultType)

				return aStoredStrategyScript(7, strategyScript.Name), nil
			})

		writeDto := aStrategyScriptWrite()
		writeDto.ResultType = ""

		_, err := fixture.strategyScriptApplication.CreateStrategyScript(t.Context(), writeDto)

		require.NoError(t, err)
	})

	t.Run("saves a script it cannot vouch for", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).Return(aStoredStrategyScript(7, "二十根均線"), nil)

		writeDto := aStrategyScriptWrite()
		writeDto.Script = "這根本不是一段程式碼"

		_, err := fixture.strategyScriptApplication.CreateStrategyScript(t.Context(), writeDto)

		require.NoError(t, err)
	})

	t.Run("reports a name another strategy script already holds", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).Return(entities.StrategyScript{}, domains.ErrStrategyScriptNameConflict)

		_, err := fixture.strategyScriptApplication.CreateStrategyScript(t.Context(), aStrategyScriptWrite())

		require.ErrorIs(t, err, domains.ErrStrategyScriptNameConflict)
	})

	t.Run("reports a storage failure as it is", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		storageFailure := errors.New("connection refused")
		fixture.strategyScriptRepository.EXPECT().
			Save(gomock.Any(), gomock.Any()).Return(entities.StrategyScript{}, storageFailure)

		_, err := fixture.strategyScriptApplication.CreateStrategyScript(t.Context(), aStrategyScriptWrite())

		require.ErrorIs(t, err, storageFailure)
	})
}

func TestStrategyScriptApplicationRefusesContentBeforeAnythingIsWritten(t *testing.T) {
	// The repository is left with no expectation at all, so any call to it fails the
	// test: nothing may be stored, and nothing already stored may be touched.
	testCases := []struct {
		name            string
		breakIt         func(writeDto *dto.StrategyScriptWriteDto)
		expectedMessage string
	}{
		{
			name:            "no name",
			breakIt:         func(writeDto *dto.StrategyScriptWriteDto) { writeDto.Name = "" },
			expectedMessage: "必須給策略腳本取一個名稱",
		},
		{
			name:            "no script",
			breakIt:         func(writeDto *dto.StrategyScriptWriteDto) { writeDto.Script = "" },
			expectedMessage: "策略腳本必須帶一段指標算式",
		},
		{
			name:            "a result type nobody offers",
			breakIt:         func(writeDto *dto.StrategyScriptWriteDto) { writeDto.ResultType = "string" },
			expectedMessage: "指標值種類只能是 float、floatList、bool、boolList、signal 其中之一",
		},
	}

	for _, testCase := range testCases {
		t.Run("creating with "+testCase.name, func(t *testing.T) {
			fixture := newStrategyScriptApplicationUnderTest(t)
			writeDto := aStrategyScriptWrite()
			testCase.breakIt(&writeDto)

			_, err := fixture.strategyScriptApplication.CreateStrategyScript(t.Context(), writeDto)

			require.ErrorIs(t, err, domains.ErrStrategyScriptValidation)
			assert.Contains(t, err.Error(), testCase.expectedMessage)
		})

		t.Run("rewriting with "+testCase.name, func(t *testing.T) {
			// The strategy script is there; only its new content is wrong. Update is left
			// unstubbed, so a write that went out anyway fails the test.
			fixture := newStrategyScriptApplicationUnderTest(t)
			fixture.strategyScriptRepository.EXPECT().
				FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScript(7, "二十根均線"), nil)
			writeDto := aStrategyScriptWrite()
			writeDto.ID = 7
			testCase.breakIt(&writeDto)

			_, err := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

			require.ErrorIs(t, err, domains.ErrStrategyScriptValidation)
			assert.Contains(t, err.Error(), testCase.expectedMessage)
		})
	}
}

func TestStrategyScriptApplicationGetStrategyScript(t *testing.T) {
	t.Run("hands back the strategy script carrying that identifier", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScript(7, "二十根均線"), nil)

		strategyScriptDto, err := fixture.strategyScriptApplication.GetStrategyScript(t.Context(), strategyScriptOwnerID, 7)

		require.NoError(t, err)
		assert.Equal(t, uint(7), strategyScriptDto.ID)
		assert.Equal(t, "二十根均線", strategyScriptDto.Name)
		assert.Equal(t, aStrategyScriptWrite().Script, strategyScriptDto.Script)
		assert.Equal(t, "floatList", strategyScriptDto.ResultType)
		assert.False(t, strategyScriptDto.CreatedAt.IsZero())
		assert.False(t, strategyScriptDto.UpdatedAt.IsZero())
	})

	t.Run("reports a strategy script that is not there", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(entities.StrategyScript{}, domains.ErrStrategyScriptNotFound)

		_, err := fixture.strategyScriptApplication.GetStrategyScript(t.Context(), strategyScriptOwnerID, 7)

		require.ErrorIs(t, err, domains.ErrStrategyScriptNotFound)
	})
}

func TestStrategyScriptApplicationListAvailableStrategyScripts(t *testing.T) {
	t.Run("hands back the caller's own strategy scriptScripts in the order it was given them", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindAllOwnedBy(gomock.Any(), strategyScriptOwnerID).Return([]entities.StrategyScript{
			aStoredStrategyScript(1, "二十根均線"),
			aStoredStrategyScript(2, "六十根均線"),
		}, nil)
		fixture.strategyScriptRepository.EXPECT().
			FindAllAdoptedBy(gomock.Any(), strategyScriptOwnerID).Return([]entities.PublishedStrategyScript{}, nil)

		availableStrategyScriptsDto, err := fixture.strategyScriptApplication.ListAvailableStrategyScripts(
			t.Context(), strategyScriptOwnerID)

		require.NoError(t, err)
		require.Len(t, availableStrategyScriptsDto.Mine, 2)
		assert.Equal(t, "二十根均線", availableStrategyScriptsDto.Mine[0].Name)
		assert.Equal(t, "六十根均線", availableStrategyScriptsDto.Mine[1].Name)

		// Every one of them carries everything it remembers, not just its name —
		// a collection of names would send the reader back for each strategy script again.
		for _, strategyScriptDto := range availableStrategyScriptsDto.Mine {
			assert.NotZero(t, strategyScriptDto.ID)
			assert.NotEmpty(t, strategyScriptDto.Name)
			assert.Equal(t, aStrategyScriptWrite().Script, strategyScriptDto.Script)
			assert.Equal(t, "floatList", strategyScriptDto.ResultType)
			assert.False(t, strategyScriptDto.CreatedAt.IsZero())
			assert.False(t, strategyScriptDto.UpdatedAt.IsZero())
		}
	})

	t.Run("hands back adopted strategy scriptScripts after the caller's own, and without their scripts", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindAllOwnedBy(gomock.Any(), strategyScriptOwnerID).
			Return([]entities.StrategyScript{aStoredStrategyScript(1, "我的")}, nil)
		fixture.strategyScriptRepository.EXPECT().
			FindAllAdoptedBy(gomock.Any(), strategyScriptOwnerID).
			Return([]entities.PublishedStrategyScript{aPublication(2, "別人的", 8)}, nil)

		availableStrategyScriptsDto, err := fixture.strategyScriptApplication.ListAvailableStrategyScripts(
			t.Context(), strategyScriptOwnerID)

		require.NoError(t, err)
		require.Len(t, availableStrategyScriptsDto.Mine, 1)
		require.Len(t, availableStrategyScriptsDto.Adopted, 1)
		assert.Equal(t, "我的", availableStrategyScriptsDto.Mine[0].Name)
		assert.Equal(t, "別人的", availableStrategyScriptsDto.Adopted[0].Name)
		assert.Equal(t, "someone@example.com", availableStrategyScriptsDto.Adopted[0].PublisherEmail)
	})

	t.Run("holding none is an answer, not a failure", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindAllOwnedBy(gomock.Any(), strategyScriptOwnerID).Return([]entities.StrategyScript{}, nil)
		fixture.strategyScriptRepository.EXPECT().
			FindAllAdoptedBy(gomock.Any(), strategyScriptOwnerID).Return([]entities.PublishedStrategyScript{}, nil)

		availableStrategyScriptsDto, err := fixture.strategyScriptApplication.ListAvailableStrategyScripts(
			t.Context(), strategyScriptOwnerID)

		require.NoError(t, err)
		assert.NotNil(t, availableStrategyScriptsDto.Mine)
		assert.NotNil(t, availableStrategyScriptsDto.Adopted)
		assert.Empty(t, availableStrategyScriptsDto.Mine)
		assert.Empty(t, availableStrategyScriptsDto.Adopted)
	})

	t.Run("reports a storage failure", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		storageFailure := errors.New("connection refused")
		fixture.strategyScriptRepository.EXPECT().
			FindAllOwnedBy(gomock.Any(), strategyScriptOwnerID).Return(nil, storageFailure)

		_, err := fixture.strategyScriptApplication.ListAvailableStrategyScripts(t.Context(), strategyScriptOwnerID)

		require.ErrorIs(t, err, storageFailure)
	})
}

func TestStrategyScriptApplicationUpdateStrategyScript(t *testing.T) {
	t.Run("rewrites the strategy script the write names", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScript(7, "二十根均線"), nil)
		fixture.strategyScriptRepository.EXPECT().
			Update(gomock.Any(), gomock.Any()).
			DoAndReturn(func(_ context.Context, strategyScript entities.StrategyScript) (entities.StrategyScript, error) {
				assert.Equal(t, uint(7), strategyScript.ID)
				assert.Equal(t, "六十根均線", strategyScript.Name)
				assert.Equal(t, "boolList", strategyScript.ResultType)

				return aStoredStrategyScript(7, strategyScript.Name), nil
			})

		writeDto := aStrategyScriptWrite()
		writeDto.ID = 7
		writeDto.Name = "六十根均線"
		writeDto.ResultType = "boolList"

		strategyScriptDto, err := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

		require.NoError(t, err)
		assert.Equal(t, uint(7), strategyScriptDto.ID)
		assert.Equal(t, "六十根均線", strategyScriptDto.Name)
	})

	t.Run("reports a strategy script that is not there", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(entities.StrategyScript{}, domains.ErrStrategyScriptNotFound)

		writeDto := aStrategyScriptWrite()
		writeDto.ID = 7

		_, err := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

		require.ErrorIs(t, err, domains.ErrStrategyScriptNotFound)
	})

	t.Run("a strategy script that is not there is reported as such, not as bad content", func(t *testing.T) {
		// Both are wrong: no strategy script carries this identifier, and the content has
		// no name. Answering about the name would send the caller off to fix it,
		// after which there is still nothing to rewrite.
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(999999)).Return(entities.StrategyScript{}, domains.ErrStrategyScriptNotFound)

		writeDto := aStrategyScriptWrite()
		writeDto.ID = 999999
		writeDto.Name = ""

		_, err := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

		require.ErrorIs(t, err, domains.ErrStrategyScriptNotFound)
		assert.NotErrorIs(t, err, domains.ErrStrategyScriptValidation)
	})

	t.Run("reports a name another strategy script already holds", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScript(7, "二十根均線"), nil)
		fixture.strategyScriptRepository.EXPECT().
			Update(gomock.Any(), gomock.Any()).Return(entities.StrategyScript{}, domains.ErrStrategyScriptNameConflict)

		writeDto := aStrategyScriptWrite()
		writeDto.ID = 7

		_, err := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

		require.ErrorIs(t, err, domains.ErrStrategyScriptNameConflict)
	})

	t.Run("refuses a rewrite that names no strategy script without writing anything", func(t *testing.T) {
		// No strategy script carries no identifier. Nothing is stubbed on the repository, so
		// a write that went out anyway — which names no row, and whose blast radius
		// is then the storage layer's decision — fails the test.
		fixture := newStrategyScriptApplicationUnderTest(t)
		writeDto := aStrategyScriptWrite()
		writeDto.ID = 0

		_, err := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

		require.ErrorIs(t, err, domains.ErrStrategyScriptNotFound)
	})
}

func TestStrategyScriptApplicationDeleteStrategyScript(t *testing.T) {
	t.Run("removes the strategy script", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(aStoredStrategyScript(7, "二十根均線"), nil)
		fixture.strategyScriptRepository.EXPECT().Delete(gomock.Any(), uint(7)).Return(nil)

		require.NoError(t, fixture.strategyScriptApplication.DeleteStrategyScript(t.Context(), strategyScriptOwnerID, 7))
	})

	t.Run("reports a strategy script that is not there", func(t *testing.T) {
		fixture := newStrategyScriptApplicationUnderTest(t)
		fixture.strategyScriptRepository.EXPECT().
			FindOne(gomock.Any(), uint(7)).Return(entities.StrategyScript{}, domains.ErrStrategyScriptNotFound)

		err := fixture.strategyScriptApplication.DeleteStrategyScript(t.Context(), strategyScriptOwnerID, 7)

		require.ErrorIs(t, err, domains.ErrStrategyScriptNotFound)
	})
}

// A rewrite naming no strategy script is refused without looking — the case above keeps
// that promise — but it used to be refused with the sentinel's own English wording
// while every other refusal spoke to the reader. The sentence is now written in one
// place, so the refusal a caller meets here and the one the store gives are the
// same sentence.
func TestStrategyScriptApplicationUpdateWithNoIdentifierSpeaksTheLanguageEveryOtherRefusalSpeaks(t *testing.T) {
	// Nothing is stubbed on the repository: nothing may reach storage.
	fixture := newStrategyScriptApplicationUnderTest(t)
	writeDto := aStrategyScriptWrite()
	writeDto.ID = 0

	_, err := fixture.strategyScriptApplication.UpdateStrategyScript(t.Context(), writeDto)

	require.ErrorIs(t, err, domains.ErrStrategyScriptNotFound)
	assert.Equal(t, "strategy script not found: 找不到識別碼為 0 的策略腳本", err.Error())
	assert.NotEqual(t, domains.ErrStrategyScriptNotFound.Error(), err.Error())
}
