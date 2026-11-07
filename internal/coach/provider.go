package coach

import (
	"context"
)

// CoachProvider defines the streaming contract for local or cloud LLM backends.
type CoachProvider interface {
	// Name returns the provider identifier (e.g. "ollama", "cloud").
	Name() string

	// HealthCheck returns nil if the backend endpoint is reachable.
	HealthCheck(ctx context.Context) error

	// Stream streams tokens asynchronously into channels.Tokens.
	Stream(ctx context.Context, req CoachRequest, channels StreamChannels) error
}
