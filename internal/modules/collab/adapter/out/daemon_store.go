package out

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	collabout "mdht/internal/modules/collab/port/out"
)

type FileDaemonStore struct {
	pidPath    string
	socketPath string
	logPath    string
}

func NewFileDaemonStore(vaultPath string) collabout.DaemonStore {
	base := filepath.Join(vaultPath, ".mdht", "collab")
	return &FileDaemonStore{
		pidPath:    filepath.Join(base, "daemon.pid"),
		socketPath: filepath.Join(base, "daemon.sock"),
		logPath:    filepath.Join(base, "daemon.log"),
	}
}

func (s *FileDaemonStore) WritePID(_ context.Context, pid int) error {
	if err := os.MkdirAll(filepath.Dir(s.pidPath), 0o755); err != nil {
		return fmt.Errorf("create daemon dir: %w", err)
	}
	return os.WriteFile(s.pidPath, []byte(strconv.Itoa(pid)), 0o644)
}

func (s *FileDaemonStore) ReadPID(_ context.Context) (int, error) {
	raw, err := os.ReadFile(s.pidPath)
	if err != nil {
		return 0, err
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(raw)))
	if err != nil {
		return 0, fmt.Errorf("decode daemon pid: %w", err)
	}
	return pid, nil
}

func (s *FileDaemonStore) ClearPID(_ context.Context) error {
	if err := os.Remove(s.pidPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("remove daemon pid: %w", err)
	}
	return nil
}

func (s *FileDaemonStore) SocketPath() string {
	return s.socketPath
}

func (s *FileDaemonStore) LogPath() string {
	return s.logPath
}
