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

// playIntentGrace bounds how long the optimistic playing state survives without
// confirmation from the player.
const playIntentGrace = 8 * time.Second

// markPlaybackStarting reports playback as active right away. Players need a few
// seconds to build the play queue and keep reporting state="paused" until then,
// during which a play/pause press would otherwise be sent as another play.
func (m *model) markPlaybackStarting() {
	m.isPlaying = true
	m.playIntentUntil = time.Now().Add(playIntentGrace)
}

// clearPlaybackIntent hands authority over the playing state back to the player.
func (m *model) clearPlaybackIntent() {
	m.playIntentUntil = time.Time{}
}

// togglePlayback toggles between play and pause
func (m *model) togglePlayback() tea.Cmd {
	// An explicit press always wins over an in-flight optimistic state.
	m.clearPlaybackIntent()

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
	if len(m.config.PlexLibraries) == 0 {
		return nil
	}

	// Default to the first library so a key that isn't in the list (left over from a
	// previously selected server) can't leave cycling permanently stuck.
	next := 0
	for i := range m.config.PlexLibraries {
		if m.config.PlexLibraries[i].Key == m.config.PlexLibraryID {
			next = (i + 1) % len(m.config.PlexLibraries)
			break
		}
	}

	m.config.PlexLibraryID = m.config.PlexLibraries[next].Key
	m.config.PlexLibraryName = m.config.PlexLibraries[next].Title
	cfgManager.Save(m.config)

	// Return a command that will refresh the current panel
	return m.refreshCurrentPanel()
}
