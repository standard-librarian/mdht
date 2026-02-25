package dto

import "time"

type StartInput struct {
	SourceID    string
	SourceTitle string
	Goal        string
}

type StartOutput struct {
	SessionID string
	SourceID  string
	StartedAt time.Time
}

type EndInput struct {
	SessionID     string
	Outcome       string
	DeltaProgress float64
}

type EndOutput struct {
	SessionID      string
	SourceID       string
	Path           string
	DurationMin    int
	DeltaProgress  float64
	ProgressBefore float64
	ProgressAfter  float64
}

type ActiveSessionOutput struct {
	SessionID   string
	SourceID    string
	SourceTitle string
	StartedAt   time.Time
	Goal        string
}
