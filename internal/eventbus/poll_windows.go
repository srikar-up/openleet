//go:build windows

package eventbus

// NeedsResizePolling reports whether the platform can deliver resize events on
// its own. Bubble Tea v1 uses SIGWINCH, which Windows does not have, so the
// geometry watcher must run there.
const NeedsResizePolling = true
