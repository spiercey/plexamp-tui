package ui

import (
	"fmt"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

func (m model) playbackStatusView() string {
	info := lipgloss.NewStyle().Foreground(lipgloss.Color("#aaaaaa"))
	value := lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffcc")).Bold(true)

	state := "⏸️ Paused"
	if m.isPlaying {
		state = "▶️ Playing"
	}

	current := "None"
	if m.currentTrack != "" {
		current = m.currentTrack
	}

	elapsed := m.currentPosition()
	progress := formatTime(elapsed) + " / " + formatTime(m.durationMs)
	bar := progressBar(elapsed, m.durationMs, 20)

	body := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffaa00")).Render("Now Playing") + "\n\n"
	body += fmt.Sprintf(
		"%s: %s\n%s: %s\n%s: %s\n%s: %d\n",
		info.Render("State"), value.Render(state),
		info.Render("Track"), value.Render(current),
		info.Render("Progress"), value.Render(bar+"  "+progress),
		info.Render("Volume"), m.volume,
	)

	return body
}

// =====================
// Playback Control Methods
// =====================

// togglePlayback toggles between play and pause
func (m *model) togglePlayback() tea.Cmd {
	if m.isPlaying {
		m.sendCommand("playback/pause")
		m.isPlaying = false
		m.lastCommand = "Pause"
	} else {
		m.sendCommand("playback/play")
		m.isPlaying = true
		m.lastCommand = "Play"
	}
	return m.pollTimeline()
}

// nextTrack skips to the next track
func (m *model) nextTrack() tea.Cmd {
	m.sendCommand("playback/skipNext")
	m.lastCommand = "Next"
	return m.pollTimeline()
}

// previousTrack goes to the previous track
func (m *model) previousTrack() tea.Cmd {
	m.sendCommand("playback/skipPrevious")
	m.lastCommand = "Previous"
	return m.pollTimeline()
}

// adjustVolume changes the volume by the specified delta (range: -100 to +100)
func (m *model) adjustVolume(delta int) tea.Cmd {
	newVol := m.volume + delta
	if newVol < 0 {
		newVol = 0
	} else if newVol > 100 {
		newVol = 100
	}

	// Use setVolume to handle the actual volume change
	m.setVolume(newVol)

	// Update the status message
	m.lastCommand = fmt.Sprintf("Volume %d%%", newVol)

	// Return a command to update the timeline
	return m.pollTimeline()
}

// seek seeks the current track by the specified number of seconds
func (m *model) seek(seconds int) tea.Cmd {
	// Calculate the new position in milliseconds
	newPos := m.positionMs + (seconds * 1000)

	// Ensure the position is within bounds
	if newPos < 0 {
		newPos = 0
	} else if m.durationMs > 0 && newPos > m.durationMs {
		newPos = m.durationMs
	}

	// Send the seek command with absolute position
	m.sendCommand(fmt.Sprintf("playback/seekTo?time=%d", newPos))
	m.lastCommand = fmt.Sprintf("Seek to %s", formatTime(newPos))

	// Update the position immediately for better UX
	m.positionMs = newPos
	m.lastUpdate = time.Now()

	return m.pollTimeline()
}

// toggleShuffle toggles shuffle mode
func (m *model) toggleShuffle() tea.Cmd {
	m.shuffle = !m.shuffle
	if m.shuffle {
		m.sendCommand("playback/shuffle/on")
		m.lastCommand = "Shuffle ON"
	} else {
		m.sendCommand("playback/shuffle/off")
		m.lastCommand = "Shuffle OFF"
	}
	return nil
}

// will use the config to cycle through the library options, it will check the current selected library and increment to the next one, if it is the last one it will go back to the first one
func (m *model) cycleLibrary() tea.Cmd {
	currentLibraryKey := m.config.PlexLibraryID

	for i := range m.config.PlexLibraries {
		if m.config.PlexLibraries[i].Key == currentLibraryKey {
			if i == len(m.config.PlexLibraries)-1 {
				m.config.PlexLibraryID = m.config.PlexLibraries[0].Key
				m.config.PlexLibraryName = m.config.PlexLibraries[0].Title
			} else {
				m.config.PlexLibraryID = m.config.PlexLibraries[i+1].Key
				m.config.PlexLibraryName = m.config.PlexLibraries[i+1].Title
			}
			cfgManager.Save(m.config)
			// Return a command that will refresh the current panel
			return m.refreshCurrentPanel()
		}
	}
	return nil
}
