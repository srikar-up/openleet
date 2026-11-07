package storage

// WorkspaceLayout represents the persistent configuration of workspace panes
// across application sessions.
type WorkspaceLayout struct {
	DescriptionWidth  int  `json:"description_width"`
	SidebarWidth      int  `json:"sidebar_width"`
	DescriptionHidden bool `json:"description_hidden"`
	SidebarHidden     bool `json:"sidebar_hidden"`
}
