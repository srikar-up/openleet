package ui

import (
	"strings"
	"testing"
	"time"

	"openleet/internal/coach"
	"openleet/internal/eventbus"
)

func TestCoachStreamPumpBatchesAndCompletes(t *testing.T) {
	hub := eventbus.NewHub()
	sub := hub.Subscribe(0)
	streams := newCoachStreams()
	streams.attach(hub)
	t.Cleanup(streams.close)

	channels := coach.NewStreamChannels()
	streams.consume(42, channels)

	for _, token := range []string{"Hel", "lo", " ", "wor", "ld"} {
		channels.Tokens <- token
	}
	close(channels.Tokens)

	var text strings.Builder
	done := false
	timeout := time.After(2 * time.Second)

	for !done {
		select {
		case <-sub.Notify():
			for _, event := range sub.Drain() {
				switch msg := event.Msg.(type) {
				case coach.CoachTokenBatchMsg:
					if msg.StreamID != 42 {
						t.Fatalf("batch carried stream %d, want 42", msg.StreamID)
					}
					text.WriteString(strings.Join(msg.Tokens, ""))
				case coach.CoachStreamDoneMsg:
					if msg.StreamID != 42 {
						t.Fatalf("done carried stream %d, want 42", msg.StreamID)
					}
					done = true
				case coach.CoachStreamErrorMsg:
					t.Fatalf("unexpected stream error: %v", msg.Err)
				}
			}
		case <-timeout:
			t.Fatal("timed out waiting for the coach stream to finish")
		}
	}

	if got, want := text.String(), "Hello world"; got != want {
		t.Fatalf("reassembled stream = %q, want %q", got, want)
	}
}

func TestCoachStreamPumpReportsProviderErrors(t *testing.T) {
	hub := eventbus.NewHub()
	sub := hub.Subscribe(0)
	streams := newCoachStreams()
	streams.attach(hub)
	t.Cleanup(streams.close)

	channels := coach.NewStreamChannels()
	streams.consume(3, channels)
	channels.Errors <- errTestStream
	close(channels.Tokens)

	timeout := time.After(2 * time.Second)
	for {
		select {
		case <-sub.Notify():
			for _, event := range sub.Drain() {
				if msg, ok := event.Msg.(coach.CoachStreamErrorMsg); ok {
					if msg.StreamID != 3 || msg.Err != errTestStream {
						t.Fatalf("unexpected error event: %#v", msg)
					}
					return
				}
			}
		case <-timeout:
			t.Fatal("timed out waiting for the provider error")
		}
	}
}

func TestCoachStreamPumpDrainsTokensBeforeDone(t *testing.T) {
	hub := eventbus.NewHub()
	sub := hub.Subscribe(0)
	streams := newCoachStreams()
	streams.attach(hub)
	t.Cleanup(streams.close)

	channels := coach.StreamChannels{
		Tokens: make(chan string, 8),
		Errors: make(chan error, 1),
		Done:   make(chan struct{}),
	}
	channels.Tokens <- "a"
	channels.Tokens <- "b"
	close(channels.Tokens)
	close(channels.Done)

	streams.consume(11, channels)

	var text strings.Builder
	done := false
	timeout := time.After(2 * time.Second)
	for !done {
		select {
		case <-sub.Notify():
			for _, event := range sub.Drain() {
				switch msg := event.Msg.(type) {
				case coach.CoachTokenBatchMsg:
					text.WriteString(strings.Join(msg.Tokens, ""))
				case coach.CoachStreamDoneMsg:
					done = true
				}
			}
		case <-timeout:
			t.Fatal("timed out waiting for the buffered tokens")
		}
	}
	if got := text.String(); got != "ab" {
		t.Fatalf("buffered tokens were truncated at Done: got %q", got)
	}
}

func TestCoachStreamsCloseIsIdempotent(t *testing.T) {
	streams := newCoachStreams()
	streams.attach(eventbus.NewHub())
	streams.consume(1, coach.NewStreamChannels())
	streams.close()
	streams.close()

	done := make(chan struct{})
	go func() {
		streams.close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("close blocked after shutdown")
	}
}

func TestCoachStreamsWithoutHubDoNotStart(t *testing.T) {
	streams := newCoachStreams()
	t.Cleanup(streams.close)

	channels := coach.NewStreamChannels()
	streams.consume(1, channels)

	select {
	case channels.Tokens <- "ignored":
		// Buffered channel accepts the token, but no pump is reading it.
	case <-time.After(50 * time.Millisecond):
	}
	if streams.activeCount() != 0 {
		t.Fatal("a stream started without an attached hub")
	}
}

var errTestStream = testStreamError("provider offline")

type testStreamError string

func (e testStreamError) Error() string { return string(e) }
