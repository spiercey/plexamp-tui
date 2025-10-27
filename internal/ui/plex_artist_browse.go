package ui

import (
	"fmt"
	"strings"

	"plexamp-tui/internal/plex"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// =====================
// Artist Browse Messages
// =====================

type artistsFetchedMsg struct {
	artists []plex.PlexArtist
	err     error
}

type artistPlaybackMsg struct {
	success bool
	err     error
}

// =====================
// Artist Browse Functions
// =====================

// fetchArtistsCmd fetches artists from the Plex server
func (m *model) fetchArtistsCmd() tea.Cmd {
	log.Debug("Fetching artists...")
	// ✅ Reapply sizing
	footerHeight := 3 // or dynamically measure your footer
	availableHeight := m.height - footerHeight - 5
	m.artistList.SetSize(m.width/2-4, availableHeight)
	if m.config == nil {
		return func() tea.Msg {
			return artistsFetchedMsg{err: fmt.Errorf("no config available")}
		}
	}

	token := plexClient.GetPlexToken()
	if token == "" {
		return func() tea.Msg {
			return artistsFetchedMsg{err: fmt.Errorf("no Plex token found - run with --auth flag")}
		}
	}

	serverAddr := m.config.PlexServerAddr
	libraryID := m.config.PlexLibraryID

	return func() tea.Msg {
		artists, err := plexClient.FetchArtists(serverAddr, libraryID, token)
		return artistsFetchedMsg{artists: artists, err: err}
	}
}

// playArtistCmd starts playback for an artist (using artist's tracks)
func (m *model) playArtistCmd(ratingKey string) tea.Cmd {
	if m.selected == "" {
		return func() tea.Msg {
			return artistPlaybackMsg{success: false, err: fmt.Errorf("no server selected")}
		}
	}

	if m.config == nil {
		return func() tea.Msg {
			return artistPlaybackMsg{success: false, err: fmt.Errorf("no config available")}
		}
	}

	serverIP := m.selected
	serverID := m.config.ServerID
	shuffle := m.shuffle

	return func() tea.Msg {
		err := PlayMetadata(serverIP, serverID, ratingKey, shuffle)
		if err != nil {
			return artistPlaybackMsg{success: false, err: err}
		}
		return artistPlaybackMsg{success: true}
	}
}

// playArtistRadioCmd starts playback for an artist's radio station
func (m *model) playArtistRadioCmd(ratingKey string) tea.Cmd {
	if m.selected == "" {
		return func() tea.Msg {
			return artistPlaybackMsg{success: false, err: fmt.Errorf("no server selected")}
		}
	}

	if m.config == nil {
		return func() tea.Msg {
			return artistPlaybackMsg{success: false, err: fmt.Errorf("no config available")}
		}
	}

	serverIP := m.selected
	serverID := m.config.ServerID
	shuffle := m.shuffle

	return func() tea.Msg {
		err := PlayArtistRadio(serverIP, serverID, ratingKey, shuffle)
		if err != nil {
			return artistPlaybackMsg{success: false, err: err}
		}
		return artistPlaybackMsg{success: true}
	}
}

// initArtistBrowse initializes the artist browse panel
func (m *model) initArtistBrowse() {
	log.Debug("Initializing artist browse")
	m.panelMode = "plex-artists"
	m.status = "Loading artists..."
	// Log the current model state
	log.Debug(fmt.Sprintf("initArtistBrowse - panelMode: %s, status: %s", m.panelMode, m.status))

	items := []list.Item{artistItem{title: "Loading artists..."}}
	// Create a new default delegate with custom styling
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false // Don't show description

	m.artistList = list.New(items, delegate, 0, 0)
	m.artistList.Title = "Plex Artists"
	m.artistList.SetShowFilter(true)
	m.artistList.SetFilteringEnabled(true)
	m.artistList.Styles.Title = titleStyle
	m.artistList.Styles.PaginationStyle = paginationStyle
	m.artistList.Styles.HelpStyle = helpStyle
	m.artistList.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("f"),
				key.WithHelp("f", "favs"),
			),
		}
	}
	m.artistList.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("f"),
				key.WithHelp("f", "Add/Remove from Favorites"),
			),
			key.NewBinding(
				key.WithKeys("r"),
				key.WithHelp("r", "Play Radio"),
			),
			key.NewBinding(
				key.WithKeys("R"),
				key.WithHelp("R", "Refresh Artists"),
			),
		}
	}

	if m.width > 0 && m.height > 0 {
		m.artistList.SetSize(m.width/2-4, m.height-4)
	}
	log.Debug(fmt.Sprintf("Initialized artist list with size: %dx%d", m.width/2-4, m.height-4))
}

// handleArtistBrowseUpdate handles updates when in artist browse mode
// It updates the model in place and returns the updated model and a command
func (m *model) handleArtistBrowseUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	log.Debug(fmt.Sprintf("handleArtistBrowseUpdate received message: %T", msg))

	// If we're in filtering mode, let the list handle the input
	if m.artistList.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.artistList, cmd = m.artistList.Update(msg)
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
			// Play selected artist's tracks
			if selected, ok := m.artistList.SelectedItem().(artistItem); ok {
				log.Debug(fmt.Sprintf("Playing artist: %s (ratingKey: %s)", selected.title, selected.ratingKey))
				m.lastCommand = fmt.Sprintf("Playing %s", selected.title)
				return m, m.playArtistCmd(selected.ratingKey)
			}
			return m, nil

		case "f":
			// add or remove selected artist from favorites (playback list)
			if selected, ok := m.artistList.SelectedItem().(artistItem); ok {
				log.Debug(fmt.Sprintf("Toggling favorite for artist: %s (ratingKey: %s)", selected.title, selected.ratingKey))
				m.lastCommand = fmt.Sprintf("Toggling favorite for %s", selected.title)
				_, cmd := m.addRemoveFavorite(selected.title, selected.ratingKey, "artist")
				selected.ToggleFavorite()
				// Update the item in the list
				m.artistList.SetItem(m.artistList.Index(), selected)
				return m, cmd
			}

		case "r": // Shift+R for artist radio
			// Play selected artist's radio station
			if selected, ok := m.artistList.SelectedItem().(artistItem); ok {
				log.Debug(fmt.Sprintf("Playing artist radio: %s (ratingKey: %s)", selected.title, selected.ratingKey))
				m.lastCommand = fmt.Sprintf("Playing %s Radio", selected.title)
				return m, m.playArtistRadioCmd(selected.ratingKey)
			}
			return m, nil

		case "R":
			// Refresh artist list
			m.status = "Refreshing artists..."
			return m, m.fetchArtistsCmd()

		default:

			// Otherwise try the common controls
			if cmd, handled := m.handleControl(key); handled {
				return m, cmd
			}
		}

	case artistsFetchedMsg:
		log.Debug(fmt.Sprintf("artistsFetchedMsg received with %d artists, error: %v", len(msg.artists), msg.err))
		if msg.err != nil {
			errMsg := fmt.Sprintf("Error fetching artists: %v", msg.err)
			m.status = errMsg
			log.Debug(errMsg)
			return m, nil
		}

		favSet := make(map[string]struct{})
		for _, pItem := range m.playbackList.Items() {
			pItem := pItem.(item)
			favSet[pItem.GetMetadataKey()] = struct{}{}
		}

		// Convert artists to list items
		var items []list.Item
		for i, artist := range msg.artists {
			if i < 5 { // Only log first 5 artists to avoid log spam
				log.Debug(fmt.Sprintf("Adding artist %d: %s (ratingKey: %s)", i+1, artist.Title, artist.RatingKey))
			}

			fav := false
			if _, exists := favSet[artist.RatingKey]; exists {
				fav = true
			}
			title := artist.Title
			if fav {
				title = fmt.Sprintf("%s ★", artist.Title)
			}
			items = append(items, artistItem{
				title:     title,
				ratingKey: artist.RatingKey,
			})
		}

		log.Debug(fmt.Sprintf("Creating new list with %d items", len(items)))
		// Create a new list with the fetched items
		// Preserve the current filter state
		filterState := m.artistList.FilterState()
		filterValue := m.artistList.FilterValue()

		// Create a new default delegate with custom styling
		delegate := list.NewDefaultDelegate()
		delegate.ShowDescription = false // Don't show description

		// Create new list with existing items
		m.artistList.SetItems(items)
		m.artistList.ResetSelected()

		// Restore filter state if there was one
		if filterState == list.Filtering {
			m.artistList.ResetFilter()
			m.artistList.FilterInput.SetValue(filterValue)
		}
		m.status = fmt.Sprintf("Loaded %d artists", len(msg.artists))
		log.Debug(fmt.Sprintf("Updated model with new artist list. List has %d items", m.artistList.VisibleItems()))

		// Force a redraw
		return m, tea.Batch(tea.ClearScreen, func() tea.Msg { return nil })

	case artistPlaybackMsg:
		if msg.success {
			m.lastCommand = "Artist Playback Started"
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
	m.artistList, listCmd = m.artistList.Update(msg)
	// Return the current model (as a pointer) and the command
	return m, listCmd
}

// =====================
// Artist Item Type
// =====================

type artistItem struct {
	title     string
	ratingKey string
}

func (i artistItem) Title() string       { return i.title }
func (i artistItem) Description() string { return "" } // No description needed
// FilterValue implements list.Item
func (i artistItem) FilterValue() string {
	// Return the title in lowercase for case-insensitive matching
	return i.title
}

func (a *artistItem) ToggleFavorite() {
	// If title already has a star, remove it
	if strings.HasSuffix(a.title, " ★") {
		a.title = strings.TrimSuffix(a.title, " ★")
	} else {
		a.title = fmt.Sprintf("%s ★", a.title)
	}
}

// Custom styles for the list
var (
	titleStyle      = lipgloss.NewStyle().MarginLeft(2)
	itemStyle       = lipgloss.NewStyle().PaddingLeft(4)
	helpStyle       = lipgloss.NewStyle().Foreground(lipgloss.Color("240")).Margin(1, 0, 0, 2)
	paginationStyle = list.DefaultStyles().PaginationStyle.PaddingLeft(4)
)
