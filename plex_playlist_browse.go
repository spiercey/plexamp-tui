package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type playlistPlaybackMsg struct {
	success bool
	err     error
}

// playlistItem represents a playlist in the list
type playlistItem struct {
	title     string
	artist    string
	year      string
	ratingKey string
}

// playlistsFetchedMsg is a message containing fetched playlists
type playlistsFetchedMsg struct {
	playlists []PlexPlaylist
	err       error
}

// Title returns the playlist title
func (i playlistItem) Title() string {
	return fmt.Sprintf("%s - %s (%s)", i.title, i.artist, i.year)
}

// Description returns the playlist description (empty for now)
func (i playlistItem) Description() string { return "" }

// FilterValue implements list.Item
func (i playlistItem) FilterValue() string {
	return i.title + " " + i.artist
}

// fetchPlaylistsCmd fetches playlists from the Plex server
func (m *model) fetchPlaylistsCmd() tea.Cmd {
	logDebug("Fetching playlists...")
	// ✅ Reapply sizing
	footerHeight := 3 // or dynamically measure your footer
	availableHeight := m.height - footerHeight - 5
	m.playlistList.SetSize(m.width/2-4, availableHeight)
	if m.config == nil {
		return func() tea.Msg {
			return playlistsFetchedMsg{err: fmt.Errorf("no config available")}
		}
	}

	token := getPlexToken()
	if token == "" {
		return func() tea.Msg {
			return playlistsFetchedMsg{err: fmt.Errorf("no Plex token found - run with --auth flag")}
		}
	}

	serverAddr := m.config.PlexServerAddr

	return func() tea.Msg {
		playlists, err := FetchPlaylists(serverAddr, token)
		return playlistsFetchedMsg{playlists: playlists, err: err}
	}
}

// initPlaylistBrowse creates a new playlist browser
func (m *model) initPlaylistBrowse() {
	m.panelMode = "plex-playlists"
	m.status = "Loading playlists..."

	// Create a new default delegate with custom styling
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false

	items := []list.Item{playlistItem{title: "Loading playlists..."}}

	// Create the list with empty items for now
	m.playlistList = list.New(items, delegate, 0, 0)
	m.playlistList.Title = "Plex Playlists"
	m.playlistList.SetShowFilter(true)
	m.playlistList.SetShowStatusBar(false)
	m.playlistList.SetFilteringEnabled(true)
	m.playlistList.Styles.Title = titleStyle
	m.playlistList.Styles.PaginationStyle = paginationStyle
	m.playlistList.Styles.HelpStyle = helpStyle
	if m.width > 0 && m.height > 0 {
		m.playlistList.SetSize(m.width/2-4, m.height-4)
	}
}
func (m *model) playPlaylistCmd(ratingKey string) tea.Cmd {
	if m.selected == "" {
		return func() tea.Msg {
			return playlistPlaybackMsg{success: false, err: fmt.Errorf("no server selected")}
		}
	}

	if m.config == nil {
		return func() tea.Msg {
			return playlistPlaybackMsg{success: false, err: fmt.Errorf("no config available")}
		}
	}

	serverIP := m.selected
	serverID := m.config.ServerID
	shuffle := m.shuffle

	return func() tea.Msg {
		err := PlayPlaylist(serverIP, serverID, ratingKey, shuffle)
		if err != nil {
			return playlistPlaybackMsg{success: false, err: err}
		}
		return playlistPlaybackMsg{success: true}
	}
}

func (m *model) handlePlaylistBrowseUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	logDebug(fmt.Sprintf("handlePlaylistBrowseUpdate received message: %T", msg))

	// If we're in filtering mode, let the list handle the input
	if m.playlistList.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.playlistList, cmd = m.playlistList.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		switch key {
		case "esc", "q":
			// Return to playback panel
			m.panelMode = "playback"
			m.status = ""
			return m, nil

		case "enter":
			// Play selected album's tracks
			if selected, ok := m.playlistList.SelectedItem().(playlistItem); ok {
				logDebug(fmt.Sprintf("Playing playlist: %s (ratingKey: %s)", selected.title, selected.ratingKey))
				m.lastCommand = fmt.Sprintf("Playing %s", selected.title)
				return m, m.playPlaylistCmd(selected.ratingKey)
			}
			return m, nil

		case "R":
			// Refresh album list
			m.status = "Refreshing albums..."
			return m, m.fetchPlaylistsCmd()

		default:

			// Otherwise try the common controls
			if cmd, handled := m.handleControl(key); handled {
				return m, cmd
			}
		}

	case playlistsFetchedMsg:
		logDebug(fmt.Sprintf("playlistsFetchedMsg received with %d playlists, error: %v", len(msg.playlists), msg.err))
		if msg.err != nil {
			errMsg := fmt.Sprintf("Error fetching playlists: %v", msg.err)
			m.status = errMsg
			logDebug(errMsg)
			return m, nil
		}

		// Convert playlists to list items
		var items []list.Item
		for i, playlist := range msg.playlists {
			if i < 5 { // Only log first 5 playlists to avoid log spam
				logDebug(fmt.Sprintf("Adding playlist %d: %s (ratingKey: %s)", i+1, playlist.Title, playlist.RatingKey))
			}
			items = append(items, playlistItem{
				title:     playlist.Title,
				ratingKey: playlist.RatingKey,
			})
		}

		logDebug(fmt.Sprintf("Creating new list with %d items", len(items)))
		// Create a new list with the fetched items
		// Preserve the current filter state
		filterState := m.playlistList.FilterState()
		filterValue := m.playlistList.FilterValue()

		// Create a new default delegate with custom styling
		delegate := list.NewDefaultDelegate()
		delegate.ShowDescription = false // Don't show description

		// Create new list with existing items
		m.playlistList.SetItems(items)
		m.playlistList.ResetSelected()

		// Restore filter state if there was one
		if filterState == list.Filtering {
			m.playlistList.ResetFilter()
			m.playlistList.FilterInput.SetValue(filterValue)
		}
		m.status = fmt.Sprintf("Loaded %d playlists", len(msg.playlists))
		logDebug(fmt.Sprintf("Updated model with new playlist list. List has %d items", m.playlistList.VisibleItems()))

		// Force a redraw
		return m, tea.Batch(tea.ClearScreen, func() tea.Msg { return nil })

	case playlistPlaybackMsg:
		if msg.success {
			m.lastCommand = "Playlist Playback Started"
			m.status = "Playback triggered successfully"
		} else {
			m.lastCommand = "Playback Failed"
			m.status = fmt.Sprintf("Playback error: %v", msg.err)
		}
		// Return the updated model and no command
		return m, nil
	}

	// Update the artist list and get the command
	var listCmd tea.Cmd
	m.playlistList, listCmd = m.playlistList.Update(msg)
	// Return the current model (as a pointer) and the command
	return m, listCmd
}

// View renders the playlist browser
func (m *model) ViewPlaylist() string {
	return m.playlistList.View() + "\n" + m.status
}
