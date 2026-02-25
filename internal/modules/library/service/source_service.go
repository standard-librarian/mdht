package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"mdht/internal/modules/library/domain"
	libraryout "mdht/internal/modules/library/port/out"
	"mdht/internal/platform/clock"
	"mdht/internal/platform/id"
	"mdht/internal/platform/slug"
)

type SourceService struct {
	clock     clock.Clock
	idGen     id.Generator
	store     libraryout.SourceStore
	projector libraryout.SourceIndexProjector
}

func NewSourceService(clock clock.Clock, idGen id.Generator, store libraryout.SourceStore, projector libraryout.SourceIndexProjector) *SourceService {
	return &SourceService{clock: clock, idGen: idGen, store: store, projector: projector}
}

func (s *SourceService) IngestFile(ctx context.Context, sourceType domain.SourceType, filePath, title string, tags, topics []string) (domain.Source, string, error) {
	if err := sourceType.Validate(); err != nil {
		return domain.Source{}, "", err
	}
	if strings.TrimSpace(filePath) == "" {
		return domain.Source{}, "", fmt.Errorf("file path is required")
	}
	title = strings.TrimSpace(title)
	if title == "" {
		title = strings.TrimSuffix(filepath.Base(filePath), filepath.Ext(filePath))
	}
	now := s.clock.Now()
	source := domain.Source{
		ID:              s.idGen.New(),
		Type:            sourceType,
		Title:           title,
		FilePath:        filePath,
		Slug:            slug.Make(title),
		Tags:            tags,
		Topics:          topics,
		Status:          "active",
		ProgressPct:     0,
		UnitKind:        "page",
		AddedAt:         now,
		UpdatedAt:       now,
		ManagedWikilink: toTopicWikilinks(topics),
	}
	if err := source.Validate(); err != nil {
		return domain.Source{}, "", err
	}
	path, err := s.store.Save(ctx, domain.SourceDocument{Source: source})
	if err != nil {
		return domain.Source{}, "", err
	}
	source.NotePath = path
	if err := s.projector.UpsertSource(ctx, source); err != nil {
		return domain.Source{}, "", err
	}
	return source, path, nil
}

func (s *SourceService) IngestURL(ctx context.Context, sourceType domain.SourceType, url, title string, tags, topics []string) (domain.Source, string, error) {
	if err := sourceType.Validate(); err != nil {
		return domain.Source{}, "", err
	}
	if strings.TrimSpace(url) == "" {
		return domain.Source{}, "", fmt.Errorf("url is required")
	}
	now := s.clock.Now()
	title = strings.TrimSpace(title)
	if title == "" {
		title = url
	}
	source := domain.Source{
		ID:              s.idGen.New(),
		Type:            sourceType,
		Title:           title,
		URL:             url,
		Slug:            slug.Make(title),
		Tags:            tags,
		Topics:          topics,
		Status:          "active",
		ProgressPct:     0,
		UnitKind:        "unit",
		AddedAt:         now,
		UpdatedAt:       now,
		ManagedWikilink: toTopicWikilinks(topics),
	}
	if err := source.Validate(); err != nil {
		return domain.Source{}, "", err
	}
	path, err := s.store.Save(ctx, domain.SourceDocument{Source: source})
	if err != nil {
		return domain.Source{}, "", err
	}
	source.NotePath = path
	if err := s.projector.UpsertSource(ctx, source); err != nil {
		return domain.Source{}, "", err
	}
	return source, path, nil
}

func (s *SourceService) UpdateProgress(ctx context.Context, sourceID string, pct float64, unitCurrent, unitTotal int) (domain.Source, error) {
	doc, err := s.store.FindByID(ctx, sourceID)
	if err != nil {
		return domain.Source{}, err
	}
	if pct < 0 {
		pct = 0
	}
	if pct > 100 {
		pct = 100
	}
	doc.Source.ProgressPct = pct
	doc.Source.UnitCurrent = unitCurrent
	doc.Source.UnitTotal = unitTotal
	doc.Source.UpdatedAt = s.clock.Now()
	if _, err := s.store.Save(ctx, doc); err != nil {
		return domain.Source{}, err
	}
	if err := s.projector.UpsertSource(ctx, doc.Source); err != nil {
		return domain.Source{}, err
	}
	return doc.Source, nil
}

func (s *SourceService) ListSources(ctx context.Context) ([]domain.Source, error) {
	docs, err := s.store.List(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]domain.Source, 0, len(docs))
	for _, doc := range docs {
		out = append(out, doc.Source)
	}
	return out, nil
}

func (s *SourceService) GetSource(ctx context.Context, sourceID string) (domain.Source, error) {
	doc, err := s.store.FindByID(ctx, sourceID)
	if err != nil {
		return domain.Source{}, err
	}
	return doc.Source, nil
}

func (s *SourceService) Reindex(ctx context.Context) error {
	if err := s.projector.Reset(ctx); err != nil {
		return err
	}
	docs, err := s.store.List(ctx)
	if err != nil {
		return err
	}
	for _, doc := range docs {
		if err := s.projector.UpsertSource(ctx, doc.Source); err != nil {
			return err
		}
	}
	return nil
}

func toTopicWikilinks(topics []string) []string {
	out := make([]string, 0, len(topics))
	for _, topic := range topics {
		topic = strings.TrimSpace(topic)
		if topic == "" {
			continue
		}
		out = append(out, "[["+slug.Make(topic)+"]]")
	}
	return out
}
