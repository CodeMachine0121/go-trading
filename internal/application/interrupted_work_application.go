package application

import (
	"context"

	"github.com/CodeMachine0121/go-trading/internal/domain/models/dto"
	"github.com/CodeMachine0121/go-trading/internal/domain/service"
)

// InterruptedWorkApplication marks as failed what a vanished replica was in the middle of: assistant answers and history syncs live only in the replica that started them.
type InterruptedWorkApplication struct {
	replicaPresenceService          *service.ReplicaPresenceService
	assistantConversationService    *service.AssistantConversationService
	kCandleIngestionService         *service.KCandleIngestionService
	contractKCandleIngestionService *service.ContractKCandleIngestionService
}

func NewInterruptedWorkApplication(
	replicaPresenceService *service.ReplicaPresenceService,
	assistantConversationService *service.AssistantConversationService,
	kCandleIngestionService *service.KCandleIngestionService,
	contractKCandleIngestionService *service.ContractKCandleIngestionService,
) *InterruptedWorkApplication {
	return &InterruptedWorkApplication{
		replicaPresenceService:          replicaPresenceService,
		assistantConversationService:    assistantConversationService,
		kCandleIngestionService:         kCandleIngestionService,
		contractKCandleIngestionService: contractKCandleIngestionService,
	}
}

// BeatHeartbeat says this replica is still alive, so its own work is never taken for interrupted.
func (interruptedWorkApplication *InterruptedWorkApplication) BeatHeartbeat(executionContext context.Context) error {
	return interruptedWorkApplication.replicaPresenceService.Beat(executionContext)
}

// FailWorkLeftByLastRun runs at startup and spares only other live replicas: with a fixed REPLICA_NAME, anything under this replica's own name was left by its previous run.
// With the default name, which changes on every start, the previous run looks like another replica until it misses three beats; the sweep on duty catches it then.
func (interruptedWorkApplication *InterruptedWorkApplication) FailWorkLeftByLastRun(
	executionContext context.Context,
) (dto.InterruptedWorkDto, error) {
	otherLiveReplicaNames, findError := interruptedWorkApplication.replicaPresenceService.
		OtherLiveReplicaNames(executionContext)
	if findError != nil {
		return dto.InterruptedWorkDto{}, findError
	}

	return interruptedWorkApplication.failWorkOutside(executionContext, otherLiveReplicaNames)
}

// FailWorkOfVanishedReplicas runs while serving: this replica and every other one still beating are spared.
func (interruptedWorkApplication *InterruptedWorkApplication) FailWorkOfVanishedReplicas(
	executionContext context.Context,
) (dto.InterruptedWorkDto, error) {
	otherLiveReplicaNames, findError := interruptedWorkApplication.replicaPresenceService.
		OtherLiveReplicaNames(executionContext)
	if findError != nil {
		return dto.InterruptedWorkDto{}, findError
	}

	return interruptedWorkApplication.failWorkOutside(executionContext,
		append(otherLiveReplicaNames, interruptedWorkApplication.replicaPresenceService.ThisReplicaName()))
}

// failWorkOutside is shared by both sweeps, which differ only in whether this replica's own work is spared.
func (interruptedWorkApplication *InterruptedWorkApplication) failWorkOutside(
	executionContext context.Context, liveReplicaNames []string,
) (dto.InterruptedWorkDto, error) {
	answerCount, answerError := interruptedWorkApplication.assistantConversationService.
		FailInterruptedAnswers(executionContext, liveReplicaNames)
	if answerError != nil {
		return dto.InterruptedWorkDto{}, answerError
	}

	historySyncCount, historySyncError := interruptedWorkApplication.kCandleIngestionService.
		FailInterruptedHistorySyncs(executionContext, liveReplicaNames)
	if historySyncError != nil {
		return dto.InterruptedWorkDto{Answers: answerCount}, historySyncError
	}

	contractHistorySyncCount, contractHistorySyncError := interruptedWorkApplication.contractKCandleIngestionService.
		FailInterruptedHistorySyncs(executionContext, liveReplicaNames)
	if contractHistorySyncError != nil {
		return dto.InterruptedWorkDto{Answers: answerCount, HistorySyncs: historySyncCount}, contractHistorySyncError
	}

	return dto.InterruptedWorkDto{
		Answers: answerCount, HistorySyncs: historySyncCount, ContractHistorySyncs: contractHistorySyncCount,
	}, nil
}
