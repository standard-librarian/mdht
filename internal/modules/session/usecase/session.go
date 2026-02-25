package usecase

import (
	"context"
	"fmt"

	"mdht/internal/modules/library/dto"
	libraryin "mdht/internal/modules/library/port/in"
	sessiondto "mdht/internal/modules/session/dto"
	sessionin "mdht/internal/modules/session/port/in"
	sessionout "mdht/internal/modules/session/port/out"
	"mdht/internal/modules/session/service"
	apperrors "mdht/internal/platform/errors"
)

type Interactor struct {
	svc         *service.SessionService
	library     libraryin.Usecase
	activeStore sessionout.ActiveSessionStore
}

func NewInteractor(svc *service.SessionService, library libraryin.Usecase, activeStore sessionout.ActiveSessionStore) sessionin.Usecase {
	return &Interactor{svc: svc, library: library, activeStore: activeStore}
}

func (i *Interactor) Start(ctx context.Context, input sessiondto.StartInput) (sessiondto.StartOutput, error) {
	if i.activeStore != nil {
		_, err := i.activeStore.LoadActive(ctx)
		if err == nil {
			return sessiondto.StartOutput{}, apperrors.ErrActiveSessionExists
		}
		if err != nil && err != apperrors.ErrNoActiveSession {
			return sessiondto.StartOutput{}, err
		}
	}

	sourceTitle := input.SourceTitle
	if sourceTitle == "" && i.library != nil {
		source, err := i.library.GetSource(ctx, input.SourceID)
		if err != nil {
			return sessiondto.StartOutput{}, err
		}
		sourceTitle = source.Title
	}

	active, err := i.svc.Start(ctx, input.SourceID, sourceTitle, input.Goal)
	if err != nil {
		return sessiondto.StartOutput{}, err
	}
	if i.activeStore != nil {
		if err := i.activeStore.SaveActive(ctx, active); err != nil {
			return sessiondto.StartOutput{}, err
		}
	}
	return sessiondto.StartOutput{SessionID: active.SessionID, SourceID: active.SourceID, StartedAt: active.StartedAt}, nil
}

func (i *Interactor) End(ctx context.Context, input sessiondto.EndInput) (sessiondto.EndOutput, error) {
	if input.DeltaProgress < 0 {
		return sessiondto.EndOutput{}, fmt.Errorf("delta progress must be non-negative")
	}
	if i.activeStore == nil {
		return sessiondto.EndOutput{}, apperrors.ErrNoActiveSession
	}

	active, err := i.activeStore.LoadActive(ctx)
	if err != nil {
		return sessiondto.EndOutput{}, err
	}
	if input.SessionID != "" && input.SessionID != active.SessionID {
		return sessiondto.EndOutput{}, fmt.Errorf("session id mismatch")
	}
	if i.library == nil {
		return sessiondto.EndOutput{}, fmt.Errorf("library usecase is not configured")
	}

	source, err := i.library.GetSource(ctx, active.SourceID)
	if err != nil {
		return sessiondto.EndOutput{}, err
	}
	before := source.Percent
	after := before + input.DeltaProgress
	if after > 100 {
		after = 100
	}
	if after < 0 {
		after = 0
	}

	session, path, err := i.svc.End(ctx, active, input.Outcome, input.DeltaProgress, before, after)
	if err != nil {
		return sessiondto.EndOutput{}, err
	}
	if _, err := i.library.UpdateProgress(ctx, dto.UpdateProgressInput{SourceID: active.SourceID, Percent: after}); err != nil {
		return sessiondto.EndOutput{}, err
	}
	if err := i.activeStore.ClearActive(ctx); err != nil {
		return sessiondto.EndOutput{}, err
	}

	return sessiondto.EndOutput{
		SessionID:      session.ID,
		SourceID:       session.SourceID,
		Path:           path,
		DurationMin:    session.DurationMin,
		DeltaProgress:  session.DeltaProgress,
		ProgressBefore: session.ProgressBefore,
		ProgressAfter:  session.ProgressAfter,
	}, nil
}

func (i *Interactor) GetActive(ctx context.Context) (sessiondto.ActiveSessionOutput, error) {
	if i.activeStore == nil {
		return sessiondto.ActiveSessionOutput{}, apperrors.ErrNoActiveSession
	}
	active, err := i.activeStore.LoadActive(ctx)
	if err != nil {
		return sessiondto.ActiveSessionOutput{}, err
	}
	return sessiondto.ActiveSessionOutput{
		SessionID:   active.SessionID,
		SourceID:    active.SourceID,
		SourceTitle: active.SourceTitle,
		StartedAt:   active.StartedAt,
		Goal:        active.Goal,
	}, nil
}
