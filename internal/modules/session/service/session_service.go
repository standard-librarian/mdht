package service

import (
	"context"
	"fmt"

	"mdht/internal/modules/session/domain"
	sessionout "mdht/internal/modules/session/port/out"
	"mdht/internal/platform/clock"
	"mdht/internal/platform/id"
)

type SessionService struct {
	clock clock.Clock
	idGen id.Generator
	store sessionout.SessionStore
}

func NewSessionService(clock clock.Clock, idGen id.Generator, store sessionout.SessionStore) *SessionService {
	return &SessionService{clock: clock, idGen: idGen, store: store}
}

func (s *SessionService) Start(_ context.Context, sourceID, sourceTitle, goal string) (domain.ActiveSession, error) {
	if sourceID == "" {
		return domain.ActiveSession{}, fmt.Errorf("source id is required")
	}
	return domain.ActiveSession{
		SessionID:   s.idGen.New(),
		SourceID:    sourceID,
		SourceTitle: sourceTitle,
		StartedAt:   s.clock.Now(),
		Goal:        goal,
	}, nil
}

func (s *SessionService) End(ctx context.Context, active domain.ActiveSession, outcome string, deltaProgress, progressBefore, progressAfter float64) (domain.Session, string, error) {
	endedAt := s.clock.Now()
	duration := int(endedAt.Sub(active.StartedAt).Minutes())
	if duration < 0 {
		duration = 0
	}
	session := domain.Session{
		ID:             active.SessionID,
		SourceID:       active.SourceID,
		SourceTitle:    active.SourceTitle,
		StartedAt:      active.StartedAt,
		EndedAt:        endedAt,
		DurationMin:    duration,
		Goal:           active.Goal,
		Outcome:        outcome,
		DeltaProgress:  deltaProgress,
		ProgressBefore: progressBefore,
		ProgressAfter:  progressAfter,
	}
	path, err := s.store.Save(ctx, session)
	if err != nil {
		return domain.Session{}, "", err
	}
	return session, path, nil
}
