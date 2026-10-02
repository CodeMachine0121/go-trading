package _interface

import (
	"context"
	"time"
)

//go:generate go tool mockgen -source=i_replica_heartbeat_repository.go -destination=mocks/mock_i_replica_heartbeat_repository.go -package=mocks

// IReplicaHeartbeatRepository records which replicas are alive.
type IReplicaHeartbeatRepository interface {
	// Beat records that replicaName was alive at seenAt.
	Beat(executionContext context.Context, replicaName string, seenAt time.Time) error

	// FindSeenSince returns every replica that beat at or after since.
	FindSeenSince(executionContext context.Context, since time.Time) ([]string, error)
}
