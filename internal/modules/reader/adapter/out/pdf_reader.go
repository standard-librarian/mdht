package out

import (
	"context"
	"fmt"
	"strings"

	"mdht/internal/modules/reader/domain"
	readerout "mdht/internal/modules/reader/port/out"
	"rsc.io/pdf"
)

type LocalPDFReader struct{}

func NewLocalPDFReader() readerout.PDFReader {
	return &LocalPDFReader{}
}

func (r *LocalPDFReader) ReadPage(_ context.Context, path string, page int) (domain.Page, int, error) {
	doc, err := pdf.Open(path)
	if err != nil {
		return domain.Page{}, 0, fmt.Errorf("open pdf: %w", err)
	}
	total := doc.NumPage()
	if total == 0 {
		return domain.Page{Number: 1, Text: ""}, 0, nil
	}
	if page > total {
		page = total
	}
	p := doc.Page(page)
	if p.V.IsNull() {
		return domain.Page{}, total, fmt.Errorf("pdf page %d is null", page)
	}
	content := p.Content()
	parts := make([]string, 0, len(content.Text))
	for _, text := range content.Text {
		if strings.TrimSpace(text.S) == "" {
			continue
		}
		parts = append(parts, text.S)
	}
	return domain.Page{Number: page, Text: strings.Join(parts, " ")}, total, nil
}
