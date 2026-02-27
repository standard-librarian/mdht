package usecase

import (
	"context"
	"time"

	"mdht/internal/modules/collab/domain"
	"mdht/internal/modules/collab/dto"
	collabin "mdht/internal/modules/collab/port/in"
	collabout "mdht/internal/modules/collab/port/out"
)

type servicePort interface {
	RunDaemon(ctx context.Context) error
	StartDaemon(ctx context.Context) error
	StopDaemon(ctx context.Context) error
	DaemonStatus(ctx context.Context) (collabout.DaemonRuntimeStatus, error)
	WorkspaceInit(ctx context.Context, name string) (domain.Workspace, error)
	WorkspaceShow(ctx context.Context) (domain.Workspace, string, []domain.Peer, error)
	WorkspaceRotateKey(ctx context.Context, gracePeriod time.Duration) (domain.Workspace, error)
	PeerAdd(ctx context.Context, addr, label string) (domain.Peer, error)
	PeerApprove(ctx context.Context, peerID string) (domain.Peer, error)
	PeerRevoke(ctx context.Context, peerID string) (domain.Peer, error)
	PeerRemove(ctx context.Context, peerID string) error
	PeerList(ctx context.Context) ([]domain.Peer, error)
	Status(ctx context.Context) (collabout.DaemonStatus, error)
	DaemonLogs(ctx context.Context, tail int) (string, error)
	ActivityTail(ctx context.Context, query collabout.ActivityQuery) ([]domain.ActivityEvent, error)
	ConflictsList(ctx context.Context, entityKey string) ([]domain.ConflictRecord, error)
	ConflictResolve(ctx context.Context, conflictID string, strategy domain.ConflictStrategy) (domain.ConflictRecord, error)
	SyncNow(ctx context.Context) (int, error)
	SnapshotExport(ctx context.Context) (string, error)
	Metrics(ctx context.Context) (collabout.MetricsSnapshot, error)
}

type Interactor struct {
	svc servicePort
}

func NewInteractor(svc servicePort) collabin.Usecase {
	return &Interactor{svc: svc}
}

func (i *Interactor) RunDaemon(ctx context.Context) error {
	return i.svc.RunDaemon(ctx)
}

func (i *Interactor) StartDaemon(ctx context.Context) error {
	return i.svc.StartDaemon(ctx)
}

func (i *Interactor) StopDaemon(ctx context.Context) error {
	return i.svc.StopDaemon(ctx)
}

func (i *Interactor) DaemonStatus(ctx context.Context) (dto.DaemonStatusOutput, error) {
	status, err := i.svc.DaemonStatus(ctx)
	if err != nil {
		return dto.DaemonStatusOutput{}, err
	}
	return dto.DaemonStatusOutput{
		Running:    status.Running,
		PID:        status.PID,
		SocketPath: status.SocketPath,
		Status:     mapStatus(status.Status),
	}, nil
}

func (i *Interactor) WorkspaceInit(ctx context.Context, name string) (dto.WorkspaceOutput, error) {
	workspace, err := i.svc.WorkspaceInit(ctx, name)
	if err != nil {
		return dto.WorkspaceOutput{}, err
	}
	return mapWorkspace(workspace), nil
}

func (i *Interactor) WorkspaceShow(ctx context.Context) (dto.WorkspaceShowOutput, error) {
	workspace, nodeID, peers, err := i.svc.WorkspaceShow(ctx)
	if err != nil {
		return dto.WorkspaceShowOutput{}, err
	}
	return dto.WorkspaceShowOutput{
		Workspace: mapWorkspace(workspace),
		NodeID:    nodeID,
		Peers:     len(peers),
	}, nil
}

func (i *Interactor) WorkspaceRotateKey(ctx context.Context, gracePeriod time.Duration) (dto.WorkspaceOutput, error) {
	workspace, err := i.svc.WorkspaceRotateKey(ctx, gracePeriod)
	if err != nil {
		return dto.WorkspaceOutput{}, err
	}
	return mapWorkspace(workspace), nil
}

func (i *Interactor) PeerAdd(ctx context.Context, addr, label string) (dto.PeerOutput, error) {
	peer, err := i.svc.PeerAdd(ctx, addr, label)
	if err != nil {
		return dto.PeerOutput{}, err
	}
	return mapPeer(peer), nil
}

func (i *Interactor) PeerApprove(ctx context.Context, peerID string) (dto.PeerOutput, error) {
	peer, err := i.svc.PeerApprove(ctx, peerID)
	if err != nil {
		return dto.PeerOutput{}, err
	}
	return mapPeer(peer), nil
}

func (i *Interactor) PeerRevoke(ctx context.Context, peerID string) (dto.PeerOutput, error) {
	peer, err := i.svc.PeerRevoke(ctx, peerID)
	if err != nil {
		return dto.PeerOutput{}, err
	}
	return mapPeer(peer), nil
}

func (i *Interactor) PeerRemove(ctx context.Context, peerID string) error {
	return i.svc.PeerRemove(ctx, peerID)
}

func (i *Interactor) PeerList(ctx context.Context) ([]dto.PeerOutput, error) {
	peers, err := i.svc.PeerList(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.PeerOutput, 0, len(peers))
	for _, item := range peers {
		out = append(out, mapPeer(item))
	}
	return out, nil
}

func (i *Interactor) Status(ctx context.Context) (dto.StatusOutput, error) {
	status, err := i.svc.Status(ctx)
	if err != nil {
		return dto.StatusOutput{}, err
	}
	return mapStatus(status), nil
}

func (i *Interactor) DaemonLogs(ctx context.Context, tail int) (string, error) {
	return i.svc.DaemonLogs(ctx, tail)
}

func (i *Interactor) ActivityTail(ctx context.Context, since time.Time, limit int) ([]dto.ActivityOutput, error) {
	events, err := i.svc.ActivityTail(ctx, collabout.ActivityQuery{Since: since, Limit: limit})
	if err != nil {
		return nil, err
	}
	out := make([]dto.ActivityOutput, 0, len(events))
	for _, event := range events {
		out = append(out, dto.ActivityOutput{
			ID:         event.ID,
			OccurredAt: event.OccurredAt,
			Type:       string(event.Type),
			Message:    event.Message,
			Fields:     event.Fields,
		})
	}
	return out, nil
}

func (i *Interactor) ConflictsList(ctx context.Context, entityKey string) ([]dto.ConflictOutput, error) {
	conflicts, err := i.svc.ConflictsList(ctx, entityKey)
	if err != nil {
		return nil, err
	}
	out := make([]dto.ConflictOutput, 0, len(conflicts))
	for _, conflict := range conflicts {
		out = append(out, mapConflict(conflict))
	}
	return out, nil
}

func (i *Interactor) ConflictResolve(ctx context.Context, conflictID, strategy string) (dto.ConflictOutput, error) {
	record, err := i.svc.ConflictResolve(ctx, conflictID, domain.ConflictStrategy(strategy))
	if err != nil {
		return dto.ConflictOutput{}, err
	}
	return mapConflict(record), nil
}

func (i *Interactor) SyncNow(ctx context.Context) (dto.ReconcileOutput, error) {
	applied, err := i.svc.SyncNow(ctx)
	if err != nil {
		return dto.ReconcileOutput{}, err
	}
	return dto.ReconcileOutput{Applied: applied}, nil
}

func (i *Interactor) SnapshotExport(ctx context.Context) (dto.ExportStateOutput, error) {
	payload, err := i.svc.SnapshotExport(ctx)
	if err != nil {
		return dto.ExportStateOutput{}, err
	}
	return dto.ExportStateOutput{Payload: payload}, nil
}

func (i *Interactor) Metrics(ctx context.Context) (dto.MetricsOutput, error) {
	metrics, err := i.svc.Metrics(ctx)
	if err != nil {
		return dto.MetricsOutput{}, err
	}
	return dto.MetricsOutput{
		WorkspaceID:      metrics.WorkspaceID,
		NodeID:           metrics.NodeID,
		ApprovedPeers:    metrics.ApprovedPeers,
		PendingConflicts: metrics.PendingConflicts,
		PendingOps:       metrics.PendingOps,
		LastSyncAt:       metrics.LastSyncAt,
		CollectedAt:      metrics.CollectedAt,
		Counters: dto.ValidationCountersOutput{
			InvalidAuthTag:      metrics.Counters.InvalidAuthTag,
			WorkspaceMismatch:   metrics.Counters.WorkspaceMismatch,
			UnauthenticatedPeer: metrics.Counters.UnauthenticatedPeer,
			DecodeErrors:        metrics.Counters.DecodeErrors,
			BroadcastSendErrors: metrics.Counters.BroadcastSendErrors,
			ReconcileSendErrors: metrics.Counters.ReconcileSendErrors,
			ReconnectAttempts:   metrics.Counters.ReconnectAttempts,
			ReconnectSuccesses:  metrics.Counters.ReconnectSuccesses,
		},
	}, nil
}

func mapWorkspace(workspace domain.Workspace) dto.WorkspaceOutput {
	return dto.WorkspaceOutput{
		ID:            workspace.ID,
		Name:          workspace.Name,
		CreatedAt:     workspace.CreatedAt,
		SchemaVersion: workspace.SchemaVersion,
	}
}

func mapPeer(peer domain.Peer) dto.PeerOutput {
	return dto.PeerOutput{
		PeerID:    peer.PeerID,
		Address:   peer.Address,
		Label:     peer.Label,
		State:     string(peer.State),
		FirstSeen: peer.FirstSeen,
		LastSeen:  peer.LastSeen,
		AddedAt:   peer.AddedAt,
		LastError: peer.LastError,
	}
}

func mapStatus(status collabout.DaemonStatus) dto.StatusOutput {
	return dto.StatusOutput{
		Online:            status.Online,
		PeerCount:         status.PeerCount,
		ApprovedPeerCount: status.ApprovedPeerCount,
		PendingConflicts:  status.PendingConflicts,
		PendingOps:        status.PendingOps,
		LastSyncAt:        status.LastSyncAt,
		NodeID:            status.NodeID,
		WorkspaceID:       status.WorkspaceID,
		ListenAddrs:       status.ListenAddrs,
		MetricsAddress:    status.MetricsAddress,
		Counters: dto.ValidationCountersOutput{
			InvalidAuthTag:      status.Counters.InvalidAuthTag,
			WorkspaceMismatch:   status.Counters.WorkspaceMismatch,
			UnauthenticatedPeer: status.Counters.UnauthenticatedPeer,
			DecodeErrors:        status.Counters.DecodeErrors,
			BroadcastSendErrors: status.Counters.BroadcastSendErrors,
			ReconcileSendErrors: status.Counters.ReconcileSendErrors,
			ReconnectAttempts:   status.Counters.ReconnectAttempts,
			ReconnectSuccesses:  status.Counters.ReconnectSuccesses,
		},
	}
}

func mapConflict(conflict domain.ConflictRecord) dto.ConflictOutput {
	return dto.ConflictOutput{
		ID:          conflict.ID,
		EntityKey:   conflict.EntityKey,
		Field:       conflict.Field,
		LocalValue:  conflict.LocalValue,
		RemoteValue: conflict.RemoteValue,
		Status:      string(conflict.Status),
		Strategy:    conflict.Strategy,
		MergedValue: conflict.MergedValue,
		CreatedAt:   conflict.CreatedAt,
		ResolvedAt:  conflict.ResolvedAt,
		ResolvedBy:  conflict.ResolvedBy,
	}
}
