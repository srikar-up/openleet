package coach

import "openleet/internal/model"

// Action represents the kind of Socratic assistance requested.
type Action string

const (
	ActionHint       Action = "hint"       // Next conceptual step or nudge
	ActionExplain    Action = "explain"    // Explain problem, examples, constraints
	ActionComplexity Action = "complexity" // Analyze asymptotic time/space complexity
	ActionDiagnose   Action = "diagnose"   // Analyze why a test case failed
	ActionQuestion   Action = "question"   // Free-form user question
)

// GenerationOptions controls model sampling and privacy for every provider.
type GenerationOptions struct {
	Temperature float64
	MaxTokens   int
	IncludeCode bool
}

// DefaultGenerationOptions returns the recommended baseline generation settings.
func DefaultGenerationOptions() GenerationOptions {
	return GenerationOptions{
		Temperature: 0.2,
		MaxTokens:   1024,
		IncludeCode: true,
	}
}

// CoachRequest contains the context needed for Socratic coaching.
type CoachRequest struct {
	Action         Action
	Problem        *model.Problem
	UserCode       string
	Language       string
	TestResults    *model.ExecutionResult
	UserQuestion   string
	Temperature    float64
	TemperatureSet bool
	MaxTokens      int
}

// StreamChannels holds the output channels for non-blocking token delivery.
//
// Ownership: the provider only sends; the Manager closes Tokens (then Done)
// when the stream ends, after queueing any terminal error. Consumers must treat
// Tokens closure or Done as "drain what is buffered, then finish".
type StreamChannels struct {
	Tokens chan string
	Errors chan error
	Done   chan struct{}
}

// NewStreamChannels creates initialized buffered channels.
func NewStreamChannels() StreamChannels {
	return StreamChannels{
		Tokens: make(chan string, 100),
		Errors: make(chan error, 1),
		Done:   make(chan struct{}),
	}
}

// Close safely shuts down the streaming channels.
func (sc StreamChannels) Close() {
	select {
	case <-sc.Done:
		// already closed
	default:
		close(sc.Done)
	}
}
