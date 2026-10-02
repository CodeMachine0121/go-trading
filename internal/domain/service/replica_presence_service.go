package service

import (
	"context"
	"slices"
	"time"

	domaininterface "github.com/CodeMachine0121/go-trading/internal/domain/interface"
)

// replicaHeartbeatsBeforeVanished is how many beats a replica may miss before its work counts as interrupted; one late beat is not a death.
const replicaHeartbeatsBeforeVanished = 3

// ReplicaPresenceService says this replica is alive and which others are.
type ReplicaPresenceService struct {
	replicaHeartbeatRepository domaininterface.IReplicaHeartbeatRepository
	clockProxy                 domaininterface.IClockProxy
	replicaName                string
	beatInterval               time.Duration
}

func NewReplicaPresenceService(
	replicaHeartbeatRepository domaininterface.IReplicaHeartbeatRepository,
	clockProxy domaininterface.IClockProxy,
	replicaName string,
	beatInterval time.Duration,
) *ReplicaPresenceService {
	return &ReplicaPresenceService{
		replicaHeartbeatRepository: replicaHeartbeatRepository,
		clockProxy:                 clockProxy,
		replicaName:                replicaName,
		beatInterval:               beatInterval,
	}
}

func (replicaPresenceService *ReplicaPresenceService) Beat(executionContext context.Context) error {
	return replicaPresenceService.replicaHeartbeatRepository.Beat(
		executionContext, replicaPresenceService.replicaName, replicaPresenceService.clockProxy.Now())
}

// OtherLiveReplicaNames is every other replica still beating; this one is left out, so whatever it left behind under its own name before restarting counts as interrupted.
func (replicaPresenceService *ReplicaPresenceService) OtherLiveReplicaNames(
	executionContext context.Context,
) ([]string, error) {
	seenSince := replicaPresenceService.clockProxy.Now().Add(
		-replicaHeartbeatsBeforeVanished * replicaPresenceService.beatInterval)

	replicaNames, findError := replicaPresenceService.replicaHeartbeatRepository.FindSeenSince(
		executionContext, seenSince)
	if findError != nil {
		return nil, findError
	}

	return slices.DeleteFunc(replicaNames, func(replicaName string) bool {
		return replicaName == replicaPresenceService.replicaName
	}), nil
}

// ThisReplicaName is the name this replica marks its own work with.
func (replicaPresenceService *ReplicaPresenceService) ThisReplicaName() string {
	return replicaPresenceService.replicaName
}
