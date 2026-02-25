package domain

import (
	"fmt"
	"strings"
	"time"
)

type SourceType string

const (
	SourceTypeBook    SourceType = "book"
	SourceTypeArticle SourceType = "article"
	SourceTypePaper   SourceType = "paper"
	SourceTypeVideo   SourceType = "video"
	SourceTypeCourse  SourceType = "course"
)

const (
	ManagedLinksStart = "<!-- mdht:links:start -->"
	ManagedLinksEnd   = "<!-- mdht:links:end -->"
	SchemaVersion     = 1
)

type Source struct {
	ID              string
	Type            SourceType
	Title           string
	Authors         []string
	URL             string
	FilePath        string
	NotePath        string
	Slug            string
	Tags            []string
	Topics          []string
	Status          string
	ProgressPct     float64
	UnitKind        string
	UnitCurrent     int
	UnitTotal       int
	AddedAt         time.Time
	UpdatedAt       time.Time
	LastSessionID   string
	ManagedWikilink []string
}

func (s SourceType) Validate() error {
	switch s {
	case SourceTypeBook, SourceTypeArticle, SourceTypePaper, SourceTypeVideo, SourceTypeCourse:
		return nil
	default:
		return fmt.Errorf("unsupported source type %q", string(s))
	}
}

func (s Source) Validate() error {
	if err := s.Type.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(s.Title) == "" {
		return fmt.Errorf("title is required")
	}
	if strings.TrimSpace(s.ID) == "" {
		return fmt.Errorf("id is required")
	}
	if strings.TrimSpace(s.Slug) == "" {
		return fmt.Errorf("slug is required")
	}
	return nil
}

type SourceDocument struct {
	Source Source
	Body   string
}
