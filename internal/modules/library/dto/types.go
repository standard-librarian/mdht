package dto

type IngestFileInput struct {
	Path   string
	Type   string
	Title  string
	Topics []string
	Tags   []string
}

type IngestURLInput struct {
	URL    string
	Type   string
	Title  string
	Topics []string
	Tags   []string
}

type UpdateProgressInput struct {
	SourceID    string
	Percent     float64
	UnitCurrent int
	UnitTotal   int
}

type ReindexInput struct{}

type SourceOutput struct {
	ID       string
	Title    string
	Type     string
	Percent  float64
	NotePath string
}

type SourceDetailOutput struct {
	ID          string
	Title       string
	Type        string
	URL         string
	FilePath    string
	NotePath    string
	Status      string
	Percent     float64
	UnitKind    string
	UnitCurrent int
	UnitTotal   int
	Topics      []string
	Tags        []string
}
