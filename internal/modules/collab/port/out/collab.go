package out

import (
	"context"
	"time"

	"mdht/internal/modules/collab/domain"
)

type WorkspaceStore interface {
	Init(ctx context.Context, name string) (domain.Workspace, domain.KeyRing, domain.NodeIdentity, error)
	Load(ctx context.Context) (domain.Workspace, domain.KeyRing, domain.NodeIdentity, error)
	RotateKey(ctx context.Context, gracePeriod time.Duration) (domain.KeyRing, error)
}

type PeerStore interface {
	Add(ctx context.Context, addr, label string) (domain.Peer, error)
	Approve(ctx context.Context, peerID string) (domain.Peer, error)
	Revoke(ctx context.Context, peerID string) (domain.Peer, error)
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
	Extract(ctx context.Context, workspaceID, nodeID, keyID string, now time.Time) ([]domain.OpEnvelope, error)
}

type ProjectionApplier interface {
	Apply(ctx context.Context, state domain.CRDTState) error
}

type Transport interface {
	Start(ctx context.Context, input TransportStartInput, handlers TransportHandlers) (RuntimeTransport, error)
}

type TransportStartInput struct {
	WorkspaceID  string
	KeyRing      domain.KeyRing
	NodeIdentity domain.NodeIdentity
	Peers        []domain.Peer
}

type TransportHandlers struct {
	OnOp     func(op domain.OpEnvelope)
	OnStatus func(status NetworkStatus)
}

type ValidationCounters struct {
	InvalidAuthTag      int64
	WorkspaceMismatch   int64
	UnauthenticatedPeer int64
	DecodeErrors        int64
	BroadcastSendErrors int64
	ReconcileSendErrors int64
	ReconnectAttempts   int64
	ReconnectSuccesses  int64
}

type RuntimeTransport interface {
	Broadcast(ctx context.Context, op domain.OpEnvelope) error
	Reconcile(ctx context.Context, ops []domain.OpEnvelope) error
	AddPeer(ctx context.Context, peer domain.Peer) error
	ApprovePeer(ctx context.Context, peer domain.Peer) error
	RevokePeer(ctx context.Context, peerID string) error
	RemovePeer(ctx context.Context, peerID string) error
	UpdateKeyRing(ctx context.Context, ring domain.KeyRing) error
	Status() NetworkStatus
	Stop() error
}

type NetworkStatus struct {
	Online      bool
	PeerCount   int
	LastSyncAt  time.Time
	ListenAddrs []string
	Counters    ValidationCounters
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
	WorkspaceRotateKey(ctx context.Context, socketPath string, gracePeriod time.Duration) (domain.Workspace, error)
	PeerAdd(ctx context.Context, socketPath string, addr, label string) (domain.Peer, error)
	PeerApprove(ctx context.Context, socketPath string, peerID string) (domain.Peer, error)
	PeerRevoke(ctx context.Context, socketPath string, peerID string) (domain.Peer, error)
	PeerRemove(ctx context.Context, socketPath string, peerID string) error
	PeerList(ctx context.Context, socketPath string) ([]domain.Peer, error)
	Status(ctx context.Context, socketPath string) (DaemonStatus, error)
	ActivityTail(ctx context.Context, socketPath string, query ActivityQuery) ([]domain.ActivityEvent, error)
	ConflictsList(ctx context.Context, socketPath string, entityKey string) ([]domain.ConflictRecord, error)
	ConflictResolve(ctx context.Context, socketPath, conflictID string, strategy domain.ConflictStrategy) (domain.ConflictRecord, error)
	SyncNow(ctx context.Context, socketPath string) (int, error)
	SnapshotExport(ctx context.Context, socketPath string) (string, error)
	Metrics(ctx context.Context, socketPath string) (MetricsSnapshot, error)
	Stop(ctx context.Context, socketPath string) error
}

type IPCHandler interface {
	WorkspaceInit(ctx context.Context, name string) (domain.Workspace, error)
	WorkspaceShow(ctx context.Context) (domain.Workspace, string, []domain.Peer, error)
	WorkspaceRotateKey(ctx context.Context, gracePeriod time.Duration) (domain.Workspace, error)
	PeerAdd(ctx context.Context, addr, label string) (domain.Peer, error)
	PeerApprove(ctx context.Context, peerID string) (domain.Peer, error)
	PeerRevoke(ctx context.Context, peerID string) (domain.Peer, error)
	PeerRemove(ctx context.Context, peerID string) error
	PeerList(ctx context.Context) ([]domain.Peer, error)
	Status(ctx context.Context) (DaemonStatus, error)
	ActivityTail(ctx context.Context, query ActivityQuery) ([]domain.ActivityEvent, error)
	ConflictsList(ctx context.Context, entityKey string) ([]domain.ConflictRecord, error)
	ConflictResolve(ctx context.Context, conflictID string, strategy domain.ConflictStrategy) (domain.ConflictRecord, error)
	SyncNow(ctx context.Context) (int, error)
	SnapshotExport(ctx context.Context) (string, error)
	Metrics(ctx context.Context) (MetricsSnapshot, error)
	Stop(ctx context.Context) error
}

type DaemonStatus struct {
	Online            bool
	PeerCount         int
	ApprovedPeerCount int
	PendingConflicts  int
	PendingOps        int
	LastSyncAt        time.Time
	NodeID            string
	WorkspaceID       string
	ListenAddrs       []string
	Counters          ValidationCounters
	MetricsAddress    string
}

type DaemonRuntimeStatus struct {
	Running    bool
	PID        int
	SocketPath string
	Status     DaemonStatus
}

type DoctorCheck struct {
	Name    string
	OK      bool
	Details string
}

type ActivityQuery struct {
	Since time.Time
	Limit int
}

type ActivityStore interface {
	Append(ctx context.Context, event domain.ActivityEvent) error
	Tail(ctx context.Context, query ActivityQuery) ([]domain.ActivityEvent, error)
}

type ConflictStore interface {
	List(ctx context.Context, entityKey string) ([]domain.ConflictRecord, error)
	Upsert(ctx context.Context, record domain.ConflictRecord) error
}

type MetricsSnapshot struct {
	WorkspaceID      string             `json:"workspace_id"`
	NodeID           string             `json:"node_id"`
	ApprovedPeers    int                `json:"approved_peers"`
	PendingConflicts int                `json:"pending_conflicts"`
	PendingOps       int                `json:"pending_ops"`
	LastSyncAt       time.Time          `json:"last_sync_at"`
	Counters         ValidationCounters `json:"counters"`
	CollectedAt      time.Time          `json:"collected_at"`
}
