package out

import (
	"context"
	"fmt"
	"net"
	"net/rpc"
	"net/rpc/jsonrpc"
	"os"
	"path/filepath"
	"time"

	"mdht/internal/modules/collab/domain"
	collabout "mdht/internal/modules/collab/port/out"
)

type JSONRPCServer struct{}

type JSONRPCClient struct{}

func NewJSONRPCServer() collabout.IPCServer {
	return &JSONRPCServer{}
}

func NewJSONRPCClient() collabout.IPCClient {
	return &JSONRPCClient{}
}

type rpcHandler struct {
	h collabout.IPCHandler
}

type workspaceInitReq struct {
	Name string
}

type workspaceShowResp struct {
	Workspace domain.Workspace
	NodeID    string
	Peers     []domain.Peer
}

type peerAddrReq struct {
	Addr string
}

type peerRemoveReq struct {
	PeerID string
}

type statusResp struct {
	Status collabout.DaemonStatus
}

type reconcileResp struct {
	Applied int
}

type exportStateResp struct {
	Payload string
}

type empty struct{}

func (s *rpcHandler) WorkspaceInit(req workspaceInitReq, resp *domain.Workspace) error {
	workspace, err := s.h.WorkspaceInit(context.Background(), req.Name)
	if err != nil {
		return err
	}
	*resp = workspace
	return nil
}

func (s *rpcHandler) WorkspaceShow(_ empty, resp *workspaceShowResp) error {
	workspace, nodeID, peers, err := s.h.WorkspaceShow(context.Background())
	if err != nil {
		return err
	}
	resp.Workspace = workspace
	resp.NodeID = nodeID
	resp.Peers = peers
	return nil
}

func (s *rpcHandler) PeerAdd(req peerAddrReq, resp *domain.Peer) error {
	peer, err := s.h.PeerAdd(context.Background(), req.Addr)
	if err != nil {
		return err
	}
	*resp = peer
	return nil
}

func (s *rpcHandler) PeerRemove(req peerRemoveReq, _ *empty) error {
	return s.h.PeerRemove(context.Background(), req.PeerID)
}

func (s *rpcHandler) PeerList(_ empty, resp *[]domain.Peer) error {
	peers, err := s.h.PeerList(context.Background())
	if err != nil {
		return err
	}
	*resp = peers
	return nil
}

func (s *rpcHandler) Status(_ empty, resp *statusResp) error {
	status, err := s.h.Status(context.Background())
	if err != nil {
		return err
	}
	resp.Status = status
	return nil
}

func (s *rpcHandler) ReconcileNow(_ empty, resp *reconcileResp) error {
	applied, err := s.h.ReconcileNow(context.Background())
	if err != nil {
		return err
	}
	resp.Applied = applied
	return nil
}

func (s *rpcHandler) ExportState(_ empty, resp *exportStateResp) error {
	payload, err := s.h.ExportState(context.Background())
	if err != nil {
		return err
	}
	resp.Payload = payload
	return nil
}

func (s *rpcHandler) Stop(_ empty, _ *empty) error {
	return s.h.Stop(context.Background())
}

func (s *JSONRPCServer) Serve(ctx context.Context, socketPath string, handler collabout.IPCHandler) error {
	if err := os.MkdirAll(filepath.Dir(socketPath), 0o755); err != nil {
		return fmt.Errorf("create ipc dir: %w", err)
	}
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove stale ipc socket: %w", err)
	}
	ln, err := net.Listen("unix", socketPath)
	if err != nil {
		return fmt.Errorf("listen ipc socket: %w", err)
	}
	if err := os.Chmod(socketPath, 0o600); err != nil {
		_ = ln.Close()
		return fmt.Errorf("chmod ipc socket: %w", err)
	}
	defer ln.Close()

	rpcSrv := rpc.NewServer()
	if err := rpcSrv.RegisterName("Collab", &rpcHandler{h: handler}); err != nil {
		return fmt.Errorf("register ipc handler: %w", err)
	}

	stop := make(chan struct{})
	go func() {
		select {
		case <-ctx.Done():
			_ = ln.Close()
		case <-stop:
		}
	}()
	defer close(stop)

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if ne, ok := err.(net.Error); ok && ne.Timeout() {
				continue
			}
			return err
		}
		go rpcSrv.ServeCodec(jsonrpc.NewServerCodec(conn))
	}
}

func (c *JSONRPCClient) WorkspaceInit(ctx context.Context, socketPath string, name string) (domain.Workspace, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return domain.Workspace{}, err
	}
	defer client.Close()
	resp := domain.Workspace{}
	if err := client.Call("Collab.WorkspaceInit", workspaceInitReq{Name: name}, &resp); err != nil {
		return domain.Workspace{}, err
	}
	return resp, nil
}

func (c *JSONRPCClient) WorkspaceShow(ctx context.Context, socketPath string) (domain.Workspace, string, []domain.Peer, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return domain.Workspace{}, "", nil, err
	}
	defer client.Close()
	resp := workspaceShowResp{}
	if err := client.Call("Collab.WorkspaceShow", empty{}, &resp); err != nil {
		return domain.Workspace{}, "", nil, err
	}
	return resp.Workspace, resp.NodeID, resp.Peers, nil
}

func (c *JSONRPCClient) PeerAdd(ctx context.Context, socketPath string, addr string) (domain.Peer, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return domain.Peer{}, err
	}
	defer client.Close()
	resp := domain.Peer{}
	if err := client.Call("Collab.PeerAdd", peerAddrReq{Addr: addr}, &resp); err != nil {
		return domain.Peer{}, err
	}
	return resp, nil
}

func (c *JSONRPCClient) PeerRemove(ctx context.Context, socketPath string, peerID string) error {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Call("Collab.PeerRemove", peerRemoveReq{PeerID: peerID}, &empty{})
}

func (c *JSONRPCClient) PeerList(ctx context.Context, socketPath string) ([]domain.Peer, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	resp := []domain.Peer{}
	if err := client.Call("Collab.PeerList", empty{}, &resp); err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *JSONRPCClient) Status(ctx context.Context, socketPath string) (collabout.DaemonStatus, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return collabout.DaemonStatus{}, err
	}
	defer client.Close()
	resp := statusResp{}
	if err := client.Call("Collab.Status", empty{}, &resp); err != nil {
		return collabout.DaemonStatus{}, err
	}
	return resp.Status, nil
}

func (c *JSONRPCClient) ReconcileNow(ctx context.Context, socketPath string) (int, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return 0, err
	}
	defer client.Close()
	resp := reconcileResp{}
	if err := client.Call("Collab.ReconcileNow", empty{}, &resp); err != nil {
		return 0, err
	}
	return resp.Applied, nil
}

func (c *JSONRPCClient) ExportState(ctx context.Context, socketPath string) (string, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return "", err
	}
	defer client.Close()
	resp := exportStateResp{}
	if err := client.Call("Collab.ExportState", empty{}, &resp); err != nil {
		return "", err
	}
	return resp.Payload, nil
}

func (c *JSONRPCClient) Stop(ctx context.Context, socketPath string) error {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Call("Collab.Stop", empty{}, &empty{})
}

func dialClient(ctx context.Context, socketPath string) (*rpc.Client, error) {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return nil, err
	}
	_ = conn.SetDeadline(time.Now().Add(10 * time.Second))
	client := rpc.NewClientWithCodec(jsonrpc.NewClientCodec(conn))
	return client, nil
}
