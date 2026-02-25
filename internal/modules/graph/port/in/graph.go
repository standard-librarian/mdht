package in

import "context"

type SyncSourceInput struct {
	SourceID    string
	SourceTitle string
	Topics      []string
}

type Usecase interface {
	SyncSource(ctx context.Context, input SyncSourceInput) error
}
