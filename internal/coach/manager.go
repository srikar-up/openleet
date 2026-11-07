package coach

import (
	"context"
	"fmt"
	"sync"
	"time"

	tea "github.com/charmbracelet/bubbletea"
)

// Manager coordinates dual-engine Socratic coaching with automatic fallback.
type Manager struct {
	localProvider CoachProvider
	cloudProvider CoachProvider
	generation    GenerationOptions
	mu            sync.RWMutex

	activeMu     sync.Mutex
	activeCancel context.CancelFunc
	activeID     uint64
	streamSeq    uint64
}

// NewManager constructs an initialized Coach Manager.
func NewManager(local, cloud CoachProvider) *Manager {
	return &Manager{
		localProvider: local,
		cloudProvider: cloud,
		generation:    DefaultGenerationOptions(),
	}
}

// SetProviders swaps the local and cloud providers atomically. Streams that
// already selected a provider keep using their snapshot.
func (m *Manager) SetProviders(local, cloud CoachProvider) {
	if m == nil {
		return
	}
	m.mu.Lock()
	m.localProvider = local
	m.cloudProvider = cloud
	m.mu.Unlock()
}

// SetGenerationOptions updates the sampling and privacy settings for future streams.
func (m *Manager) SetGenerationOptions(opts GenerationOptions) {
	if m == nil {
		return
	}
	if opts.Temperature < 0 {
		opts.Temperature = 0
	} else if opts.Temperature > 2 {
		opts.Temperature = 2
	}

	if opts.MaxTokens <= 0 {
		opts.MaxTokens = 1024
	} else if opts.MaxTokens < 128 {
		opts.MaxTokens = 128
	} else if opts.MaxTokens > 8192 {
		opts.MaxTokens = 8192
	}

	m.mu.Lock()
	m.generation = opts
	m.mu.Unlock()
}

// Stream initiates a streaming session, choosing the healthiest provider.
func (m *Manager) Stream(ctx context.Context, req CoachRequest) (StreamChannels, string, uint64, error) {
	// Fast health check with 500ms deadline to avoid hanging the UI
	healthCtx, healthCancel := context.WithTimeout(ctx, 500*time.Millisecond)
	defer healthCancel()

	provider := m.selectProvider(healthCtx)
	if provider == nil {
		return StreamChannels{}, "", 0, fmt.Errorf("no Socratic Coach provider available (local Ollama is offline and no cloud API key found)")
	}

	reqCopy := req
	m.mu.RLock()
	opts := m.generation
	m.mu.RUnlock()

	if !opts.IncludeCode {
		reqCopy.UserCode = ""
	}
	reqCopy.Temperature = opts.Temperature
	reqCopy.TemperatureSet = true
	reqCopy.MaxTokens = opts.MaxTokens

	streamCtx, streamCancel := context.WithCancel(ctx)

	m.activeMu.Lock()
	if m.activeCancel != nil {
		m.activeCancel()
	}
	m.streamSeq++
	m.activeCancel = streamCancel
	m.activeID = m.streamSeq
	streamID := m.streamSeq
	m.activeMu.Unlock()

	channels := NewStreamChannels()
	providerName := provider.Name()

	go func() {
		defer streamCancel()
		// The manager owns channel closure, not the provider. Providers only
		// send. This ordering guarantees the error is queued before Tokens
		// closes, so the consumer can never observe a closed stream and miss
		// the reason it failed.
		defer channels.Close()
		defer close(channels.Tokens)

		err := provider.Stream(streamCtx, reqCopy, channels)
		if err != nil {
			select {
			case channels.Errors <- err:
			default:
			}
		}

		m.activeMu.Lock()
		if m.activeID == streamID {
			m.activeCancel = nil
			m.activeID = 0
		}
		m.activeMu.Unlock()
	}()

	return channels, providerName, streamID, nil
}

// CancelActiveStream cancels any in-flight streaming context.
func (m *Manager) CancelActiveStream() {
	m.activeMu.Lock()
	defer m.activeMu.Unlock()
	if m.activeCancel != nil {
		m.activeCancel()
		m.activeCancel = nil
		m.activeID = 0
	}
}

func (m *Manager) selectProvider(ctx context.Context) CoachProvider {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 1. Try local Ollama first
	if m.localProvider != nil {
		if err := m.localProvider.HealthCheck(ctx); err == nil {
			return m.localProvider
		}
	}

	// 2. Seamlessly fallback to cloud provider
	if m.cloudProvider != nil {
		if err := m.cloudProvider.HealthCheck(ctx); err == nil {
			return m.cloudProvider
		}
	}

	return nil
}

// WaitForNextToken returns a tea.Cmd that listens for the next token or termination.
func WaitForNextToken(channels StreamChannels) tea.Cmd {
	return func() tea.Msg {
		select {
		case token, ok := <-channels.Tokens:
			if !ok {
				return CoachStreamDoneMsg{}
			}
			return CoachTokenMsg{Token: token, Channels: channels}

		case err := <-channels.Errors:
			if err != nil {
				return CoachStreamErrorMsg{Err: err}
			}
			return CoachStreamDoneMsg{}

		case <-channels.Done:
			// Drain a token that was buffered before the stream closed; Done is
			// not allowed to overtake the final tokens.
			select {
			case token, ok := <-channels.Tokens:
				if ok {
					return CoachTokenMsg{Token: token, Channels: channels}
				}
			default:
			}
			return CoachStreamDoneMsg{}
		}
	}
}
