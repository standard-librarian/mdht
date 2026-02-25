package usecase

import (
	"context"

	"mdht/internal/modules/reader/dto"
	readerin "mdht/internal/modules/reader/port/in"
	"mdht/internal/modules/reader/service"
)

type Interactor struct {
	svc *service.ReaderService
}

func NewInteractor(svc *service.ReaderService) readerin.Usecase {
	return &Interactor{svc: svc}
}

func (i *Interactor) OpenMarkdown(ctx context.Context, input dto.OpenMarkdownInput) (dto.OpenMarkdownOutput, error) {
	content, err := i.svc.OpenMarkdown(ctx, input.Path)
	if err != nil {
		return dto.OpenMarkdownOutput{}, err
	}
	return dto.OpenMarkdownOutput{Content: content}, nil
}

func (i *Interactor) OpenPDF(ctx context.Context, input dto.OpenPDFInput) (dto.OpenPDFOutput, error) {
	page, total, err := i.svc.OpenPDF(ctx, input.Path, input.Page)
	if err != nil {
		return dto.OpenPDFOutput{}, err
	}
	return dto.OpenPDFOutput{Page: page.Number, TotalPage: total, Text: page.Text}, nil
}

func (i *Interactor) OpenSource(ctx context.Context, input dto.OpenSourceInput) (dto.OpenResult, error) {
	mode, source, page, total, content, launched, err := i.svc.OpenSource(ctx, input.SourceID, input.Mode, input.Page, input.LaunchExternal)
	if err != nil {
		return dto.OpenResult{}, err
	}
	result := dto.OpenResult{
		SourceID:         source.ID,
		Title:            source.Title,
		Type:             source.Type,
		Mode:             mode,
		Page:             page.Number,
		TotalPage:        total,
		Content:          content,
		Percent:          source.Percent,
		ExternalLaunched: launched,
	}
	if mode == "external" {
		if source.URL != "" {
			result.ExternalTarget = source.URL
		} else {
			result.ExternalTarget = source.FilePath
		}
	}
	return result, nil
}

func (i *Interactor) UpdateProgress(ctx context.Context, input dto.UpdateProgressInput) error {
	return i.svc.UpdateProgress(ctx, input.SourceID, input.Percent, input.UnitCurrent, input.UnitTotal)
}
