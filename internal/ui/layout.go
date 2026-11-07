package ui

// frame is the canvas for a single render pass. It is derived from the root
// model's width/height every time View runs, never stored, so a resize cannot
// leave a component computing against a stale delta.
type frame struct {
	Width  int
	Height int
}

func newFrame(width, height int) frame {
	if width < 0 {
		width = 0
	}
	if height < 0 {
		height = 0
	}
	return frame{Width: width, Height: height}
}

// tooSmall reports whether the terminal cannot host the planned three-column
// workspace without clipping a pane.
func (f frame) tooSmall() bool {
	return f.Width < 100 || f.Height < 30
}

// bodyHeight is the solve screen's content area between the top bar and the
// footer hint row.
func (f frame) bodyHeight() int {
	height := f.Height - 2
	if height < 4 {
		height = 4
	}
	return height
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
