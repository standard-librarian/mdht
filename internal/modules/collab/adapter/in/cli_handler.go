package in

import (
	"context"
	"time"

	"mdht/internal/modules/collab/dto"
	collabin "mdht/internal/modules/collab/port/in"
)

type CLIHandler struct {
	usecase collabin.Usecase
}

func NewCLIHandler(usecase collabin.Usecase) CLIHandler {
	return CLIHandler{usecase: usecase}
}

func (h CLIHandler) RunDaemon(ctx context.Context) error {
	return h.usecase.RunDaemon(ctx)
}

func (h CLIHandler) StartDaemon(ctx context.Context) error {
	return h.usecase.StartDaemon(ctx)
}

func (h CLIHandler) StopDaemon(ctx context.Context) error {
	return h.usecase.StopDaemon(ctx)
}

func (h CLIHandler) DaemonStatus(ctx context.Context) (dto.DaemonStatusOutput, error) {
	return h.usecase.DaemonStatus(ctx)
}

func (h CLIHandler) WorkspaceInit(ctx context.Context, name string) (dto.WorkspaceOutput, error) {
	return h.usecase.WorkspaceInit(ctx, name)
}

func (h CLIHandler) WorkspaceShow(ctx context.Context) (dto.WorkspaceShowOutput, error) {
	return h.usecase.WorkspaceShow(ctx)
}

func (h CLIHandler) WorkspaceRotateKey(ctx context.Context, gracePeriod time.Duration) (dto.WorkspaceOutput, error) {
	return h.usecase.WorkspaceRotateKey(ctx, gracePeriod)
}

func (h CLIHandler) PeerAdd(ctx context.Context, addr, label string) (dto.PeerOutput, error) {
	return h.usecase.PeerAdd(ctx, addr, label)
}

func (h CLIHandler) PeerApprove(ctx context.Context, peerID string) (dto.PeerOutput, error) {
	return h.usecase.PeerApprove(ctx, peerID)
}

func (h CLIHandler) PeerRevoke(ctx context.Context, peerID string) (dto.PeerOutput, error) {
	return h.usecase.PeerRevoke(ctx, peerID)
}

func (h CLIHandler) PeerDial(ctx context.Context, peerID string) (dto.PeerOutput, error) {
	return h.usecase.PeerDial(ctx, peerID)
}

func (h CLIHandler) PeerLatency(ctx context.Context) ([]dto.PeerLatencyOutput, error) {
	return h.usecase.PeerLatency(ctx)
}

func (h CLIHandler) PeerRemove(ctx context.Context, peerID string) error {
	return h.usecase.PeerRemove(ctx, peerID)
}

func (h CLIHandler) PeerList(ctx context.Context) ([]dto.PeerOutput, error) {
	return h.usecase.PeerList(ctx)
}

func (h CLIHandler) Status(ctx context.Context) (dto.StatusOutput, error) {
	return h.usecase.Status(ctx)
}

func (h CLIHandler) NetStatus(ctx context.Context) (dto.NetStatusOutput, error) {
	return h.usecase.NetStatus(ctx)
}

func (h CLIHandler) NetProbe(ctx context.Context) (dto.NetProbeOutput, error) {
	return h.usecase.NetProbe(ctx)
}

func (h CLIHandler) DaemonLogs(ctx context.Context, tail int) (string, error) {
	return h.usecase.DaemonLogs(ctx, tail)
}

func (h CLIHandler) ActivityTail(ctx context.Context, since time.Time, limit int) ([]dto.ActivityOutput, error) {
	return h.usecase.ActivityTail(ctx, since, limit)
}

func (h CLIHandler) ConflictsList(ctx context.Context, entityKey string) ([]dto.ConflictOutput, error) {
	return h.usecase.ConflictsList(ctx, entityKey)
}

func (h CLIHandler) ConflictResolve(ctx context.Context, conflictID, strategy string) (dto.ConflictOutput, error) {
	return h.usecase.ConflictResolve(ctx, conflictID, strategy)
}

func (h CLIHandler) SyncNow(ctx context.Context) (dto.ReconcileOutput, error) {
	return h.usecase.SyncNow(ctx)
}

func (h CLIHandler) SyncHealth(ctx context.Context) (dto.SyncHealthOutput, error) {
	return h.usecase.SyncHealth(ctx)
}

func (h CLIHandler) SnapshotExport(ctx context.Context) (dto.ExportStateOutput, error) {
	return h.usecase.SnapshotExport(ctx)
}

func (h CLIHandler) Metrics(ctx context.Context) (dto.MetricsOutput, error) {
	return h.usecase.Metrics(ctx)
}
