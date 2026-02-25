package usecase

import (
	"context"

	graphin "mdht/internal/modules/graph/port/in"
	"mdht/internal/modules/library/domain"
	"mdht/internal/modules/library/dto"
	libraryin "mdht/internal/modules/library/port/in"
	"mdht/internal/modules/library/service"
)

type Interactor struct {
	svc   *service.SourceService
	graph graphin.Usecase
}

func NewInteractor(svc *service.SourceService, graph graphin.Usecase) libraryin.Usecase {
	return &Interactor{svc: svc, graph: graph}
}

func (i *Interactor) IngestFile(ctx context.Context, input dto.IngestFileInput) (dto.SourceOutput, error) {
	source, path, err := i.svc.IngestFile(ctx, domain.SourceType(input.Type), input.Path, input.Title, input.Tags, input.Topics)
	if err != nil {
		return dto.SourceOutput{}, err
	}
	if i.graph != nil {
		_ = i.graph.SyncSource(ctx, graphin.SyncSourceInput{SourceID: source.ID, SourceTitle: source.Title, Topics: source.Topics})
	}
	return dto.SourceOutput{ID: source.ID, Title: source.Title, Type: string(source.Type), NotePath: path, Percent: source.ProgressPct}, nil
}

func (i *Interactor) IngestURL(ctx context.Context, input dto.IngestURLInput) (dto.SourceOutput, error) {
	source, path, err := i.svc.IngestURL(ctx, domain.SourceType(input.Type), input.URL, input.Title, input.Tags, input.Topics)
	if err != nil {
		return dto.SourceOutput{}, err
	}
	if i.graph != nil {
		_ = i.graph.SyncSource(ctx, graphin.SyncSourceInput{SourceID: source.ID, SourceTitle: source.Title, Topics: source.Topics})
	}
	return dto.SourceOutput{ID: source.ID, Title: source.Title, Type: string(source.Type), NotePath: path, Percent: source.ProgressPct}, nil
}

func (i *Interactor) UpdateProgress(ctx context.Context, input dto.UpdateProgressInput) (dto.SourceOutput, error) {
	source, err := i.svc.UpdateProgress(ctx, input.SourceID, input.Percent, input.UnitCurrent, input.UnitTotal)
	if err != nil {
		return dto.SourceOutput{}, err
	}
	return dto.SourceOutput{ID: source.ID, Title: source.Title, Type: string(source.Type), Percent: source.ProgressPct, NotePath: source.NotePath}, nil
}

func (i *Interactor) ListSources(ctx context.Context) ([]dto.SourceOutput, error) {
	sources, err := i.svc.ListSources(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]dto.SourceOutput, 0, len(sources))
	for _, source := range sources {
		out = append(out, dto.SourceOutput{
			ID:       source.ID,
			Title:    source.Title,
			Type:     string(source.Type),
			Percent:  source.ProgressPct,
			NotePath: source.NotePath,
		})
	}
	return out, nil
}

func (i *Interactor) GetSource(ctx context.Context, id string) (dto.SourceDetailOutput, error) {
	source, err := i.svc.GetSource(ctx, id)
	if err != nil {
		return dto.SourceDetailOutput{}, err
	}
	return dto.SourceDetailOutput{
		ID:          source.ID,
		Title:       source.Title,
		Type:        string(source.Type),
		URL:         source.URL,
		FilePath:    source.FilePath,
		NotePath:    source.NotePath,
		Status:      source.Status,
		Percent:     source.ProgressPct,
		UnitKind:    source.UnitKind,
		UnitCurrent: source.UnitCurrent,
		UnitTotal:   source.UnitTotal,
		Topics:      source.Topics,
		Tags:        source.Tags,
	}, nil
}

func (i *Interactor) Reindex(ctx context.Context, _ dto.ReindexInput) error {
	return i.svc.Reindex(ctx)
}
