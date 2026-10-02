package entities

import "time"

// ReplicaHeartbeat is a replica saying it is still alive; work a replica started is interrupted only once its heartbeat stops.
type ReplicaHeartbeat struct {
	Name       string    `gorm:"primaryKey;size:255"`
	LastSeenAt time.Time `gorm:"type:timestamptz;not null;index:idx_replica_heartbeats_last_seen_at"`
}

func (replicaHeartbeat ReplicaHeartbeat) TableName() string {
	return "ReplicaHeartbeats"
}
