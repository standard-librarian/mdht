package domain

type NodeKind string

const (
	NodeKindSource NodeKind = "source"
	NodeKindTopic  NodeKind = "topic"
)

type TopicSummary struct {
	TopicSlug   string
	SourceCount int
}

type Node struct {
	ID    string
	Label string
	Kind  NodeKind
}
