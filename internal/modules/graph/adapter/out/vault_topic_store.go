package out

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	graphout "mdht/internal/modules/graph/port/out"
)

type VaultTopicStore struct {
	vaultPath string
}

func NewVaultTopicStore(vaultPath string) graphout.TopicStore {
	return &VaultTopicStore{vaultPath: vaultPath}
}

func (s *VaultTopicStore) AppendSourceLink(_ context.Context, topic, sourceTitle, sourceID string) error {
	path := filepath.Join(s.vaultPath, "topics", topic+".md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create topics dir: %w", err)
	}
	line := fmt.Sprintf("- [[%s]] (%s)", sourceTitle, sourceID)
	content := "# " + topic + "\n\n## Sources\n"
	if b, err := os.ReadFile(path); err == nil {
		content = string(b)
	}
	if strings.Contains(content, line) {
		return nil
	}
	if !strings.Contains(content, "## Sources") {
		content += "\n## Sources\n"
	}
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	content += line + "\n"
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return fmt.Errorf("write topic note: %w", err)
	}
	return nil
}
