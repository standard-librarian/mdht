package config

import (
	"fmt"
	"path/filepath"
)

type Config struct {
	VaultPath string
	DBPath    string
}

func New(vaultPath string) (Config, error) {
	if vaultPath == "" {
		return Config{}, fmt.Errorf("vault path is required")
	}
	return Config{
		VaultPath: vaultPath,
		DBPath:    filepath.Join(vaultPath, ".mdht", "mdht.db"),
	}, nil
}
