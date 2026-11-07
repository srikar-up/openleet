package ui

import (
	"context"
	"sync"
	"time"

	"openleet/internal/coach"
	"openleet/internal/eventbus"
)

const (
	// tokenFlushInterval groups provider tokens into readable increments. 16ms
	// is one 60Hz frame; the pump's 33ms cap then decides how often the view is
	// actually recomputed.
	tokenFlushInterval = 16 * time.Millisecond

	// tokenBatchLimit bounds a single batch when a provider dumps a large
	// response without pausing.
	tokenBatchLimit = 256
)

// coachStreams owns the goroutines that drain provider token channels. Every
// token is published to the Hub, never written to a model directly: the only
// writer to UI state remains AppModel.Update.
type coachStreams struct {
	ctx    context.Context
	cancel context.CancelFunc

	mu     sync.Mutex
	hub    *eventbus.Hub
	active map[uint64]context.CancelFunc
	closed bool

	wg sync.WaitGroup
}

func newCoachStreams() *coachStreams {
	ctx, cancel := context.WithCancel(context.Background())
	return &coachStreams{
		ctx:    ctx,
		cancel: cancel,
		active: make(map[uint64]context.CancelFunc),
	}
}

// attach points the stream pumps at a bus. Without a bus the console keeps its
// original tea.Cmd-per-token path, which is slower but still correct.
func (s *coachStreams) attach(hub *eventbus.Hub) {
	if hub == nil {
		return
	}
	s.mu.Lock()
	s.hub = hub
	s.mu.Unlock()
}

// consume starts a pump for one provider stream. Reusing an ID cancels the
// previous pump so a retry cannot interleave with a stale response.
func (s *coachStreams) consume(id uint64, channels coach.StreamChannels) {
	s.mu.Lock()
	if s.closed || s.hub == nil {
		s.mu.Unlock()
		return
	}
	if previous, ok := s.active[id]; ok {
		previous()
	}
	ctx, cancel := context.WithCancel(s.ctx)
	hub := s.hub
	s.active[id] = cancel
	s.wg.Add(1)
	s.mu.Unlock()

	go func() {
		defer s.wg.Done()
		defer func() {
			s.mu.Lock()
			delete(s.active, id)
			s.mu.Unlock()
			cancel()
		}()
		pumpCoachStream(ctx, hub, id, channels)
	}()
}

// activeCount reports how many provider pumps are running.
func (s *coachStreams) activeCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.active)
}

// close stops every pump and waits for them to leave their select loops.
func (s *coachStreams) close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	for id, cancel := range s.active {
		cancel()
		delete(s.active, id)
	}
	s.mu.Unlock()

	s.cancel()
	s.wg.Wait()
}

// pumpCoachStream reads provider channels and publishes batches. It never
// touches a model, and it exits on context cancellation so a cancelled request
// cannot leak a goroutine.
func pumpCoachStream(ctx context.Context, hub *eventbus.Hub, id uint64, channels coach.StreamChannels) {
	batch := make([]string, 0, tokenBatchLimit)
	finished := false

	flush := func() {
		if len(batch) == 0 {
			return
		}
		tokens := append([]string(nil), batch...)
		batch = batch[:0]
		hub.Publish(eventbus.KindToken, coach.CoachTokenBatchMsg{StreamID: id, Tokens: tokens})
	}

	finish := func(err error) {
		if finished {
			return
		}
		finished = true
		flush()
		if err != nil {
			hub.Publish(eventbus.KindLifecycle, coach.CoachStreamErrorMsg{Err: err, StreamID: id})
			return
		}
		hub.Publish(eventbus.KindLifecycle, coach.CoachStreamDoneMsg{StreamID: id})
	}

	ticker := time.NewTicker(tokenFlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case token, ok := <-channels.Tokens:
			if !ok {
				finish(pendingStreamError(channels.Errors))
				return
			}
			batch = append(batch, token)
			if len(batch) >= tokenBatchLimit {
				flush()
			}

		case err := <-channels.Errors:
			finish(err)
			return

		case <-channels.Done:
			// Done means the producer will not send more tokens. Tokens that
			// were already buffered are still valid, so drain them before
			// publishing completion. Otherwise a close that races the last
			// tokens truncates the visible transcript.
			for {
				select {
				case token, ok := <-channels.Tokens:
					if !ok {
						finish(pendingStreamError(channels.Errors))
						return
					}
					batch = append(batch, token)
					if len(batch) >= tokenBatchLimit {
						flush()
					}
				case err := <-channels.Errors:
					finish(err)
					return
				default:
					finish(pendingStreamError(channels.Errors))
					return
				}
			}

		case <-ticker.C:
			flush()
		}
	}
}

// pendingStreamError drains a provider error that raced with channel closure.
// Providers close Tokens on return, so the error can still be sitting in the
// buffered Errors channel when the token channel is observed closed.
func pendingStreamError(errors <-chan error) error {
	select {
	case err := <-errors:
		return err
	default:
		return nil
	}
}
