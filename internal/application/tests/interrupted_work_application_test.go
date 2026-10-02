package application_test

import (
	"errors"
	"testing"
	"time"

	"github.com/CodeMachine0121/go-trading/internal/application"
	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
	"github.com/CodeMachine0121/go-trading/internal/domain/interface/mocks"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/domains"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/models/vo"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

const thisSweepingReplica = "replica-this"

var sweptAt = time.Date(2026, 10, 3, 8, 0, 0, 0, time.UTC)

type interruptedWorkUnderTest struct {
	interruptedWorkApplication    *application.InterruptedWorkApplication
	replicaHeartbeatRepository    *mocks.MockIReplicaHeartbeatRepository
	conversationRepository        *mocks.MockIConversationRepository
	historySyncRunRepository      *mocks.MockIKCandleHistorySyncRunRepository
	contractHistorySyncRepository *mocks.MockIKCandleContractHistorySyncRunRepository
}

func newInterruptedWorkUnderTest(t *testing.T) interruptedWorkUnderTest {
	controller := gomock.NewController(t)
	replicaHeartbeatRepository := mocks.NewMockIReplicaHeartbeatRepository(controller)
	conversationRepository := mocks.NewMockIConversationRepository(controller)
	historySyncRunRepository := mocks.NewMockIKCandleHistorySyncRunRepository(controller)
	contractHistorySyncRepository := mocks.NewMockIKCandleContractHistorySyncRunRepository(controller)
	clockProxy := mocks.NewMockIClockProxy(controller)
	clockProxy.EXPECT().Now().Return(sweptAt).AnyTimes()
	catalog := domains.NewMarketCatalogDomain(map[vo.MarketVo]vo.MarketRulesVo{vo.MarketCrypto: {}})

	return interruptedWorkUnderTest{
		interruptedWorkApplication: application.NewInterruptedWorkApplication(
			service.NewReplicaPresenceService(replicaHeartbeatRepository, clockProxy, thisSweepingReplica, 10*time.Second),
			service.NewAssistantConversationService(conversationRepository, mocks.NewMockIAssistantProxy(controller),
				[]domaininterface.IAssistantQuery{}, clockProxy, 20, 8, 300000, 2000, thisSweepingReplica),
			service.NewKCandleIngestionService(mocks.NewMockIKCandleRepository(controller), historySyncRunRepository,
				mocks.NewMockITradingSymbolRepository(controller), mocks.NewMockIMarketDataProxy(controller), clockProxy,
				catalog, 5, 24*time.Hour, 2, thisSweepingReplica),
			service.NewContractKCandleIngestionService(mocks.NewMockIKCandleContractRepository(controller),
				contractHistorySyncRepository, mocks.NewMockIContractTradingSymbolRepository(controller),
				mocks.NewMockIContractMarketDataProxy(controller), clockProxy, catalog, 5, 24*time.Hour, nil, 2,
				thisSweepingReplica)),
		replicaHeartbeatRepository:    replicaHeartbeatRepository,
		conversationRepository:        conversationRepository,
		historySyncRunRepository:      historySyncRunRepository,
		contractHistorySyncRepository: contractHistorySyncRepository,
	}
}

// expectSweptOutside expects all three kinds of work swept, sparing exactly liveReplicaNames.
func (underTest interruptedWorkUnderTest) expectSweptOutside(liveReplicaNames []string) {
	underTest.conversationRepository.EXPECT().
		FailRunningTurnsOutside(gomock.Any(), liveReplicaNames, gomock.Any()).Return(1, nil)
	underTest.historySyncRunRepository.EXPECT().
		FailRunningOutside(gomock.Any(), liveReplicaNames, gomock.Any(), sweptAt).Return(2, nil)
	underTest.contractHistorySyncRepository.EXPECT().
		FailRunningOutside(gomock.Any(), liveReplicaNames, gomock.Any(), sweptAt).Return(3, nil)
}

func TestInterruptedWorkApplicationAtStartupSparesOnlyOtherLiveReplicas(t *testing.T) {
	underTest := newInterruptedWorkUnderTest(t)
	// Three missed beats of ten seconds make a replica gone; this one is left out of the spared list.
	underTest.replicaHeartbeatRepository.EXPECT().FindSeenSince(gomock.Any(), sweptAt.Add(-30*time.Second)).
		Return([]string{"replica-other", thisSweepingReplica}, nil)
	underTest.expectSweptOutside([]string{"replica-other"})

	interrupted, sweepError := underTest.interruptedWorkApplication.FailWorkLeftByLastRun(t.Context())

	require.NoError(t, sweepError)
	assert.Equal(t, dto.InterruptedWorkDto{Answers: 1, HistorySyncs: 2, ContractHistorySyncs: 3}, interrupted)
}

func TestInterruptedWorkApplicationWhileServingSparesThisReplicaToo(t *testing.T) {
	underTest := newInterruptedWorkUnderTest(t)
	underTest.replicaHeartbeatRepository.EXPECT().FindSeenSince(gomock.Any(), gomock.Any()).
		Return([]string{"replica-other"}, nil)
	underTest.expectSweptOutside([]string{"replica-other", thisSweepingReplica})

	_, sweepError := underTest.interruptedWorkApplication.FailWorkOfVanishedReplicas(t.Context())

	require.NoError(t, sweepError)
}

func TestInterruptedWorkApplicationSweepsNothingWhenItCannotTellWhoIsAlive(t *testing.T) {
	underTest := newInterruptedWorkUnderTest(t)
	underTest.replicaHeartbeatRepository.EXPECT().FindSeenSince(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("storage unavailable"))
	// No sweep expectations: not knowing who is alive must never fail someone's work in flight.

	_, startupError := underTest.interruptedWorkApplication.FailWorkLeftByLastRun(t.Context())
	underTest.replicaHeartbeatRepository.EXPECT().FindSeenSince(gomock.Any(), gomock.Any()).
		Return(nil, errors.New("storage unavailable"))
	_, servingError := underTest.interruptedWorkApplication.FailWorkOfVanishedReplicas(t.Context())

	assert.Error(t, startupError)
	assert.Error(t, servingError)
}

func TestInterruptedWorkApplicationStopsAtTheFirstSweepThatFails(t *testing.T) {
	testCases := []struct {
		name     string
		arrange  func(underTest interruptedWorkUnderTest)
		expected dto.InterruptedWorkDto
	}{
		{
			name: "answers could not be swept",
			arrange: func(underTest interruptedWorkUnderTest) {
				underTest.conversationRepository.EXPECT().FailRunningTurnsOutside(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(0, errors.New("storage unavailable"))
			},
			expected: dto.InterruptedWorkDto{},
		},
		{
			name: "history syncs could not be swept",
			arrange: func(underTest interruptedWorkUnderTest) {
				underTest.conversationRepository.EXPECT().FailRunningTurnsOutside(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(1, nil)
				underTest.historySyncRunRepository.EXPECT().FailRunningOutside(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(0, errors.New("storage unavailable"))
			},
			expected: dto.InterruptedWorkDto{Answers: 1},
		},
		{
			name: "contract history syncs could not be swept",
			arrange: func(underTest interruptedWorkUnderTest) {
				underTest.conversationRepository.EXPECT().FailRunningTurnsOutside(gomock.Any(), gomock.Any(), gomock.Any()).
					Return(1, nil)
				underTest.historySyncRunRepository.EXPECT().FailRunningOutside(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(2, nil)
				underTest.contractHistorySyncRepository.EXPECT().FailRunningOutside(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return(0, errors.New("storage unavailable"))
			},
			expected: dto.InterruptedWorkDto{Answers: 1, HistorySyncs: 2},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			underTest := newInterruptedWorkUnderTest(t)
			underTest.replicaHeartbeatRepository.EXPECT().FindSeenSince(gomock.Any(), gomock.Any()).Return(nil, nil)
			testCase.arrange(underTest)

			interrupted, sweepError := underTest.interruptedWorkApplication.FailWorkOfVanishedReplicas(t.Context())

			assert.Error(t, sweepError)
			assert.Equal(t, testCase.expected, interrupted)
		})
	}
}

func TestInterruptedWorkApplicationBeatsUnderThisReplicasName(t *testing.T) {
	underTest := newInterruptedWorkUnderTest(t)
	underTest.replicaHeartbeatRepository.EXPECT().Beat(gomock.Any(), thisSweepingReplica, sweptAt).Return(nil)

	require.NoError(t, underTest.interruptedWorkApplication.BeatHeartbeat(t.Context()))
}
