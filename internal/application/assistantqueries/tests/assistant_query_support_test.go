package assistantqueries_test

import (
	"context"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	"github.com/CodeMachine0121/go-trading/internal/application/assistantqueries"
	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/entities"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/shopspring/decimal"
	"go.uber.org/mock/gomock"
)

// queryMaxResults is large enough never to interfere with what a case checks.
const queryMaxResults = 1000

// at is a moment on 2026-08-29.
func at(hour int, minute int) time.Time {
	return time.Date(2026, 8, 29, hour, minute, 0, 0, time.UTC)
}

func kCandleAt(openTime time.Time, closePrice string) entities.KCandle {
	return entities.KCandle{
		Symbol:              "BTCUSDT",
		OpenTime:            openTime,
		Open:                decimal.RequireFromString("100"),
		High:                decimal.RequireFromString("120"),
		Low:                 decimal.RequireFromString("90"),
		Close:               decimal.RequireFromString(closePrice),
		Volume:              decimal.RequireFromString("11"),
		QuoteVolume:         decimal.NewNullDecimal(decimal.RequireFromString("1200")),
		TakerBuyBaseVolume:  decimal.NewNullDecimal(decimal.RequireFromString("5")),
		TakerBuyQuoteVolume: decimal.NewNullDecimal(decimal.RequireFromString("600")),
	}
}

// indicatorNow sits on a five-minute edge, so the 09:10 candle's bucket has finished.
var indicatorNow = at(9, 15)

// assistantViewerID owns every strategy script these tests save or read, so they test capabilities rather than visibility.
const assistantViewerID = uint(1)

// assistantOrigin is a lookup asked by the viewer from one answer of one conversation.
var assistantOrigin = vo.AssistantQueryOriginVo{ViewerID: assistantViewerID, ConversationID: 5, TurnID: 50}

// revisionProposedAt is when every proposal in these tests is made.
var revisionProposedAt = time.Date(2026, 9, 26, 8, 0, 0, 0, time.UTC)

// assistantRevisionUnderTest mocks only the storage of proposals and of what the assistant created, so the
// proposal rules and the ordinary rewrite rules run for real.
type assistantRevisionUnderTest struct {
	application               *application.AssistantRevisionApplication
	pendingRevisionRepository *mocks.MockIAssistantPendingRevisionRepository
	createdSubjectRepository  *mocks.MockIAssistantCreatedSubjectRepository
	// createdInConversation is what every "did the assistant create it here" lookup answers; false unless set.
	createdInConversation *bool
	// askedAboutConversations and createdSubjects record what reached the created-subject storage.
	askedAboutConversations *[]uint
	createdSubjects         *[]entities.AssistantCreatedSubject
	// createdSubjectFailure is what remembering a creation fails with; nothing unless a test says otherwise.
	createdSubjectFailure *error
}

func newAssistantRevisionUnderTest(
	controller *gomock.Controller,
	strategyScriptApplication *application.StrategyScriptApplication,
	tradingStrategyApplication *application.TradingStrategyApplication,
) assistantRevisionUnderTest {
	pendingRevisionRepository := mocks.NewMockIAssistantPendingRevisionRepository(controller)
	createdSubjectRepository := mocks.NewMockIAssistantCreatedSubjectRepository(controller)
	createdInConversation := new(bool)
	askedAboutConversations := &[]uint{}
	createdSubjects := &[]entities.AssistantCreatedSubject{}
	createdSubjectFailure := new(error)
	createdSubjectRepository.EXPECT().Exists(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, conversationID uint, _ string, _ uint) (bool, error) {
			*askedAboutConversations = append(*askedAboutConversations, conversationID)

			return *createdInConversation, nil
		}).AnyTimes()
	createdSubjectRepository.EXPECT().Save(gomock.Any(), gomock.Any()).
		DoAndReturn(func(_ context.Context, createdSubject entities.AssistantCreatedSubject) error {
			*createdSubjects = append(*createdSubjects, createdSubject)

			return *createdSubjectFailure
		}).AnyTimes()
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(revisionProposedAt).AnyTimes()

	return assistantRevisionUnderTest{
		application: application.NewAssistantRevisionApplication(
			service.NewAssistantRevisionService(pendingRevisionRepository, createdSubjectRepository, clockProxy),
			[]domaininterface.IAssistantRevisionApplier{
				assistantqueries.NewStrategyScriptRevisionApplier(strategyScriptApplication),
				assistantqueries.NewTradingStrategyRevisionApplier(tradingStrategyApplication),
			}),
		pendingRevisionRepository: pendingRevisionRepository,
		createdSubjectRepository:  createdSubjectRepository,
		createdInConversation:     createdInConversation,
		askedAboutConversations:   askedAboutConversations,
		createdSubjects:           createdSubjects,
		createdSubjectFailure:     createdSubjectFailure,
	}
}
