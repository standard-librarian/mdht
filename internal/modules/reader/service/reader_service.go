package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"mdht/internal/modules/reader/domain"
	readerout "mdht/internal/modules/reader/port/out"
)

type ReaderService struct {
	mdReader         readerout.MarkdownReader
	pdfReader        readerout.PDFReader
	progress         readerout.ProgressPort
	sourceResolver   readerout.SourceResolver
	externalLauncher readerout.ExternalLauncher
}

func NewReaderService(
	mdReader readerout.MarkdownReader,
	pdfReader readerout.PDFReader,
	progress readerout.ProgressPort,
	sourceResolver readerout.SourceResolver,
	externalLauncher readerout.ExternalLauncher,
) *ReaderService {
	return &ReaderService{
		mdReader:         mdReader,
		pdfReader:        pdfReader,
		progress:         progress,
		sourceResolver:   sourceResolver,
		externalLauncher: externalLauncher,
	}
}

func (s *ReaderService) OpenMarkdown(ctx context.Context, path string) (string, error) {
	return s.mdReader.Read(ctx, path)
}

func (s *ReaderService) OpenPDF(ctx context.Context, path string, page int) (domain.Page, int, error) {
	if page <= 0 {
		page = 1
	}
	return s.pdfReader.ReadPage(ctx, path, page)
}

func (s *ReaderService) OpenSource(ctx context.Context, sourceID, mode string, page int, launchExternal bool) (string, domain.SourceRef, domain.Page, int, string, bool, error) {
	if s.sourceResolver == nil {
		return "", domain.SourceRef{}, domain.Page{}, 0, "", false, fmt.Errorf("source resolver is not configured")
	}
	source, err := s.sourceResolver.Resolve(ctx, sourceID)
	if err != nil {
		return "", domain.SourceRef{}, domain.Page{}, 0, "", false, err
	}

	resolvedMode, err := resolveMode(mode, source)
	if err != nil {
		return "", domain.SourceRef{}, domain.Page{}, 0, "", false, err
	}

	switch resolvedMode {
	case "markdown":
		if source.FilePath == "" {
			return "", domain.SourceRef{}, domain.Page{}, 0, "", false, fmt.Errorf("source has no readable file path")
		}
		content, err := s.OpenMarkdown(ctx, source.FilePath)
		if err != nil {
			return "", domain.SourceRef{}, domain.Page{}, 0, "", false, err
		}
		return resolvedMode, source, domain.Page{}, 0, content, false, nil
	case "pdf":
		if source.FilePath == "" {
			return "", domain.SourceRef{}, domain.Page{}, 0, "", false, fmt.Errorf("source has no readable file path")
		}
		pdfPage, total, err := s.OpenPDF(ctx, source.FilePath, page)
		if err != nil {
			return "", domain.SourceRef{}, domain.Page{}, 0, "", false, err
		}
		return resolvedMode, source, pdfPage, total, pdfPage.Text, false, nil
	case "external":
		target := source.URL
		if target == "" {
			target = source.FilePath
		}
		if target == "" {
			return "", domain.SourceRef{}, domain.Page{}, 0, "", false, fmt.Errorf("source has no external target")
		}
		launched := false
		if launchExternal && s.externalLauncher != nil {
			if err := s.externalLauncher.Open(ctx, target); err != nil {
				return "", domain.SourceRef{}, domain.Page{}, 0, "", false, err
			}
			launched = true
		}
		content := fmt.Sprintf("type=%s title=%s target=%s", source.Type, source.Title, target)
		return resolvedMode, source, domain.Page{}, 0, content, launched, nil
	default:
		return "", domain.SourceRef{}, domain.Page{}, 0, "", false, fmt.Errorf("unsupported reader mode")
	}
}

func resolveMode(mode string, source domain.SourceRef) (string, error) {
	mode = strings.ToLower(strings.TrimSpace(mode))
	if mode == "" {
		mode = "auto"
	}
	if mode != "auto" && mode != "markdown" && mode != "pdf" {
		return "", fmt.Errorf("invalid mode %q", mode)
	}
	if mode != "auto" {
		return mode, nil
	}
	sourceType := strings.ToLower(source.Type)
	if sourceType == "video" || sourceType == "course" {
		return "external", nil
	}
	ext := strings.ToLower(filepath.Ext(source.FilePath))
	switch ext {
	case ".pdf":
		return "pdf", nil
	case ".md", ".markdown", ".txt":
		return "markdown", nil
	}
	if source.URL != "" {
		return "external", nil
	}
	return "markdown", nil
}

func (s *ReaderService) UpdateProgress(ctx context.Context, sourceID string, percent float64, unitCurrent, unitTotal int) error {
	if s.progress == nil {
		return fmt.Errorf("progress port is not configured")
	}
	return s.progress.Update(ctx, sourceID, percent, unitCurrent, unitTotal)
}
