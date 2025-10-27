package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m model) appControlsView() string {
	body := ""

	if m.usingDefaultCfg {
		body += lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5555")).Render(
			"⚠️ Using default config\n\n")
	}

	plexControls := ""
	if m.plexAuthenticated {
		plexControls = "\n  1 Artists  2 Albums  3 Playlists"
	}

	controlsText := fmt.Sprintf("Controls:\n  ↑/↓ navigate\n  Enter select\n  [p / space] Play/Pause\n  n Next\n  b Previous\n  +/- Volume %s\n  q Quit", plexControls)
	controls := lipgloss.NewStyle().MarginTop(1).Foreground(lipgloss.Color("#8888ff")).Render(controlsText)

	return fmt.Sprintf("%s%s", body, controls)
}
