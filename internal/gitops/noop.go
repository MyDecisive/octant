package gitops

import "context"

// NoopPublisher is used when no GitHub App integration is configured, or as a safe fallback for
// callers constructed without a Publisher (e.g. in tests).
type NoopPublisher struct{}

var _ Publisher = (*NoopPublisher)(nil)

func NewNoopPublisher() *NoopPublisher {
	return &NoopPublisher{}
}

func (*NoopPublisher) PublishManifests(context.Context, PublishInput) (*PublishResult, error) {
	return nil, nil // nolint:nilnil
}
