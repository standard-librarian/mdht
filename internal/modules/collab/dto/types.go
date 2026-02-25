package dto

import "time"

type WorkspaceOutput struct {
	ID        string
	Name      string
	CreatedAt time.Time
}

type WorkspaceShowOutput struct {
	Workspace WorkspaceOutput
	NodeID    string
	Peers     int
}

type PeerOutput struct {
	PeerID    string
	Address   string
	AddedAt   time.Time
	LastError string
}

type StatusOutput struct {
	Online      bool
	PeerCount   int
	PendingOps  int
	LastSyncAt  time.Time
	NodeID      string
	WorkspaceID string
	ListenAddrs []string
	Counters    ValidationCountersOutput
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
