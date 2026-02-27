package dto

import "time"

type WorkspaceOutput struct {
	ID            string
	Name          string
	CreatedAt     time.Time
	SchemaVersion int
}

type WorkspaceShowOutput struct {
	Workspace WorkspaceOutput
	NodeID    string
	Peers     int
}

type PeerOutput struct {
	PeerID    string
	Address   string
	Label     string
	State     string
	FirstSeen time.Time
	LastSeen  time.Time
	AddedAt   time.Time
	LastError string
}

type StatusOutput struct {
	Online            bool
	PeerCount         int
	ApprovedPeerCount int
	PendingConflicts  int
	PendingOps        int
	LastSyncAt        time.Time
	NodeID            string
	WorkspaceID       string
	ListenAddrs       []string
	MetricsAddress    string
	Counters          ValidationCountersOutput
}

type DaemonStatusOutput struct {
	Running    bool
	PID        int
	SocketPath string
	Status     StatusOutput
}

type ValidationCountersOutput struct {
	InvalidAuthTag      int64
	WorkspaceMismatch   int64
	UnauthenticatedPeer int64
	DecodeErrors        int64
	BroadcastSendErrors int64
	ReconcileSendErrors int64
	ReconnectAttempts   int64
	ReconnectSuccesses  int64
}

type DoctorCheckOutput struct {
	Name    string
	OK      bool
	Details string
}

type DoctorOutput struct {
	Checks []DoctorCheckOutput
}

type ReconcileOutput struct {
	Applied int
}

type ExportStateOutput struct {
	Payload string
}

type ActivityOutput struct {
	ID         string
	OccurredAt time.Time
	Type       string
	Message    string
	Fields     map[string]string
}

type ConflictOutput struct {
	ID          string
	EntityKey   string
	Field       string
	LocalValue  string
	RemoteValue string
	Status      string
	Strategy    string
	MergedValue string
	CreatedAt   time.Time
	ResolvedAt  time.Time
	ResolvedBy  string
}

type MetricsOutput struct {
	WorkspaceID      string
	NodeID           string
	ApprovedPeers    int
	PendingConflicts int
	PendingOps       int
	LastSyncAt       time.Time
	CollectedAt      time.Time
	Counters         ValidationCountersOutput
}
