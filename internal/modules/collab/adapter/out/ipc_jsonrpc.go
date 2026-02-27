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

type WorkspaceInitReq struct {
	Name string
}

type WorkspaceRotateKeyReq struct {
	GraceSeconds int64
}

type WorkspaceShowResp struct {
	Workspace domain.Workspace
	NodeID    string
	Peers     []domain.Peer
}

type PeerAddReq struct {
	Addr  string
	Label string
}

type PeerReq struct {
	PeerID string
}

type StatusResp struct {
	Status collabout.DaemonStatus
}

type ActivityTailReq struct {
	SinceUnixMilli int64
	Limit          int
}

type ActivityTailResp struct {
	Events []domain.ActivityEvent
}

type ConflictsListReq struct {
	EntityKey string
}

type ConflictsListResp struct {
	Records []domain.ConflictRecord
}

type ConflictResolveReq struct {
	ConflictID string
	Strategy   string
}

type ConflictResolveResp struct {
	Record domain.ConflictRecord
}

type SyncResp struct {
	Applied int
}

type SnapshotResp struct {
	Payload string
}

type MetricsResp struct {
	Metrics collabout.MetricsSnapshot
}

type Empty struct{}

func (s *rpcHandler) WorkspaceInit(req WorkspaceInitReq, resp *domain.Workspace) error {
	workspace, err := s.h.WorkspaceInit(context.Background(), req.Name)
	if err != nil {
		return toRPCError(err)
	}
	*resp = workspace
	return nil
}

func (s *rpcHandler) WorkspaceShow(_ Empty, resp *WorkspaceShowResp) error {
	workspace, nodeID, peers, err := s.h.WorkspaceShow(context.Background())
	if err != nil {
		return toRPCError(err)
	}
	resp.Workspace = workspace
	resp.NodeID = nodeID
	resp.Peers = peers
	return nil
}

func (s *rpcHandler) WorkspaceRotateKey(req WorkspaceRotateKeyReq, resp *domain.Workspace) error {
	workspace, err := s.h.WorkspaceRotateKey(context.Background(), time.Duration(req.GraceSeconds)*time.Second)
	if err != nil {
		return toRPCError(err)
	}
	*resp = workspace
	return nil
}

func (s *rpcHandler) PeerAdd(req PeerAddReq, resp *domain.Peer) error {
	peer, err := s.h.PeerAdd(context.Background(), req.Addr, req.Label)
	if err != nil {
		return toRPCError(err)
	}
	*resp = peer
	return nil
}

func (s *rpcHandler) PeerApprove(req PeerReq, resp *domain.Peer) error {
	peer, err := s.h.PeerApprove(context.Background(), req.PeerID)
	if err != nil {
		return toRPCError(err)
	}
	*resp = peer
	return nil
}

func (s *rpcHandler) PeerRevoke(req PeerReq, resp *domain.Peer) error {
	peer, err := s.h.PeerRevoke(context.Background(), req.PeerID)
	if err != nil {
		return toRPCError(err)
	}
	*resp = peer
	return nil
}

func (s *rpcHandler) PeerRemove(req PeerReq, _ *Empty) error {
	return toRPCError(s.h.PeerRemove(context.Background(), req.PeerID))
}

func (s *rpcHandler) PeerList(_ Empty, resp *[]domain.Peer) error {
	peers, err := s.h.PeerList(context.Background())
	if err != nil {
		return toRPCError(err)
	}
	*resp = peers
	return nil
}

func (s *rpcHandler) Status(_ Empty, resp *StatusResp) error {
	status, err := s.h.Status(context.Background())
	if err != nil {
		return toRPCError(err)
	}
	resp.Status = status
	return nil
}

func (s *rpcHandler) ActivityTail(req ActivityTailReq, resp *ActivityTailResp) error {
	query := collabout.ActivityQuery{Limit: req.Limit}
	if req.SinceUnixMilli > 0 {
		query.Since = time.UnixMilli(req.SinceUnixMilli).UTC()
	}
	events, err := s.h.ActivityTail(context.Background(), query)
	if err != nil {
		return toRPCError(err)
	}
	resp.Events = events
	return nil
}

func (s *rpcHandler) ConflictsList(req ConflictsListReq, resp *ConflictsListResp) error {
	records, err := s.h.ConflictsList(context.Background(), req.EntityKey)
	if err != nil {
		return toRPCError(err)
	}
	resp.Records = records
	return nil
}

func (s *rpcHandler) ConflictResolve(req ConflictResolveReq, resp *ConflictResolveResp) error {
	record, err := s.h.ConflictResolve(context.Background(), req.ConflictID, domain.ConflictStrategy(req.Strategy))
	if err != nil {
		return toRPCError(err)
	}
	resp.Record = record
	return nil
}

func (s *rpcHandler) SyncNow(_ Empty, resp *SyncResp) error {
	applied, err := s.h.SyncNow(context.Background())
	if err != nil {
		return toRPCError(err)
	}
	resp.Applied = applied
	return nil
}

func (s *rpcHandler) SnapshotExport(_ Empty, resp *SnapshotResp) error {
	payload, err := s.h.SnapshotExport(context.Background())
	if err != nil {
		return toRPCError(err)
	}
	resp.Payload = payload
	return nil
}

func (s *rpcHandler) Metrics(_ Empty, resp *MetricsResp) error {
	metrics, err := s.h.Metrics(context.Background())
	if err != nil {
		return toRPCError(err)
	}
	resp.Metrics = metrics
	return nil
}

func (s *rpcHandler) Stop(_ Empty, _ *Empty) error {
	return toRPCError(s.h.Stop(context.Background()))
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
	if err := rpcSrv.RegisterName("CollabV2", &rpcHandler{h: handler}); err != nil {
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
	if err := client.Call("CollabV2.WorkspaceInit", WorkspaceInitReq{Name: name}, &resp); err != nil {
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
	resp := WorkspaceShowResp{}
	if err := client.Call("CollabV2.WorkspaceShow", Empty{}, &resp); err != nil {
		return domain.Workspace{}, "", nil, err
	}
	return resp.Workspace, resp.NodeID, resp.Peers, nil
}

func (c *JSONRPCClient) WorkspaceRotateKey(ctx context.Context, socketPath string, gracePeriod time.Duration) (domain.Workspace, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return domain.Workspace{}, err
	}
	defer client.Close()
	resp := domain.Workspace{}
	req := WorkspaceRotateKeyReq{GraceSeconds: int64(gracePeriod.Seconds())}
	if err := client.Call("CollabV2.WorkspaceRotateKey", req, &resp); err != nil {
		return domain.Workspace{}, err
	}
	return resp, nil
}

func (c *JSONRPCClient) PeerAdd(ctx context.Context, socketPath string, addr, label string) (domain.Peer, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return domain.Peer{}, err
	}
	defer client.Close()
	resp := domain.Peer{}
	if err := client.Call("CollabV2.PeerAdd", PeerAddReq{Addr: addr, Label: label}, &resp); err != nil {
		return domain.Peer{}, err
	}
	return resp, nil
}

func (c *JSONRPCClient) PeerApprove(ctx context.Context, socketPath string, peerID string) (domain.Peer, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return domain.Peer{}, err
	}
	defer client.Close()
	resp := domain.Peer{}
	if err := client.Call("CollabV2.PeerApprove", PeerReq{PeerID: peerID}, &resp); err != nil {
		return domain.Peer{}, err
	}
	return resp, nil
}

func (c *JSONRPCClient) PeerRevoke(ctx context.Context, socketPath string, peerID string) (domain.Peer, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return domain.Peer{}, err
	}
	defer client.Close()
	resp := domain.Peer{}
	if err := client.Call("CollabV2.PeerRevoke", PeerReq{PeerID: peerID}, &resp); err != nil {
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
	return client.Call("CollabV2.PeerRemove", PeerReq{PeerID: peerID}, &Empty{})
}

func (c *JSONRPCClient) PeerList(ctx context.Context, socketPath string) ([]domain.Peer, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	resp := []domain.Peer{}
	if err := client.Call("CollabV2.PeerList", Empty{}, &resp); err != nil {
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
	resp := StatusResp{}
	if err := client.Call("CollabV2.Status", Empty{}, &resp); err != nil {
		return collabout.DaemonStatus{}, err
	}
	return resp.Status, nil
}

func (c *JSONRPCClient) ActivityTail(ctx context.Context, socketPath string, query collabout.ActivityQuery) ([]domain.ActivityEvent, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	req := ActivityTailReq{Limit: query.Limit}
	if !query.Since.IsZero() {
		req.SinceUnixMilli = query.Since.UTC().UnixMilli()
	}
	resp := ActivityTailResp{}
	if err := client.Call("CollabV2.ActivityTail", req, &resp); err != nil {
		return nil, err
	}
	return resp.Events, nil
}

func (c *JSONRPCClient) ConflictsList(ctx context.Context, socketPath string, entityKey string) ([]domain.ConflictRecord, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return nil, err
	}
	defer client.Close()
	resp := ConflictsListResp{}
	if err := client.Call("CollabV2.ConflictsList", ConflictsListReq{EntityKey: entityKey}, &resp); err != nil {
		return nil, err
	}
	return resp.Records, nil
}

func (c *JSONRPCClient) ConflictResolve(ctx context.Context, socketPath, conflictID string, strategy domain.ConflictStrategy) (domain.ConflictRecord, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return domain.ConflictRecord{}, err
	}
	defer client.Close()
	resp := ConflictResolveResp{}
	req := ConflictResolveReq{ConflictID: conflictID, Strategy: string(strategy)}
	if err := client.Call("CollabV2.ConflictResolve", req, &resp); err != nil {
		return domain.ConflictRecord{}, err
	}
	return resp.Record, nil
}

func (c *JSONRPCClient) SyncNow(ctx context.Context, socketPath string) (int, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return 0, err
	}
	defer client.Close()
	resp := SyncResp{}
	if err := client.Call("CollabV2.SyncNow", Empty{}, &resp); err != nil {
		return 0, err
	}
	return resp.Applied, nil
}

func (c *JSONRPCClient) SnapshotExport(ctx context.Context, socketPath string) (string, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return "", err
	}
	defer client.Close()
	resp := SnapshotResp{}
	if err := client.Call("CollabV2.SnapshotExport", Empty{}, &resp); err != nil {
		return "", err
	}
	return resp.Payload, nil
}

func (c *JSONRPCClient) Metrics(ctx context.Context, socketPath string) (collabout.MetricsSnapshot, error) {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return collabout.MetricsSnapshot{}, err
	}
	defer client.Close()
	resp := MetricsResp{}
	if err := client.Call("CollabV2.Metrics", Empty{}, &resp); err != nil {
		return collabout.MetricsSnapshot{}, err
	}
	return resp.Metrics, nil
}

func (c *JSONRPCClient) Stop(ctx context.Context, socketPath string) error {
	client, err := dialClient(ctx, socketPath)
	if err != nil {
		return err
	}
	defer client.Close()
	return client.Call("CollabV2.Stop", Empty{}, &Empty{})
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

func toRPCError(err error) error {
	if err == nil {
		return nil
	}
	code := domain.ErrorCodeFromError(err)
	if code == "" {
		return err
	}
	return fmt.Errorf("%s: %s", code, err.Error())
}
