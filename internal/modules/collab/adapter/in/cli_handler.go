package in

import (
	"context"

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

func (h CLIHandler) PeerAdd(ctx context.Context, addr string) (dto.PeerOutput, error) {
	return h.usecase.PeerAdd(ctx, addr)
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

func (h CLIHandler) Doctor(ctx context.Context) (dto.DoctorOutput, error) {
	return h.usecase.Doctor(ctx)
}

func (h CLIHandler) DaemonLogs(ctx context.Context, tail int) (string, error) {
	return h.usecase.DaemonLogs(ctx, tail)
}

func (h CLIHandler) ReconcileNow(ctx context.Context) (dto.ReconcileOutput, error) {
	return h.usecase.ReconcileNow(ctx)
}

func (h CLIHandler) ExportState(ctx context.Context) (string, error) {
	out, err := h.usecase.ExportState(ctx)
	if err != nil {
		return "", err
	}
	return out.Payload, nil
}
