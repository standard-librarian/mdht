package usecase

import (
	"context"

	"mdht/internal/modules/collab/dto"
	collabin "mdht/internal/modules/collab/port/in"
	"mdht/internal/modules/collab/service"
)

type Interactor struct {
	svc *service.CollabService
}

func NewInteractor(svc *service.CollabService) collabin.Usecase {
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
		Status: dto.StatusOutput{
			Online:      status.Status.Online,
			PeerCount:   status.Status.PeerCount,
			PendingOps:  status.Status.PendingOps,
			LastSyncAt:  status.Status.LastSyncAt,
			NodeID:      status.Status.NodeID,
			WorkspaceID: status.Status.WorkspaceID,
			ListenAddrs: status.Status.ListenAddrs,
		},
	}, nil
}

func (i *Interactor) WorkspaceInit(ctx context.Context, name string) (dto.WorkspaceOutput, error) {
	workspace, err := i.svc.WorkspaceInit(ctx, name)
	if err != nil {
		return dto.WorkspaceOutput{}, err
	}
	return dto.WorkspaceOutput{ID: workspace.ID, Name: workspace.Name, CreatedAt: workspace.CreatedAt}, nil
}

func (i *Interactor) WorkspaceShow(ctx context.Context) (dto.WorkspaceShowOutput, error) {
	workspace, nodeID, peers, err := i.svc.WorkspaceShow(ctx)
	if err != nil {
		return dto.WorkspaceShowOutput{}, err
	}
	return dto.WorkspaceShowOutput{
		Workspace: dto.WorkspaceOutput{ID: workspace.ID, Name: workspace.Name, CreatedAt: workspace.CreatedAt},
		NodeID:    nodeID,
		Peers:     len(peers),
	}, nil
}

func (i *Interactor) PeerAdd(ctx context.Context, addr string) (dto.PeerOutput, error) {
	peer, err := i.svc.PeerAdd(ctx, addr)
	if err != nil {
		return dto.PeerOutput{}, err
	}
	return dto.PeerOutput{PeerID: peer.PeerID, Address: peer.Address, AddedAt: peer.AddedAt, LastError: peer.LastError}, nil
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
		out = append(out, dto.PeerOutput{PeerID: item.PeerID, Address: item.Address, AddedAt: item.AddedAt, LastError: item.LastError})
	}
	return out, nil
}

func (i *Interactor) Status(ctx context.Context) (dto.StatusOutput, error) {
	status, err := i.svc.Status(ctx)
	if err != nil {
		return dto.StatusOutput{}, err
	}
	return dto.StatusOutput{
		Online:      status.Online,
		PeerCount:   status.PeerCount,
		PendingOps:  status.PendingOps,
		LastSyncAt:  status.LastSyncAt,
		NodeID:      status.NodeID,
		WorkspaceID: status.WorkspaceID,
		ListenAddrs: status.ListenAddrs,
	}, nil
}

func (i *Interactor) ReconcileNow(ctx context.Context) (dto.ReconcileOutput, error) {
	applied, err := i.svc.ReconcileNow(ctx)
	if err != nil {
		return dto.ReconcileOutput{}, err
	}
	return dto.ReconcileOutput{Applied: applied}, nil
}

func (i *Interactor) ExportState(ctx context.Context) (dto.ExportStateOutput, error) {
	payload, err := i.svc.ExportState(ctx)
	if err != nil {
		return dto.ExportStateOutput{}, err
	}
	return dto.ExportStateOutput{Payload: payload}, nil
}
