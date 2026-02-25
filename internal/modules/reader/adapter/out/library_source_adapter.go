package out

import (
	"context"

	libraryin "mdht/internal/modules/library/port/in"
	"mdht/internal/modules/reader/domain"
	readerout "mdht/internal/modules/reader/port/out"
)

type LibrarySourceAdapter struct {
	library libraryin.Usecase
}

func NewLibrarySourceAdapter(library libraryin.Usecase) readerout.SourceResolver {
	return &LibrarySourceAdapter{library: library}
}

func (a *LibrarySourceAdapter) Resolve(ctx context.Context, sourceID string) (domain.SourceRef, error) {
	source, err := a.library.GetSource(ctx, sourceID)
	if err != nil {
		return domain.SourceRef{}, err
	}
	return domain.SourceRef{
		ID:       source.ID,
		Title:    source.Title,
		Type:     source.Type,
		URL:      source.URL,
		FilePath: source.FilePath,
		NotePath: source.NotePath,
		Percent:  source.Percent,
	}, nil
}
