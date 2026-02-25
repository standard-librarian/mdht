package out

import (
	"context"
	"time"

	"mdht/internal/modules/collab/domain"
)

type WorkspaceStore interface {
	Init(ctx context.Context, name string) (domain.Workspace, []byte, domain.NodeIdentity, error)
	Load(ctx context.Context) (domain.Workspace, []byte, domain.NodeIdentity, error)
}

type PeerStore interface {
	Add(ctx context.Context, addr string) (domain.Peer, error)
	Remove(ctx context.Context, peerID string) error
	List(ctx context.Context) ([]domain.Peer, error)
}

type OpLogStore interface {
	Append(ctx context.Context, op domain.OpEnvelope) error
	List(ctx context.Context) ([]domain.OpEnvelope, error)
}

type SnapshotStore interface {
	Load(ctx context.Context) (domain.CRDTState, error)
	Save(ctx context.Context, state domain.CRDTState) error
}

type ProjectionExtractor interface {
	Extract(ctx context.Context, workspaceID, nodeID string, now time.Time) ([]domain.OpEnvelope, error)
}

type ProjectionApplier interface {
	Apply(ctx context.Context, state domain.CRDTState) error
}

type Transport interface {
	Start(ctx context.Context, input TransportStartInput, handlers TransportHandlers) (RuntimeTransport, error)
}

type TransportStartInput struct {
	WorkspaceID  string
	WorkspaceKey []byte
	NodeIdentity domain.NodeIdentity
	Peers        []domain.Peer
}

type TransportHandlers struct {
	OnOp     func(op domain.OpEnvelope)
	OnStatus func(status NetworkStatus)
}

type RuntimeTransport interface {
	Broadcast(ctx context.Context, op domain.OpEnvelope) error
	Reconcile(ctx context.Context, ops []domain.OpEnvelope) error
	AddPeer(ctx context.Context, peer domain.Peer) error
	RemovePeer(ctx context.Context, peerID string) error
	Status() NetworkStatus
	Stop() error
}

type NetworkStatus struct {
	Online      bool
	PeerCount   int
	LastSyncAt  time.Time
	ListenAddrs []string
}

type DaemonStore interface {
	WritePID(ctx context.Context, pid int) error
	ReadPID(ctx context.Context) (int, error)
	ClearPID(ctx context.Context) error
	SocketPath() string
	LogPath() string
}

// IPCServer serves the stable JSON-RPC daemon API.
type IPCServer interface {
	Serve(ctx context.Context, socketPath string, handler IPCHandler) error
}

// IPCClient talks to the local daemon JSON-RPC API.
type IPCClient interface {
	WorkspaceInit(ctx context.Context, socketPath string, name string) (domain.Workspace, error)
	WorkspaceShow(ctx context.Context, socketPath string) (domain.Workspace, string, []domain.Peer, error)
	PeerAdd(ctx context.Context, socketPath string, addr string) (domain.Peer, error)
	PeerRemove(ctx context.Context, socketPath string, peerID string) error
	PeerList(ctx context.Context, socketPath string) ([]domain.Peer, error)
	Status(ctx context.Context, socketPath string) (DaemonStatus, error)
	ReconcileNow(ctx context.Context, socketPath string) (int, error)
	ExportState(ctx context.Context, socketPath string) (string, error)
	Stop(ctx context.Context, socketPath string) error
}

type IPCHandler interface {
	WorkspaceInit(ctx context.Context, name string) (domain.Workspace, error)
	WorkspaceShow(ctx context.Context) (domain.Workspace, string, []domain.Peer, error)
	PeerAdd(ctx context.Context, addr string) (domain.Peer, error)
	PeerRemove(ctx context.Context, peerID string) error
	PeerList(ctx context.Context) ([]domain.Peer, error)
	Status(ctx context.Context) (DaemonStatus, error)
	ReconcileNow(ctx context.Context) (int, error)
	ExportState(ctx context.Context) (string, error)
	Stop(ctx context.Context) error
}

type DaemonStatus struct {
	Online      bool
	PeerCount   int
	PendingOps  int
	LastSyncAt  time.Time
	NodeID      string
	WorkspaceID string
	ListenAddrs []string
}
