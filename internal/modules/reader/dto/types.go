package dto

type OpenMarkdownInput struct {
	Path string
}

type OpenMarkdownOutput struct {
	Content string
}

type OpenPDFInput struct {
	Path string
	Page int
}

type OpenPDFOutput struct {
	Page      int
	TotalPage int
	Text      string
}

type OpenSourceInput struct {
	SourceID       string
	Mode           string
	Page           int
	LaunchExternal bool
}

type OpenResult struct {
	SourceID         string
	Title            string
	Type             string
	Mode             string
	Page             int
	TotalPage        int
	Content          string
	ExternalTarget   string
	ExternalLaunched bool
	Percent          float64
}

type UpdateProgressInput struct {
	SourceID    string
	Percent     float64
	UnitCurrent int
	UnitTotal   int
}
