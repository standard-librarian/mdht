package domain

import "time"

const SchemaVersion = 1

type ActiveSession struct {
	SessionID   string    `json:"session_id"`
	SourceID    string    `json:"source_id"`
	SourceTitle string    `json:"source_title"`
	StartedAt   time.Time `json:"started_at"`
	Goal        string    `json:"goal"`
}

type Session struct {
	ID             string
	SourceID       string
	SourceTitle    string
	StartedAt      time.Time
	EndedAt        time.Time
	DurationMin    int
	Goal           string
	Outcome        string
	DeltaProgress  float64
	ProgressBefore float64
	ProgressAfter  float64
}
