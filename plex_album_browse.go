package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type albumPlaybackMsg struct {
	success bool
	err     error
}

// albumItem represents an album in the list
type albumItem struct {
	title     string
	artist    string
	year      string
	ratingKey string
}

// Title returns the album title
func (i albumItem) Title() string {
	return fmt.Sprintf("%s - %s (%s)", i.title, i.artist, i.year)
}

// Description returns the album description (empty for now)
func (i albumItem) Description() string { return "" }

// FilterValue implements list.Item
func (i albumItem) FilterValue() string {
	return i.title + " " + i.artist
}

// fetchAlbumsCmd fetches albums from the Plex server
func (m *model) fetchAlbumsCmd() tea.Cmd {
	logDebug("Fetching albums...")
	if m.config == nil {
		return func() tea.Msg {
			return albumsFetchedMsg{err: fmt.Errorf("no config available")}
		}
	}

	token := getPlexToken()
	if token == "" {
		return func() tea.Msg {
			return albumsFetchedMsg{err: fmt.Errorf("no Plex token found - run with --auth flag")}
		}
	}

	serverAddr := m.config.PlexServerAddr
	libraryID := m.config.PlexLibraryID

	return func() tea.Msg {
		albums, err := FetchAlbums(serverAddr, libraryID, token)
		return albumsFetchedMsg{albums: albums, err: err}
	}
}

// initAlbumBrowse creates a new album browser
func (m *model) initAlbumBrowse() {
	m.panelMode = "plex-albums"
	m.status = "Loading albums..."

	// Create a new default delegate with custom styling
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false

	items := []list.Item{albumItem{title: "Loading albums..."}}

	// Create the list with empty items for now
	m.albumList = list.New(items, delegate, 0, 0)
	m.albumList.Title = "Plex Albums"
	m.albumList.SetShowFilter(true)
	m.albumList.SetShowStatusBar(false)
	m.albumList.SetFilteringEnabled(true)
	m.albumList.Styles.Title = titleStyle
	m.albumList.Styles.PaginationStyle = paginationStyle
	m.albumList.Styles.HelpStyle = helpStyle
	if m.width > 0 && m.height > 0 {
		m.albumList.SetSize(m.width/2-4, m.height-4)
	}
}
func (m *model) playAlbumCmd(ratingKey string) tea.Cmd {
	if m.selected == "" {
		return func() tea.Msg {
			return albumPlaybackMsg{success: false, err: fmt.Errorf("no server selected")}
		}
	}

	if m.config == nil {
		return func() tea.Msg {
			return albumPlaybackMsg{success: false, err: fmt.Errorf("no config available")}
		}
	}

	serverIP := m.selected
	serverID := m.config.ServerID
	shuffle := m.shuffle

	return func() tea.Msg {
		err := PlayMetadata(serverIP, serverID, ratingKey, shuffle)
		if err != nil {
			return albumPlaybackMsg{success: false, err: err}
		}
		return albumPlaybackMsg{success: true}
	}
}

func (m *model) handleAlbumBrowseUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	logDebug(fmt.Sprintf("handleAlbumBrowseUpdate received message: %T", msg))

	// If we're in filtering mode, let the list handle the input
	if m.albumList.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.albumList, cmd = m.albumList.Update(msg)
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
			if selected, ok := m.albumList.SelectedItem().(albumItem); ok {
				logDebug(fmt.Sprintf("Playing album: %s (ratingKey: %s)", selected.title, selected.ratingKey))
				m.lastCommand = fmt.Sprintf("Playing %s", selected.title)
				return m, m.playAlbumCmd(selected.ratingKey)
			}
			return m, nil

		case "R":
			// Refresh album list
			m.status = "Refreshing albums..."
			return m, m.fetchAlbumsCmd()

		default:

			// Otherwise try the common controls
			if cmd, handled := m.handleControl(key); handled {
				return m, cmd
			}
		}

	case albumsFetchedMsg:
		logDebug(fmt.Sprintf("albumsFetchedMsg received with %d albums, error: %v", len(msg.albums), msg.err))
		if msg.err != nil {
			errMsg := fmt.Sprintf("Error fetching albums: %v", msg.err)
			m.status = errMsg
			logDebug(errMsg)
			return m, nil
		}

		// Convert albums to list items
		var items []list.Item
		for i, album := range msg.albums {
			if i < 5 { // Only log first 5 albums to avoid log spam
				logDebug(fmt.Sprintf("Adding album %d: %s (ratingKey: %s)", i+1, album.Title, album.RatingKey))
			}
			items = append(items, albumItem{
				title:     album.Title,
				artist:    album.ParentTitle,
				year:      album.Year,
				ratingKey: album.RatingKey,
			})
		}

		logDebug(fmt.Sprintf("Creating new list with %d items", len(items)))
		// Create a new list with the fetched items
		// Preserve the current filter state
		filterState := m.albumList.FilterState()
		filterValue := m.albumList.FilterValue()

		// Create a new default delegate with custom styling
		delegate := list.NewDefaultDelegate()
		delegate.ShowDescription = false // Don't show description

		// Create new list with existing items
		m.albumList.SetItems(items)
		m.albumList.ResetSelected()

		// Restore filter state if there was one
		if filterState == list.Filtering {
			m.albumList.ResetFilter()
			m.albumList.FilterInput.SetValue(filterValue)
		}
		m.status = fmt.Sprintf("Loaded %d albums", len(msg.albums))
		logDebug(fmt.Sprintf("Updated model with new album list. List has %d items", m.albumList.VisibleItems()))

		// Force a redraw
		return m, tea.Batch(tea.ClearScreen, func() tea.Msg { return nil })

	case albumPlaybackMsg:
		if msg.success {
			m.lastCommand = "Album Playback Started"
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
	m.albumList, listCmd = m.albumList.Update(msg)
	// Return the current model (as a pointer) and the command
	return m, listCmd
}

// View renders the album browser
func (m *model) ViewAlbum() string {
	return m.albumList.View() + "\n" + m.status
}

// albumsFetchedMsg is a message containing fetched albums
type albumsFetchedMsg struct {
	albums []PlexAlbum
	err    error
}
