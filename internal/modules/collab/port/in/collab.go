package in

import (
	"context"

	"mdht/internal/modules/collab/dto"
)

type Usecase interface {
	RunDaemon(ctx context.Context) error
	StartDaemon(ctx context.Context) error
	StopDaemon(ctx context.Context) error
	DaemonStatus(ctx context.Context) (dto.DaemonStatusOutput, error)

	WorkspaceInit(ctx context.Context, name string) (dto.WorkspaceOutput, error)
	WorkspaceShow(ctx context.Context) (dto.WorkspaceShowOutput, error)

	PeerAdd(ctx context.Context, addr string) (dto.PeerOutput, error)
	PeerRemove(ctx context.Context, peerID string) error
	PeerList(ctx context.Context) ([]dto.PeerOutput, error)

	Status(ctx context.Context) (dto.StatusOutput, error)
	Doctor(ctx context.Context) (dto.DoctorOutput, error)
	DaemonLogs(ctx context.Context, tail int) (string, error)
	ReconcileNow(ctx context.Context) (dto.ReconcileOutput, error)
	ExportState(ctx context.Context) (dto.ExportStateOutput, error)
}
