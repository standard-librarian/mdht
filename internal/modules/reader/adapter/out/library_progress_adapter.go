package out

import (
	"context"

	"mdht/internal/modules/library/dto"
	libraryin "mdht/internal/modules/library/port/in"
	readerout "mdht/internal/modules/reader/port/out"
)

type LibraryProgressAdapter struct {
	library libraryin.Usecase
}

func NewLibraryProgressAdapter(library libraryin.Usecase) readerout.ProgressPort {
	return &LibraryProgressAdapter{library: library}
}

func (a *LibraryProgressAdapter) Update(ctx context.Context, sourceID string, percent float64, unitCurrent, unitTotal int) error {
	_, err := a.library.UpdateProgress(ctx, dto.UpdateProgressInput{
		SourceID:    sourceID,
		Percent:     percent,
		UnitCurrent: unitCurrent,
		UnitTotal:   unitTotal,
	})
	return err
}
