package out

import (
	"context"
	"fmt"
	"os/exec"
	"runtime"

	readerout "mdht/internal/modules/reader/port/out"
)

type OSExternalLauncher struct{}

func NewOSExternalLauncher() readerout.ExternalLauncher {
	return &OSExternalLauncher{}
}

func (l *OSExternalLauncher) Open(_ context.Context, target string) error {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", target)
	case "linux":
		cmd = exec.Command("xdg-open", target)
	default:
		return fmt.Errorf("external open is not supported on %s", runtime.GOOS)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("open external target: %w", err)
	}
	return nil
}
