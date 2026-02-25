package usecase_test

import (
	"context"
	"fmt"
	"testing"

	"mdht/internal/modules/reader/domain"
	"mdht/internal/modules/reader/dto"
	"mdht/internal/modules/reader/service"
	"mdht/internal/modules/reader/usecase"
)

type errMDReader struct{}

func (errMDReader) Read(context.Context, string) (string, error) { return "", fmt.Errorf("md fail") }

type errPDFReader struct{}

func (errPDFReader) ReadPage(context.Context, string, int) (domain.Page, int, error) {
	return domain.Page{}, 0, fmt.Errorf("pdf fail")
}

func TestOpenMarkdownAndPDFErrors(t *testing.T) {
	t.Parallel()
	uc := usecase.NewInteractor(service.NewReaderService(
		errMDReader{},
		errPDFReader{},
		&fakeProgress{},
		fakeResolver{source: domain.SourceRef{ID: "s-1", Title: "Doc", Type: "article", FilePath: "/tmp/readme.md", Percent: 11}},
		&fakeLauncher{},
	))
	if _, err := uc.OpenMarkdown(context.Background(), dto.OpenMarkdownInput{Path: "/tmp/a.md"}); err == nil {
		t.Fatalf("expected markdown error")
	}
	if _, err := uc.OpenPDF(context.Background(), dto.OpenPDFInput{Path: "/tmp/a.pdf", Page: 2}); err == nil {
		t.Fatalf("expected pdf error")
	}
}
