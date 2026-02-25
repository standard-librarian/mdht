package usecase_test

import (
	"context"
	"testing"

	"mdht/internal/modules/reader/domain"
	"mdht/internal/modules/reader/dto"
	"mdht/internal/modules/reader/service"
	"mdht/internal/modules/reader/usecase"
)

type fakeMDReader struct{}

func (fakeMDReader) Read(context.Context, string) (string, error) { return "# ok", nil }

type fakePDFReader struct{}

func (fakePDFReader) ReadPage(context.Context, string, int) (domain.Page, int, error) {
	return domain.Page{Number: 2, Text: "pdf text"}, 10, nil
}

type fakeProgress struct {
	called bool
}

func (f *fakeProgress) Update(context.Context, string, float64, int, int) error {
	f.called = true
	return nil
}

type fakeResolver struct {
	source domain.SourceRef
}

func (r fakeResolver) Resolve(context.Context, string) (domain.SourceRef, error) {
	return r.source, nil
}

type fakeLauncher struct {
	opened string
}

func (l *fakeLauncher) Open(_ context.Context, target string) error {
	l.opened = target
	return nil
}

func TestOpenMarkdownAndPDFDirect(t *testing.T) {
	t.Parallel()
	uc := usecase.NewInteractor(service.NewReaderService(
		fakeMDReader{},
		fakePDFReader{},
		&fakeProgress{},
		fakeResolver{source: domain.SourceRef{ID: "s-1", Title: "Doc", Type: "article", FilePath: "/tmp/readme.md", Percent: 11}},
		&fakeLauncher{},
	))
	md, err := uc.OpenMarkdown(context.Background(), dto.OpenMarkdownInput{Path: "/tmp/a.md"})
	if err != nil {
		t.Fatalf("open markdown: %v", err)
	}
	if md.Content == "" {
		t.Fatalf("markdown content should not be empty")
	}
	pdf, err := uc.OpenPDF(context.Background(), dto.OpenPDFInput{Path: "/tmp/a.pdf", Page: 2})
	if err != nil {
		t.Fatalf("open pdf: %v", err)
	}
	if pdf.Page != 2 || pdf.TotalPage != 10 {
		t.Fatalf("unexpected pdf output: %+v", pdf)
	}
}

func TestOpenSourceAutoModes(t *testing.T) {
	t.Parallel()
	launcher := &fakeLauncher{}
	svc := service.NewReaderService(
		fakeMDReader{},
		fakePDFReader{},
		&fakeProgress{},
		fakeResolver{source: domain.SourceRef{ID: "s-1", Title: "Doc", Type: "article", FilePath: "/tmp/readme.md", Percent: 11}},
		launcher,
	)
	uc := usecase.NewInteractor(svc)

	out, err := uc.OpenSource(context.Background(), dto.OpenSourceInput{SourceID: "s-1", Mode: "auto"})
	if err != nil {
		t.Fatalf("open source: %v", err)
	}
	if out.Mode != "markdown" || out.Content == "" {
		t.Fatalf("expected markdown mode with content, got %+v", out)
	}

	svc = service.NewReaderService(
		fakeMDReader{},
		fakePDFReader{},
		&fakeProgress{},
		fakeResolver{source: domain.SourceRef{ID: "s-2", Title: "PDF", Type: "paper", FilePath: "/tmp/file.pdf", Percent: 20}},
		launcher,
	)
	uc = usecase.NewInteractor(svc)
	out, err = uc.OpenSource(context.Background(), dto.OpenSourceInput{SourceID: "s-2", Mode: "auto", Page: 2})
	if err != nil {
		t.Fatalf("open pdf source: %v", err)
	}
	if out.Mode != "pdf" || out.Page != 2 || out.TotalPage != 10 {
		t.Fatalf("expected pdf mode page info, got %+v", out)
	}

	svc = service.NewReaderService(
		fakeMDReader{},
		fakePDFReader{},
		&fakeProgress{},
		fakeResolver{source: domain.SourceRef{ID: "s-3", Title: "Video", Type: "video", URL: "https://example.com/video", Percent: 33}},
		launcher,
	)
	uc = usecase.NewInteractor(svc)
	out, err = uc.OpenSource(context.Background(), dto.OpenSourceInput{SourceID: "s-3", Mode: "auto", LaunchExternal: true})
	if err != nil {
		t.Fatalf("open video source: %v", err)
	}
	if out.Mode != "external" || !out.ExternalLaunched || launcher.opened == "" {
		t.Fatalf("expected external launch, got %+v", out)
	}
}

func TestOpenSourceInvalidModeAndUpdateProgress(t *testing.T) {
	t.Parallel()
	progress := &fakeProgress{}
	svc := service.NewReaderService(
		fakeMDReader{},
		fakePDFReader{},
		progress,
		fakeResolver{source: domain.SourceRef{ID: "s-1", Title: "Doc", Type: "article", FilePath: "/tmp/readme.md", Percent: 11}},
		&fakeLauncher{},
	)
	uc := usecase.NewInteractor(svc)

	if _, err := uc.OpenSource(context.Background(), dto.OpenSourceInput{SourceID: "s-1", Mode: "broken"}); err == nil {
		t.Fatalf("invalid mode should fail")
	}
	if err := uc.UpdateProgress(context.Background(), dto.UpdateProgressInput{SourceID: "s-1", Percent: 20}); err != nil {
		t.Fatalf("update progress: %v", err)
	}
	if !progress.called {
		t.Fatalf("expected progress port to be called")
	}
}

func TestOpenSourceExternalWithoutTargetFails(t *testing.T) {
	t.Parallel()
	svc := service.NewReaderService(
		fakeMDReader{},
		fakePDFReader{},
		&fakeProgress{},
		fakeResolver{source: domain.SourceRef{ID: "s-4", Title: "Video", Type: "video"}},
		&fakeLauncher{},
	)
	uc := usecase.NewInteractor(svc)
	if _, err := uc.OpenSource(context.Background(), dto.OpenSourceInput{SourceID: "s-4", Mode: "auto"}); err == nil {
		t.Fatalf("missing external target should fail")
	}
}
