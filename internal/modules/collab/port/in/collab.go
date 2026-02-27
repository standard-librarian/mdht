package in

import (
	"context"
	"time"

	"mdht/internal/modules/collab/dto"
)

type Usecase interface {
	RunDaemon(ctx context.Context) error
	StartDaemon(ctx context.Context) error
	StopDaemon(ctx context.Context) error
	DaemonStatus(ctx context.Context) (dto.DaemonStatusOutput, error)

	WorkspaceInit(ctx context.Context, name string) (dto.WorkspaceOutput, error)
	WorkspaceShow(ctx context.Context) (dto.WorkspaceShowOutput, error)
	WorkspaceRotateKey(ctx context.Context, gracePeriod time.Duration) (dto.WorkspaceOutput, error)

	PeerAdd(ctx context.Context, addr, label string) (dto.PeerOutput, error)
	PeerApprove(ctx context.Context, peerID string) (dto.PeerOutput, error)
	PeerRevoke(ctx context.Context, peerID string) (dto.PeerOutput, error)
	PeerRemove(ctx context.Context, peerID string) error
	PeerList(ctx context.Context) ([]dto.PeerOutput, error)

	Status(ctx context.Context) (dto.StatusOutput, error)
	DaemonLogs(ctx context.Context, tail int) (string, error)
	ActivityTail(ctx context.Context, since time.Time, limit int) ([]dto.ActivityOutput, error)
	ConflictsList(ctx context.Context, entityKey string) ([]dto.ConflictOutput, error)
	ConflictResolve(ctx context.Context, conflictID, strategy string) (dto.ConflictOutput, error)
	SyncNow(ctx context.Context) (dto.ReconcileOutput, error)
	SnapshotExport(ctx context.Context) (dto.ExportStateOutput, error)
	Metrics(ctx context.Context) (dto.MetricsOutput, error)
}
