package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// footerView renders the application footer
func (m model) footerView() string {
	header := lipgloss.NewStyle().Foreground(lipgloss.Color("#ffaa00"))
	value := lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffcc")).Bold(true)
	info := lipgloss.NewStyle().Foreground(lipgloss.Color("#8888ff"))
	footerStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.RoundedBorder()).
		BorderTop(true).
		BorderForeground(lipgloss.Color("#00ffff")).
		Padding(0, 1)

	var shuffleValue string
	if m.shuffle {
		shuffleValue = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Bold(true).Render("ON")
	} else {
		shuffleValue = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5555")).Bold(true).Render("OFF")
	}
	// --- Left side (your existing info)
	left := ""
	left += fmt.Sprintf("%s %s: %s \n", header.Render("Shuffle"), info.Render("(h)"), shuffleValue)
	if len(m.config.PlexLibraries) > 0 {
		left += fmt.Sprintf("%s %s: ", header.Render("Library"), info.Render("(Tab)"))
		for _, library := range m.config.PlexLibraries {
			if library.Key == m.config.PlexLibraryID {
				left += fmt.Sprintf("%s | ", value.Render(library.Title))
			} else {
				left += fmt.Sprintf("%s | ", library.Title)
			}
		}
		left = strings.TrimSuffix(left, "| ")
		left += "\n"
	}

	left += fmt.Sprintf("%s %s: %s | ", header.Render("Server"), info.Render("(6)"), value.Render(m.config.PlexServerName))
	left += fmt.Sprintf("%s %s: %s", header.Render("Player"), info.Render("(7)"), value.Render(m.config.SelectedPlayerName))

	// --- Right side (new)
	// Example: replace with whatever info you want (track, status, etc.)
	var right string
	if m.plexAuthenticated {
		right = fmt.Sprintf("%s: %s ", header.Render("Authenticated"), value.Render("✓"))
	} else {
		right = fmt.Sprintf("%s: %s ", header.Render("Authenticated"), value.Render("✗"))
	}

	right += fmt.Sprintf("\n%s: %s ", header.Render("Last Command"), value.Render(m.lastCommand))

	// --- Combine left and right
	leftLines := strings.Split(left, "\n")
	rightLines := strings.Split(right, "\n")

	var combinedLines []string
	maxLines := max(len(leftLines), len(rightLines))

	for i := 0; i < maxLines; i++ {
		var l, r string
		if i < len(leftLines) {
			l = leftLines[i]
		}
		if i < len(rightLines) {
			r = rightLines[i]
		}

		padding := m.width - lipgloss.Width(l) - lipgloss.Width(r) - 4 // adjust for borders/padding
		if padding < 1 {
			padding = 1
		}
		line := l + strings.Repeat(" ", padding) + r
		combinedLines = append(combinedLines, line)
	}

	return footerStyle.Width(m.width - 2).Render(strings.Join(combinedLines, "\n"))
}
