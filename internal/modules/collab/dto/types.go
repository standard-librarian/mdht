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
}

type DaemonStatusOutput struct {
	Running    bool
	PID        int
	SocketPath string
	Status     StatusOutput
}

type ReconcileOutput struct {
	Applied int
}

type ExportStateOutput struct {
	Payload string
}
