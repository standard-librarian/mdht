package rpc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/hashicorp/go-plugin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
)

const (
	PluginMapKey       = "mdht"
	serviceName        = "mdht.plugin.v1.MdhtPlugin"
	jsonCodecName      = "json"
	methodGetMetadata  = "/" + serviceName + "/GetMetadata"
	methodListCommands = "/" + serviceName + "/ListCommands"
	methodExecute      = "/" + serviceName + "/Execute"
	methodPrepareTTY   = "/" + serviceName + "/PrepareTTY"
)

var HandshakeConfig = plugin.HandshakeConfig{
	ProtocolVersion:  1,
	MagicCookieKey:   "MDHT_PLUGIN",
	MagicCookieValue: "mdht",
}

type jsonCodec struct{}

func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}

func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func (jsonCodec) Name() string {
	return jsonCodecName
}

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

type Empty struct{}

type Metadata struct {
	Name         string   `json:"name"`
	Version      string   `json:"version"`
	Capabilities []string `json:"capabilities"`
}

type CommandDescriptor struct {
	ID              string `json:"id"`
	Title           string `json:"title"`
	Description     string `json:"description"`
	Kind            string `json:"kind"`
	InputSchemaJSON string `json:"input_schema_json"`
	TimeoutMS       int32  `json:"timeout_ms"`
}

type ListCommandsResponse struct {
	Commands []CommandDescriptor `json:"commands"`
}

type ExecuteContext struct {
	VaultPath string            `json:"vault_path"`
	SourceID  string            `json:"source_id"`
	SessionID string            `json:"session_id"`
	Cwd       string            `json:"cwd"`
	Env       map[string]string `json:"env"`
}

type ExecuteRequest struct {
	CommandID string         `json:"command_id"`
	InputJSON string         `json:"input_json"`
	Context   ExecuteContext `json:"context"`
}

type ExecuteResponse struct {
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	OutputJSON string `json:"output_json"`
	ExitCode   int32  `json:"exit_code"`
}

type PrepareTTYRequest struct {
	CommandID string         `json:"command_id"`
	InputJSON string         `json:"input_json"`
	Context   ExecuteContext `json:"context"`
}

type PrepareTTYResponse struct {
	Argv []string          `json:"argv"`
	Cwd  string            `json:"cwd"`
	Env  map[string]string `json:"env"`
}

type MdhtPluginServer interface {
	GetMetadata(ctx context.Context, in *Empty) (*Metadata, error)
	ListCommands(ctx context.Context, in *Empty) (*ListCommandsResponse, error)
	Execute(ctx context.Context, in *ExecuteRequest) (*ExecuteResponse, error)
	PrepareTTY(ctx context.Context, in *PrepareTTYRequest) (*PrepareTTYResponse, error)
}

type MdhtPluginClient interface {
	GetMetadata(ctx context.Context) (*Metadata, error)
	ListCommands(ctx context.Context) (*ListCommandsResponse, error)
	Execute(ctx context.Context, in *ExecuteRequest) (*ExecuteResponse, error)
	PrepareTTY(ctx context.Context, in *PrepareTTYRequest) (*PrepareTTYResponse, error)
}

type mdhtPluginClient struct {
	conn *grpc.ClientConn
}

func NewMdhtPluginClient(conn *grpc.ClientConn) MdhtPluginClient {
	return &mdhtPluginClient{conn: conn}
}

func (c *mdhtPluginClient) GetMetadata(ctx context.Context) (*Metadata, error) {
	out := &Metadata{}
	if err := c.conn.Invoke(ctx, methodGetMetadata, &Empty{}, out, grpc.CallContentSubtype(jsonCodecName)); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mdhtPluginClient) ListCommands(ctx context.Context) (*ListCommandsResponse, error) {
	out := &ListCommandsResponse{}
	if err := c.conn.Invoke(ctx, methodListCommands, &Empty{}, out, grpc.CallContentSubtype(jsonCodecName)); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mdhtPluginClient) Execute(ctx context.Context, in *ExecuteRequest) (*ExecuteResponse, error) {
	out := &ExecuteResponse{}
	if err := c.conn.Invoke(ctx, methodExecute, in, out, grpc.CallContentSubtype(jsonCodecName)); err != nil {
		return nil, err
	}
	return out, nil
}

func (c *mdhtPluginClient) PrepareTTY(ctx context.Context, in *PrepareTTYRequest) (*PrepareTTYResponse, error) {
	out := &PrepareTTYResponse{}
	if err := c.conn.Invoke(ctx, methodPrepareTTY, in, out, grpc.CallContentSubtype(jsonCodecName)); err != nil {
		return nil, err
	}
	return out, nil
}

func RegisterMdhtPluginServer(server grpc.ServiceRegistrar, impl MdhtPluginServer) {
	server.RegisterService(&grpc.ServiceDesc{
		ServiceName: serviceName,
		HandlerType: (*MdhtPluginServer)(nil),
		Methods: []grpc.MethodDesc{
			{
				MethodName: "GetMetadata",
				Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					in := &Empty{}
					if err := dec(in); err != nil {
						return nil, err
					}
					if interceptor == nil {
						return impl.GetMetadata(ctx, in)
					}
					info := &grpc.UnaryServerInfo{Server: srv, FullMethod: methodGetMetadata}
					handler := func(ctx context.Context, req any) (any, error) {
						empty, ok := req.(*Empty)
						if !ok {
							return nil, fmt.Errorf("invalid request type")
						}
						return impl.GetMetadata(ctx, empty)
					}
					return interceptor(ctx, in, info, handler)
				},
			},
			{
				MethodName: "ListCommands",
				Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					in := &Empty{}
					if err := dec(in); err != nil {
						return nil, err
					}
					if interceptor == nil {
						return impl.ListCommands(ctx, in)
					}
					info := &grpc.UnaryServerInfo{Server: srv, FullMethod: methodListCommands}
					handler := func(ctx context.Context, req any) (any, error) {
						empty, ok := req.(*Empty)
						if !ok {
							return nil, fmt.Errorf("invalid request type")
						}
						return impl.ListCommands(ctx, empty)
					}
					return interceptor(ctx, in, info, handler)
				},
			},
			{
				MethodName: "Execute",
				Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					in := &ExecuteRequest{}
					if err := dec(in); err != nil {
						return nil, err
					}
					if interceptor == nil {
						return impl.Execute(ctx, in)
					}
					info := &grpc.UnaryServerInfo{Server: srv, FullMethod: methodExecute}
					handler := func(ctx context.Context, req any) (any, error) {
						inReq, ok := req.(*ExecuteRequest)
						if !ok {
							return nil, fmt.Errorf("invalid request type")
						}
						return impl.Execute(ctx, inReq)
					}
					return interceptor(ctx, in, info, handler)
				},
			},
			{
				MethodName: "PrepareTTY",
				Handler: func(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
					in := &PrepareTTYRequest{}
					if err := dec(in); err != nil {
						return nil, err
					}
					if interceptor == nil {
						return impl.PrepareTTY(ctx, in)
					}
					info := &grpc.UnaryServerInfo{Server: srv, FullMethod: methodPrepareTTY}
					handler := func(ctx context.Context, req any) (any, error) {
						inReq, ok := req.(*PrepareTTYRequest)
						if !ok {
							return nil, fmt.Errorf("invalid request type")
						}
						return impl.PrepareTTY(ctx, inReq)
					}
					return interceptor(ctx, in, info, handler)
				},
			},
		},
		Streams:  []grpc.StreamDesc{},
		Metadata: "schemas/plugin-rpc-v1.proto",
	}, impl)
}

type GRPCPlugin struct {
	plugin.NetRPCUnsupportedPlugin
	Impl MdhtPluginServer
}

func (p *GRPCPlugin) GRPCServer(_ *plugin.GRPCBroker, server *grpc.Server) error {
	RegisterMdhtPluginServer(server, p.Impl)
	return nil
}

func (p *GRPCPlugin) GRPCClient(_ context.Context, _ *plugin.GRPCBroker, conn *grpc.ClientConn) (any, error) {
	return NewMdhtPluginClient(conn), nil
}

func PluginMap(impl MdhtPluginServer) map[string]plugin.Plugin {
	return map[string]plugin.Plugin{
		PluginMapKey: &GRPCPlugin{Impl: impl},
	}
}
