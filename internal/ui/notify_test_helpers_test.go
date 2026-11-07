package ui

import (
	"strings"

	"openleet/internal/ui/toast"
)

func visibleToasts(app AppModel) []toast.Toast {
	if app.notifier == nil {
		return nil
	}
	return app.notifier.Visible()
}

func hasVisibleToast(app AppModel, kind toast.Kind, fragment string) bool {
	for _, visible := range visibleToasts(app) {
		if visible.Kind != kind {
			continue
		}
		if fragment == "" || strings.Contains(visible.Title, fragment) || strings.Contains(visible.Message, fragment) {
			return true
		}
	}
	return false
}
