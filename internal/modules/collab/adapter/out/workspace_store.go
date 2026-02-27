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

type workspaceFile struct {
	WorkspaceID   string    `json:"workspace_id"`
	ID            string    `json:"id,omitempty"`
	Name          string    `json:"name"`
	CreatedAt     time.Time `json:"created_at"`
	SchemaVersion int       `json:"schema_version"`
}

type FileWorkspaceStore struct {
	workspacePath string
	keysPath      string
	nodeKeyPath   string
}

func NewFileWorkspaceStore(vaultPath string) collabout.WorkspaceStore {
	base := filepath.Join(vaultPath, ".mdht", "collab")
	return &FileWorkspaceStore{
		workspacePath: filepath.Join(base, "workspace.json"),
		keysPath:      filepath.Join(base, "keys.json"),
		nodeKeyPath:   filepath.Join(base, "node.ed25519"),
	}
}

func (s *FileWorkspaceStore) Init(ctx context.Context, name string) (domain.Workspace, domain.KeyRing, domain.NodeIdentity, error) {
	if existing, ring, node, err := s.Load(ctx); err == nil {
		return existing, ring, node, nil
	}
	if strings.TrimSpace(name) == "" {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, domain.ErrInvalidWorkspaceName
	}
	if err := os.MkdirAll(filepath.Dir(s.workspacePath), 0o755); err != nil {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, fmt.Errorf("create collab dir: %w", err)
	}

	workspaceID, err := randomHex(16)
	if err != nil {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, err
	}
	workspace := domain.Workspace{
		ID:            workspaceID,
		Name:          name,
		CreatedAt:     time.Now().UTC(),
		SchemaVersion: domain.SchemaVersionV2,
	}
	if err := workspace.Validate(); err != nil {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, err
	}

	keyMaterial := make([]byte, 32)
	if _, err := rand.Read(keyMaterial); err != nil {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, fmt.Errorf("generate workspace key: %w", err)
	}
	keyID, err := randomHex(8)
	if err != nil {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, err
	}
	keyRing := domain.KeyRing{
		ActiveKeyID: keyID,
		Keys: []domain.KeyRecord{
			{
				ID:        keyID,
				KeyBase64: base64.StdEncoding.EncodeToString(keyMaterial),
				CreatedAt: time.Now().UTC(),
			},
		},
	}

	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, fmt.Errorf("generate node key: %w", err)
	}
	node := domain.NodeIdentity{
		NodeID:     hex.EncodeToString(pub),
		PrivateKey: base64.StdEncoding.EncodeToString(priv),
	}

	if err := s.writeWorkspace(workspace); err != nil {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, err
	}
	if err := s.writeKeyRing(keyRing); err != nil {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, err
	}
	if err := os.WriteFile(s.nodeKeyPath, []byte(node.PrivateKey), 0o600); err != nil {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, fmt.Errorf("write node key: %w", err)
	}
	return workspace, keyRing, node, nil
}

func (s *FileWorkspaceStore) Load(_ context.Context) (domain.Workspace, domain.KeyRing, domain.NodeIdentity, error) {
	workspaceRaw, err := os.ReadFile(s.workspacePath)
	if err != nil {
		if os.IsNotExist(err) {
			return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, domain.ErrWorkspaceNotInitialized
		}
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, fmt.Errorf("read workspace: %w", err)
	}
	var wf workspaceFile
	if err := json.Unmarshal(workspaceRaw, &wf); err != nil {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, fmt.Errorf("decode workspace: %w", err)
	}
	id := strings.TrimSpace(wf.WorkspaceID)
	if id == "" {
		id = strings.TrimSpace(wf.ID)
	}
	workspace := domain.Workspace{
		ID:            id,
		Name:          wf.Name,
		CreatedAt:     wf.CreatedAt,
		SchemaVersion: wf.SchemaVersion,
	}
	if workspace.SchemaVersion == 0 {
		workspace.SchemaVersion = domain.SchemaVersionV2
	}
	if err := workspace.Validate(); err != nil {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, err
	}

	keysRaw, err := os.ReadFile(s.keysPath)
	if err != nil {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, fmt.Errorf("read keys: %w", err)
	}
	var keyRing domain.KeyRing
	if err := json.Unmarshal(keysRaw, &keyRing); err != nil {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, fmt.Errorf("decode keys: %w", err)
	}
	if strings.TrimSpace(keyRing.ActiveKeyID) == "" || len(keyRing.Keys) == 0 {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, fmt.Errorf("invalid key ring")
	}

	rawNodeKey, err := os.ReadFile(s.nodeKeyPath)
	if err != nil {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, fmt.Errorf("read node key: %w", err)
	}
	private := strings.TrimSpace(string(rawNodeKey))
	decoded, err := base64.StdEncoding.DecodeString(private)
	if err != nil {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, fmt.Errorf("decode node key: %w", err)
	}
	if len(decoded) != ed25519.PrivateKeySize {
		return domain.Workspace{}, domain.KeyRing{}, domain.NodeIdentity{}, fmt.Errorf("invalid node key size")
	}
	node := domain.NodeIdentity{
		NodeID:     hex.EncodeToString(decoded[32:]),
		PrivateKey: private,
	}
	return workspace, keyRing, node, nil
}

func (s *FileWorkspaceStore) RotateKey(ctx context.Context, gracePeriod time.Duration) (domain.KeyRing, error) {
	_, keyRing, _, err := s.Load(ctx)
	if err != nil {
		return domain.KeyRing{}, err
	}
	keyMaterial := make([]byte, 32)
	if _, err := rand.Read(keyMaterial); err != nil {
		return domain.KeyRing{}, fmt.Errorf("generate workspace key: %w", err)
	}
	keyID, err := randomHex(8)
	if err != nil {
		return domain.KeyRing{}, err
	}
	keyRing.Keys = append(keyRing.Keys, domain.KeyRecord{
		ID:        keyID,
		KeyBase64: base64.StdEncoding.EncodeToString(keyMaterial),
		CreatedAt: time.Now().UTC(),
	})
	keyRing.ActiveKeyID = keyID
	if gracePeriod > 0 {
		keyRing.GraceUntil = time.Now().UTC().Add(gracePeriod)
	} else {
		keyRing.GraceUntil = time.Time{}
	}
	if err := s.writeKeyRing(keyRing); err != nil {
		return domain.KeyRing{}, err
	}
	return keyRing, nil
}

func (s *FileWorkspaceStore) writeWorkspace(workspace domain.Workspace) error {
	payload, err := json.MarshalIndent(workspaceFile{
		WorkspaceID:   workspace.ID,
		Name:          workspace.Name,
		CreatedAt:     workspace.CreatedAt,
		SchemaVersion: workspace.SchemaVersion,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode workspace: %w", err)
	}
	if err := os.WriteFile(s.workspacePath, payload, 0o644); err != nil {
		return fmt.Errorf("write workspace: %w", err)
	}
	return nil
}

func (s *FileWorkspaceStore) writeKeyRing(ring domain.KeyRing) error {
	payload, err := json.MarshalIndent(ring, "", "  ")
	if err != nil {
		return fmt.Errorf("encode keys: %w", err)
	}
	if err := os.WriteFile(s.keysPath, payload, 0o600); err != nil {
		return fmt.Errorf("write keys: %w", err)
	}
	return nil
}

func randomHex(size int) (string, error) {
	buf := make([]byte, size)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("random bytes: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
