//go:build !windows

package eventbus

// NeedsResizePolling reports whether the platform needs an explicit geometry
// probe. Unix terminals deliver SIGWINCH, which Bubble Tea already forwards, so
// the watcher is optional there.
const NeedsResizePolling = false
