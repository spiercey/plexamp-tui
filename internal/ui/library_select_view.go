package ui

import (
	"fmt"

	"github.com/charmbracelet/lipgloss"
)

func (m model) libraryControlsView() string {
	value := lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffcc")).Bold(true)

	body := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffaa00")).Render("Library Selection") + "\n\n"

	for _, library := range m.config.PlexLibraries {
		if library.Key == m.config.PlexLibraryID {
			body += fmt.Sprintf("%s\n", value.Render(library.Title))
		} else {
			body += fmt.Sprintf("%s\n", library.Title)
		}
	}

	return body
}
