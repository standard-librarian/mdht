package domain

type Page struct {
	Number int
	Text   string
}

type SourceRef struct {
	ID       string
	Title    string
	Type     string
	URL      string
	FilePath string
	NotePath string
	Percent  float64
}
