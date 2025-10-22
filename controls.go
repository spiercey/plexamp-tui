package main

import tea "github.com/charmbracelet/bubbletea"

// handleControl processes common playback control key presses
// Returns the command to execute and a boolean indicating if a control was handled
// refreshCurrentPanel returns a command that refreshes the current panel based on the panel mode
func (m *model) refreshCurrentPanel() tea.Cmd {
	switch m.panelMode {
	case "plex-artists":
		return m.fetchArtistsCmd()
	case "plex-albums":
		return m.fetchAlbumsCmd()
	case "plex-playlists":
		return m.fetchPlaylistsCmd()
	default:
		return nil
	}
}

// handleControl processes common playback control key presses
// Returns the command to execute and a boolean indicating if a control was handled
func (m *model) handleControl(key string) (tea.Cmd, bool) {
	switch key {
	case " ", "p": // Space or 'p' for play/pause
		return m.togglePlayback(), true

	case "n": // Next track
		return m.nextTrack(), true

	case "b": // Previous track
		return m.previousTrack(), true

	case "+", "]": // Volume up
		return m.adjustVolume(5), true

	case "-", "[": // Volume down
		return m.adjustVolume(-5), true

	case "h": // Toggle shuffle
		return m.toggleShuffle(), true

	case "tab": // Cycle library
		return m.cycleLibrary(), true
		
	case "r": // Refresh current panel
		return m.refreshCurrentPanel(), true

	default:
		return nil, false
	}
}
