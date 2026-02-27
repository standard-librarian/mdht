package dto

type TopicNote struct {
	TopicSlug string
	Body      string
}

type TopicSummaryOutput struct {
	TopicSlug   string `json:"topic_slug"`
	SourceCount int    `json:"source_count"`
}

type NodeOutput struct {
	ID    string `json:"id"`
	Label string `json:"label"`
	Kind  string `json:"kind"`
}

type NeighborsOutput struct {
	FocusID string       `json:"focus_id"`
	Depth   int          `json:"depth"`
	Nodes   []NodeOutput `json:"nodes"`
}

type PathOutput struct {
	FromID string       `json:"from_id"`
	ToID   string       `json:"to_id"`
	Found  bool         `json:"found"`
	Nodes  []NodeOutput `json:"nodes"`
}
