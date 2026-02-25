package domain_test

import (
	"testing"
	"time"

	"mdht/internal/modules/library/domain"
)

func TestSourceTypeValidate(t *testing.T) {
	t.Parallel()
	if err := domain.SourceType("book").Validate(); err != nil {
		t.Fatalf("book should be valid: %v", err)
	}
	if err := domain.SourceType("unknown").Validate(); err == nil {
		t.Fatalf("unknown source type should fail")
	}
}

func TestSourceValidate(t *testing.T) {
	t.Parallel()
	now := time.Now().UTC()
	base := domain.Source{
		ID:        "id-1",
		Type:      domain.SourceTypeBook,
		Title:     "Sample",
		Slug:      "sample",
		AddedAt:   now,
		UpdatedAt: now,
	}

	if err := base.Validate(); err != nil {
		t.Fatalf("source should be valid: %v", err)
	}
	invalidType := base
	invalidType.Type = "mystery"
	if err := invalidType.Validate(); err == nil {
		t.Fatalf("invalid type should fail")
	}
	missingTitle := base
	missingTitle.Title = ""
	if err := missingTitle.Validate(); err == nil {
		t.Fatalf("missing title should fail")
	}
	missingID := base
	missingID.ID = ""
	if err := missingID.Validate(); err == nil {
		t.Fatalf("missing id should fail")
	}
	missingSlug := base
	missingSlug.Slug = ""
	if err := missingSlug.Validate(); err == nil {
		t.Fatalf("missing slug should fail")
	}
}
