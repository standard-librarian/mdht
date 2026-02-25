package out

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"mdht/internal/modules/collab/domain"
	collabout "mdht/internal/modules/collab/port/out"
)

type FileWorkspaceStore struct {
	workspacePath string
	workspaceKey  string
	nodeKeyPath   string
}

func NewFileWorkspaceStore(vaultPath string) collabout.WorkspaceStore {
	base := filepath.Join(vaultPath, ".mdht", "collab")
	return &FileWorkspaceStore{
		workspacePath: filepath.Join(base, "workspace.json"),
		workspaceKey:  filepath.Join(base, "workspace.key"),
		nodeKeyPath:   filepath.Join(base, "node.ed25519"),
	}
}

func (s *FileWorkspaceStore) Init(ctx context.Context, name string) (domain.Workspace, []byte, domain.NodeIdentity, error) {
	if existing, key, node, err := s.Load(ctx); err == nil {
		return existing, key, node, nil
	}
	if strings.TrimSpace(name) == "" {
		return domain.Workspace{}, nil, domain.NodeIdentity{}, domain.ErrInvalidWorkspaceName
	}
	if err := os.MkdirAll(filepath.Dir(s.workspacePath), 0o755); err != nil {
		return domain.Workspace{}, nil, domain.NodeIdentity{}, fmt.Errorf("create collab dir: %w", err)
	}

	workspaceID, err := randomHex(16)
	if err != nil {
		return domain.Workspace{}, nil, domain.NodeIdentity{}, err
	}
	workspace := domain.Workspace{ID: workspaceID, Name: name, CreatedAt: time.Now().UTC()}
	if err := workspace.Validate(); err != nil {
		return domain.Workspace{}, nil, domain.NodeIdentity{}, err
	}

	workspacePayload, err := json.MarshalIndent(workspace, "", "  ")
	if err != nil {
		return domain.Workspace{}, nil, domain.NodeIdentity{}, fmt.Errorf("encode workspace: %w", err)
	}
	if err := os.WriteFile(s.workspacePath, workspacePayload, 0o644); err != nil {
		return domain.Workspace{}, nil, domain.NodeIdentity{}, fmt.Errorf("write workspace: %w", err)
	}

	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return domain.Workspace{}, nil, domain.NodeIdentity{}, fmt.Errorf("generate workspace key: %w", err)
	}
	if err := os.WriteFile(s.workspaceKey, []byte(base64.StdEncoding.EncodeToString(key)), 0o600); err != nil {
		return domain.Workspace{}, nil, domain.NodeIdentity{}, fmt.Errorf("write workspace key: %w", err)
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return domain.Workspace{}, nil, domain.NodeIdentity{}, fmt.Errorf("generate node key: %w", err)
	}
	node := domain.NodeIdentity{NodeID: hex.EncodeToString(pub), PrivateKey: base64.StdEncoding.EncodeToString(priv)}
	if err := os.WriteFile(s.nodeKeyPath, []byte(node.PrivateKey), 0o600); err != nil {
		return domain.Workspace{}, nil, domain.NodeIdentity{}, fmt.Errorf("write node key: %w", err)
	}

	return workspace, key, node, nil
}

func (s *FileWorkspaceStore) Load(_ context.Context) (domain.Workspace, []byte, domain.NodeIdentity, error) {
	workspace := domain.Workspace{}
	rawWorkspace, err := os.ReadFile(s.workspacePath)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.Workspace{}, nil, domain.NodeIdentity{}, domain.ErrWorkspaceNotInitialized
		}
		return domain.Workspace{}, nil, domain.NodeIdentity{}, fmt.Errorf("read workspace: %w", err)
	}
	if err := json.Unmarshal(rawWorkspace, &workspace); err != nil {
		return domain.Workspace{}, nil, domain.NodeIdentity{}, fmt.Errorf("decode workspace: %w", err)
	}

	rawKey, err := os.ReadFile(s.workspaceKey)
	if err != nil {
		return domain.Workspace{}, nil, domain.NodeIdentity{}, fmt.Errorf("read workspace key: %w", err)
	}
	workspaceKey, err := domain.RandomWorkspaceKey(strings.TrimSpace(string(rawKey)))
	if err != nil {
		return domain.Workspace{}, nil, domain.NodeIdentity{}, fmt.Errorf("decode workspace key: %w", err)
	}

	rawNodeKey, err := os.ReadFile(s.nodeKeyPath)
	if err != nil {
		return domain.Workspace{}, nil, domain.NodeIdentity{}, fmt.Errorf("read node key: %w", err)
	}
	decodedPriv, err := base64.StdEncoding.DecodeString(strings.TrimSpace(string(rawNodeKey)))
	if err != nil {
		return domain.Workspace{}, nil, domain.NodeIdentity{}, fmt.Errorf("decode node key: %w", err)
	}
	if len(decodedPriv) != ed25519.PrivateKeySize {
		return domain.Workspace{}, nil, domain.NodeIdentity{}, fmt.Errorf("invalid node key size")
	}
	pub := decodedPriv[32:]
	node := domain.NodeIdentity{NodeID: hex.EncodeToString(pub), PrivateKey: strings.TrimSpace(string(rawNodeKey))}

	return workspace, workspaceKey, node, nil
}

func randomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("random bytes: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
