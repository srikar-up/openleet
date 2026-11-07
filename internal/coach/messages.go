package coach

// CoachStreamStartedMsg is dispatched when a provider begins streaming tokens.
type CoachStreamStartedMsg struct {
	Provider string
	Action   Action
	Channels StreamChannels
	StreamID uint64
}

// CoachTokenMsg carries an incremental token from the stream. It is the
// single-token delivery path used when the event bus is not attached.
type CoachTokenMsg struct {
	Token    string
	Channels StreamChannels
	StreamID uint64
}

// CoachTokenBatchMsg carries every token that arrived inside one frame window.
// The event pump emits it so a dense stream costs one Update and one View per
// frame instead of one per token.
type CoachTokenBatchMsg struct {
	StreamID uint64
	Tokens   []string
}

// CoachStreamDoneMsg indicates that streaming has completed.
type CoachStreamDoneMsg struct {
	FullResponse string
	StreamID     uint64
}

// CoachStreamErrorMsg signals a streaming failure.
type CoachStreamErrorMsg struct {
	Err      error
	Provider string
	Fallback bool
	StreamID uint64
}
