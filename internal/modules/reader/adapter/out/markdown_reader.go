package out

import (
	"context"
	"fmt"
	"os"

	readerout "mdht/internal/modules/reader/port/out"
)

type LocalMarkdownReader struct{}

func NewLocalMarkdownReader() readerout.MarkdownReader {
	return &LocalMarkdownReader{}
}

func (r *LocalMarkdownReader) Read(_ context.Context, path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read markdown: %w", err)
	}
	return string(b), nil
}
