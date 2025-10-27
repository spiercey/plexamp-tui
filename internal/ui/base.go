// Package ui contains the main TUI model and Bubble Tea implementation for Plexamp control.
package ui

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"time"

	"plexamp-tui/internal/config"
	"plexamp-tui/internal/logger"
	"plexamp-tui/internal/plex"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// =====================
// TUI Types
// =====================

type item struct {
	Name        string
	Type        string
	MetadataKey string
}

func (i item) Title() string          { return string(i.Name) }
func (i item) Description() string    { return string(i.Type) }
func (i item) FilterValue() string    { return string(i.Name) }
func (i item) GetMetadataKey() string { return i.MetadataKey }

type model struct {
	playbackList      list.Model
	artistList        list.Model // Plex artist browse list
	albumList         list.Model // Plex album browse list
	playlistList      list.Model // Plex playlist browse list
	serverList        list.Model // Plex server browse list
	playerList        list.Model // Plex player browse list
	selected          string
	status            string
	width             int
	height            int
	isPlaying         bool
	lastCommand       string
	currentTrack      string
	volume            int
	durationMs        int
	positionMs        int
	lastUpdate        time.Time
	usingDefaultCfg   bool
	shuffle           bool // Tracks shuffle state
	plexAuthenticated bool // Plex authentication status
	timelineRequestID int

	// Panel mode: "servers", "playback", "edit", "plex-servers", "plex-libraries", "plex-artists", "plex-albums"
	panelMode      string
	playbackConfig *config.Favorites
	config         *config.Config // Store config for server ID access

	// Edit mode fields
	editMode       string // "server" or "playback"
	editIndex      int    // Index of item being edited
	editInputs     []textinput.Model
	typeSelect     list.Model // Dropdown for type selection
	editFocusIndex int
}

type MediaContainer struct {
	Timelines []Timeline `xml:"Timeline"`
}

type Timeline struct {
	Type     string `xml:"type,attr"`
	State    string `xml:"state,attr"`
	Time     int    `xml:"time,attr"`
	Duration int    `xml:"duration,attr"`
	Volume   int    `xml:"volume,attr"`
	Track    Track  `xml:"Track"`
}

type Track struct {
	Title            string `xml:"title,attr"`
	ParentTitle      string `xml:"parentTitle,attr"`
	GrandparentTitle string `xml:"grandparentTitle,attr"`
}

type (
	trackMsg string
	errMsg   struct{ err error }
	pollMsg  struct{}
)

type trackMsgWithState struct {
	TrackText string
	IsPlaying bool
	Duration  int
	Position  int
	Volume    int
	RequestID int
}

type playbackTriggeredMsg struct {
	success bool
	err     error
}

type UiManager struct {
	Model model
}

var (
	cfg         *config.Config
	favs        *config.Favorites
	plexClient  *plex.PlexClient
	cfgManager  *config.Manager
	log         *logger.Logger
	favsManager *config.FavoritesManager
)

func NewUiManager(logger *logger.Logger, config *config.Config, manager *config.Manager,
	favorites *config.Favorites, client *plex.PlexClient, favoritesMgr *config.FavoritesManager,
) *UiManager {
	log = logger
	cfg = config
	cfgManager = manager
	favs = favorites
	plexClient = client
	favsManager = favoritesMgr

	// Create playback list
	var playbackItems []list.Item
	if favs != nil {
		for _, pb := range favs.Items {
			playbackItems = append(playbackItems, item{Name: pb.Name, Type: pb.Type, MetadataKey: pb.MetadataKey})
		}
	}
	playbackList := list.New(playbackItems, list.NewDefaultDelegate(), 0, 0)
	playbackList.Title = "Favorites"
	// Add keys to the short help (shown at the bottom of the list)
	playbackList.AdditionalShortHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("a"),
				key.WithHelp("a", "add"),
			),
			key.NewBinding(
				key.WithKeys("e"),
				key.WithHelp("e", "edit"),
			),
			key.NewBinding(
				key.WithKeys("d"),
				key.WithHelp("d", "delete"),
			),
		}
	}

	// Add keys to the full help (shown when pressing '?')
	playbackList.AdditionalFullHelpKeys = func() []key.Binding {
		return []key.Binding{
			key.NewBinding(
				key.WithKeys("a"),
				key.WithHelp("a", "Add new item"),
			),
			key.NewBinding(
				key.WithKeys("e"),
				key.WithHelp("e", "Edit selected item"),
			),
			key.NewBinding(
				key.WithKeys("d"),
				key.WithHelp("d", "Delete selected item"),
			),
		}
	}

	m := model{
		playbackList:      playbackList,
		artistList:        list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
		albumList:         list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
		playlistList:      list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
		serverList:        list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
		playerList:        list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
		selected:          cfg.SelectedPlayer,
		usingDefaultCfg:   cfgManager.UsingDefault,
		playbackConfig:    favs,
		config:            cfg,
		panelMode:         "playback",
		shuffle:           true, // Default shuffle to ON
		plexAuthenticated: plexClient.VerifyPlexAuthentication(),
	}

	return &UiManager{
		Model: m,
	}
}

// =====================
// Bubble Tea Methods
// =====================

func (m model) Init() tea.Cmd {
	return tea.Batch(m.pollTimeline(), tick())
}

func tick() tea.Cmd {
	return tea.Tick(time.Second*2, func(time.Time) tea.Msg {
		return pollMsg{}
	})
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case playerSelectMsg:
		if msg.err != nil {
			m.status = "Error selecting player: " + msg.err.Error()
			return m, nil
		}
		if msg.success {
			m.config.SelectedPlayer = msg.player.address
			m.config.SelectedPlayerName = msg.player.title
			m.selected = msg.player.address
			cfgManager.Save(m.config)
			m.lastCommand = "Player Selected"
			m.status = ""
			m.panelMode = "playback" // Return to playback view after selection
		}
		return m, nil

	case serverSelectMsg:
		if msg.err != nil {
			m.status = "Error selecting server: " + msg.err.Error()
			return m, nil
		}
		if msg.success {
			m.config.ServerID = msg.server.clientIdentifier
			m.config.PlexServerAddr = msg.server.address + ":" + msg.server.port
			m.config.PlexServerName = msg.server.title
			m.config.PlexLibraries = msg.libraries

			found := false
			if len(msg.libraries) == 0 {
				log.Debug("No libraries found on this server")
				m.panelMode = "playback"
				m.lastCommand = "Server Selected Failed, No Libraries"
				m.status = "No libraries found on this server"
				return m, nil
			}

			// check if new library list has our configured library
			for _, lib := range msg.libraries {
				if lib.Title == m.config.PlexLibraryName {
					found = true
					break
				}
			}

			if !found {
				log.Debug("Current Library not found on this server, using first library")
				m.config.PlexLibraryName = msg.libraries[0].Title
				m.config.PlexLibraryID = msg.libraries[0].Key
			}

			log.Debug(fmt.Sprintf("Saving server config: %v", m.config))
			cfgManager.Save(m.config)
			m.lastCommand = "Server Selected"
			m.status = ""
			m.panelMode = "playback" // Return to playback view after selection
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

		// Reserve a few lines for the footer (and maybe the title)
		footerHeight := 3 // adjust if your footer grows taller
		titleHeight := 3
		availableHeight := msg.Height - footerHeight - titleHeight - 2

		m.playbackList.SetSize(msg.Width/2-4, availableHeight)
		m.artistList.SetSize(msg.Width/2-4, availableHeight)
		m.albumList.SetSize(msg.Width/2-4, availableHeight)
		m.playlistList.SetSize(msg.Width/2-4, availableHeight)
		m.serverList.SetSize(msg.Width/2-4, availableHeight)
		m.playerList.SetSize(msg.Width/2-4, availableHeight)

		return m, nil

	case tea.KeyMsg:
		// Handle edit mode separately
		if m.panelMode == "edit" {
			return m.handleEditUpdate(msg)
		}

		// Handle artist browse mode
		if m.panelMode == "plex-artists" {
			// Create a pointer to the current model
			modelPtr := &m
			// Call handleArtistBrowseUpdate which will modify the model directly
			updatedModel, cmd := modelPtr.handleArtistBrowseUpdate(msg)
			// The updated model might be a different instance, so we need to update our local copy
			if updatedModel != nil {
				if m2, ok := updatedModel.(model); ok {
					m = m2
				}
			}
			return m, cmd
		}

		// Handle album browse mode
		if m.panelMode == "plex-albums" {
			// Create a pointer to the current model
			modelPtr := &m
			// Call handleAlbumBrowseUpdate which will modify the model directly
			updatedModel, cmd := modelPtr.handleAlbumBrowseUpdate(msg)
			// The updated model might be a different instance, so we need to update our local copy
			if updatedModel != nil {
				if m2, ok := updatedModel.(model); ok {
					m = m2
				}
			}
			return m, cmd
		}

		// Handle playlist browse mode
		if m.panelMode == "plex-playlists" {
			// Create a pointer to the current model
			modelPtr := &m
			// Call handlePlaylistBrowseUpdate which will modify the model directly
			updatedModel, cmd := modelPtr.handlePlaylistBrowseUpdate(msg)
			// The updated model might be a different instance, so we need to update our local copy
			if updatedModel != nil {
				if m2, ok := updatedModel.(model); ok {
					m = m2
				}
			}
			return m, cmd
		}

		// Handle server browse mode
		if m.panelMode == "plex-servers" {
			// Create a pointer to the current model
			modelPtr := &m
			// Call handleServerBrowseUpdate which will modify the model directly
			updatedModel, cmd := modelPtr.handleServerBrowseUpdate(msg)
			// The updated model might be a different instance, so we need to update our local copy
			if updatedModel != nil {
				if m2, ok := updatedModel.(model); ok {
					m = m2
				}
			}
			return m, cmd
		}

		// Handle player browse mode
		if m.panelMode == "plex-players" {
			// Create a pointer to the current model
			modelPtr := &m
			// Call handlePlayerBrowseUpdate which will modify the model directly
			updatedModel, cmd := modelPtr.handlePlayerBrowseUpdate(msg)
			// The updated model might be a different instance, so we need to update our local copy
			if updatedModel != nil {
				if m2, ok := updatedModel.(model); ok {
					m = m2
				}
			}
			return m, cmd
		}

		// Handle playback selection (when in playback/favorites mode)
		if m.panelMode == "playback" {
			// Check if we're in filtering mode for the playback list
			if m.playbackList.FilterState() == list.Filtering {
				var cmd tea.Cmd
				m.playbackList, cmd = m.playbackList.Update(msg)
				return m, cmd
			}

			switch msg.String() {
			case "a":
				// Add new playback item
				m.initEditMode("playback", -1)
				return m, nil

			case "e":
				// Edit selected playback item
				index := m.playbackList.Index()
				m.initEditMode("playback", index)
				return m, nil

			case "d":
				// Delete selected playback item
				index := m.playbackList.Index()
				m.deletePlaybackItem(index)
				return m, nil

			case "r":
				// play station/radio if selection is an artist
				if selected, ok := m.playbackList.SelectedItem().(item); ok {
					for _, pb := range m.playbackConfig.Items {
						if pb.Name == string(selected.Name) && pb.Type == "artist" {
							return m, m.triggerFavoriteRadioPlayback(pb)
						}
					}
				}

			case "enter":
				// Select playback item - don't switch back to servers
				if selected, ok := m.playbackList.SelectedItem().(item); ok {
					// Find the matching playback config item
					for _, pb := range m.playbackConfig.Items {
						if pb.Name == string(selected.Name) {
							return m, m.triggerFavoritePlayback(pb)
						}
					}
				}
				return m, nil

			}
		}

		// Main app key handlers (only processed when popup is NOT open)
		key := msg.String()

		switch key {
		case "ctrl+c", "q":
			return m, tea.Quit

		default:
			// Try the common controls
			if cmd, handled := m.handleControl(key); handled {
				return m, cmd
			}
		}

	case pollMsg:
		return m, tea.Batch(m.pollTimeline(), tick())

	case trackMsgWithState:
		// Discard if this response is stale
		if msg.RequestID != m.timelineRequestID {
			return m, nil
		}
		m.currentTrack = msg.TrackText
		m.isPlaying = msg.IsPlaying
		m.durationMs = msg.Duration
		m.positionMs = msg.Position
		m.volume = msg.Volume
		m.lastUpdate = time.Now()
		return m, nil

	case trackMsg:
		m.currentTrack = string(msg)
		return m, nil

	case errMsg:
		m.status = fmt.Sprintf("Error: %v", msg.err)
		return m, nil

	case playbackTriggeredMsg:
		if msg.success {
			m.lastCommand = "Playback Started"
			m.status = "Playback triggered successfully"
		} else {
			m.lastCommand = "Playback Failed"
			m.status = fmt.Sprintf("Playback error: %v", msg.err)
		}
		return m, nil

	case artistsFetchedMsg:
		// Forward the message to the artist browse handler
		if m.panelMode == "plex-artists" {
			modelPtr := &m
			updatedModel, cmd := modelPtr.handleArtistBrowseUpdate(msg)
			if updatedModel != nil {
				if m2, ok := updatedModel.(model); ok {
					m = m2
				}
			}
			return m, cmd
		}
		return m, nil

	case albumsFetchedMsg:
		// Forward the message to the album browse handler
		if m.panelMode == "plex-albums" {
			modelPtr := &m
			updatedModel, cmd := modelPtr.handleAlbumBrowseUpdate(msg)
			if updatedModel != nil {
				if m2, ok := updatedModel.(model); ok {
					m = m2
				}
			}
			return m, cmd
		}
		return m, nil

	case playlistsFetchedMsg:
		// Forward the message to the playlist browse handler
		if m.panelMode == "plex-playlists" {
			modelPtr := &m
			updatedModel, cmd := modelPtr.handlePlaylistBrowseUpdate(msg)
			if updatedModel != nil {
				if m2, ok := updatedModel.(model); ok {
					m = m2
				}
			}
			return m, cmd
		}
		return m, nil

	case serversFetchedMsg:
		// Forward the message to the server browse handler
		if m.panelMode == "plex-servers" {
			modelPtr := &m
			updatedModel, cmd := modelPtr.handleServerBrowseUpdate(msg)
			if updatedModel != nil {
				if m2, ok := updatedModel.(model); ok {
					m = m2
				}
			}
			return m, cmd
		}
		return m, nil

	case playersFetchedMsg:
		// Forward the message to the player browse handler
		if m.panelMode == "plex-players" {
			modelPtr := &m
			updatedModel, cmd := modelPtr.handlePlayerBrowseUpdate(msg)
			if updatedModel != nil {
				if m2, ok := updatedModel.(model); ok {
					m = m2
				}
			}
			return m, cmd
		}
		return m, nil
	}

	// Update the appropriate list based on panel mode
	var cmd tea.Cmd
	if m.panelMode == "playback" {
		m.playbackList, cmd = m.playbackList.Update(msg)
	} else if m.panelMode == "plex-artists" {
		m.artistList, cmd = m.artistList.Update(msg)
	} else if m.panelMode == "plex-albums" {
		m.albumList, cmd = m.albumList.Update(msg)
	} else if m.panelMode == "plex-playlists" {
		m.playlistList, cmd = m.playlistList.Update(msg)
	} else if m.panelMode == "plex-servers" {
		m.serverList, cmd = m.serverList.Update(msg)
	} else if m.panelMode == "plex-players" {
		m.playerList, cmd = m.playerList.Update(msg)
	}
	return m, cmd
}

func (m model) View() string {
	border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00ffff")).Render("🎧 Plexamp Control")

	// Show edit panel if in edit mode
	if m.panelMode == "edit" {
		editContent := m.editPanelView()
		editPanel := border.Width(m.width - 4).Render(editContent)
		return lipgloss.JoinVertical(lipgloss.Left, title, editPanel)
	}

	// Build left panel content
	var leftPanelContent string
	switch m.panelMode {
	case "playback":
		leftPanelContent = m.playbackList.View()
	case "plex-artists":
		leftPanelContent = m.artistList.View()
	case "plex-albums":
		leftPanelContent = m.albumList.View()
	case "plex-playlists":
		leftPanelContent = m.playlistList.View()
	case "plex-servers":
		leftPanelContent = m.serverList.View()
	case "plex-players":
		leftPanelContent = m.playerList.View()
	}

	// Left panel
	leftPanel := border.Width(m.width/2 - 2).Render(leftPanelContent)

	// Right side has two stacked panels
	playbackPanel := border.Width(m.width/2 - 2).Render(m.playbackStatusView())
	controlsPanel := border.Width(m.width/2 - 2).Render(m.appControlsView())
	rightSide := lipgloss.JoinVertical(lipgloss.Left, playbackPanel, controlsPanel)

	content := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightSide)

	// Combine all elements with the footer at the bottom
	return lipgloss.JoinVertical(lipgloss.Left,
		lipgloss.JoinVertical(lipgloss.Left, title, content),
		"\n"+m.footerView(),
	)
}

// helper
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// =====================
// Plexamp control logic
// =====================

func (m *model) sendCommand(path string) {
	if m.selected == "" {
		m.status = "No Plexamp instance selected"
		return
	}
	url := fmt.Sprintf("http://%s:32500/player/%s", m.selected, path)
	go func() {
		_, err := http.Get(url)
		if err != nil {
			m.status = fmt.Sprintf("Error: %v", err)
		} else {
			m.status = fmt.Sprintf("[%s] Sent %s", m.selected, path)
		}
	}()
	time.Sleep(50 * time.Millisecond)
}

func (m *model) pollTimeline() tea.Cmd {
	if m.selected == "" {
		return nil
	}
	reqID := m.timelineRequestID
	selected := m.selected

	return func() tea.Msg {
		url := fmt.Sprintf("http://%s:32500/player/timeline/poll?wait=1&includeMetadata=1&commandID=1&type=music", selected)
		resp, err := http.Get(url)
		if err != nil {
			return trackMsgWithState{RequestID: reqID, TrackText: "", IsPlaying: false, Duration: 0, Position: 0, Volume: 0}
		}
		defer resp.Body.Close()

		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return trackMsgWithState{RequestID: reqID, TrackText: "", IsPlaying: false, Duration: 0, Position: 0, Volume: 0}
		}

		var mc MediaContainer
		if err := xml.Unmarshal(data, &mc); err != nil {
			return trackMsgWithState{RequestID: reqID, TrackText: "", IsPlaying: false, Duration: 0, Position: 0, Volume: 0}
		}

		var chosen *Timeline
		for i := range mc.Timelines {
			t := &mc.Timelines[i]
			if t.Type == "music" {
				chosen = t
				break
			}
		}
		if chosen == nil && len(mc.Timelines) > 0 {
			chosen = &mc.Timelines[0]
		}

		track := ""
		isPlaying := false
		duration := 0
		position := 0
		volume := 0
		if chosen != nil {
			if chosen.Track.Title != "" {
				track = fmt.Sprintf("%s - %s (%s)", chosen.Track.GrandparentTitle, chosen.Track.Title, chosen.Track.ParentTitle)
			}
			isPlaying = chosen.State == "playing"
			duration = chosen.Duration
			position = chosen.Time
			volume = chosen.Volume
		}

		return trackMsgWithState{
			TrackText: track,
			IsPlaying: isPlaying,
			Duration:  duration,
			Position:  position,
			Volume:    volume,
			RequestID: reqID,
		}
	}
}

// =====================
// Helpers
// =====================

func (m model) currentPosition() int {
	pos := m.positionMs
	if m.isPlaying && !m.lastUpdate.IsZero() {
		pos += int(time.Since(m.lastUpdate).Milliseconds())
	}
	if pos < 0 {
		pos = 0
	}
	if m.durationMs > 0 && pos > m.durationMs {
		pos = m.durationMs
	}
	return pos
}

func formatTime(ms int) string {
	if ms <= 0 {
		return "0:00"
	}
	sec := ms / 1000
	m := sec / 60
	s := sec % 60
	return fmt.Sprintf("%d:%02d", m, s)
}

func progressBar(pos, dur, width int) string {
	if dur <= 0 || width <= 0 {
		bar := "["
		for i := 0; i < width; i++ {
			bar += "-"
		}
		bar += "]"
		return bar
	}
	f := float64(pos) / float64(dur)
	if f < 0 {
		f = 0
	}
	if f > 1 {
		f = 1
	}
	filled := int(f * float64(width))
	bar := "["
	for i := 0; i < width; i++ {
		if i < filled {
			bar += "#"
		} else {
			bar += "-"
		}
	}
	bar += "]"
	return bar
}

// setVolume sets the volume directly to the specified value (0-100)
func (m *model) setVolume(v int) {
	if m.selected == "" {
		return
	}
	m.volume = v
	url := fmt.Sprintf("http://%s:32500/player/playback/setParameters?volume=%d&commandID=1&type=music", m.selected, v)
	go func() { _, _ = http.Get(url) }()
}

func (m *model) triggerPlaybackCmd(fullURL string) tea.Cmd {
	if m.selected == "" {
		return func() tea.Msg {
			return playbackTriggeredMsg{success: false, err: fmt.Errorf("no server selected")}
		}
	}

	serverIP := m.selected
	shuffle := m.shuffle
	return func() tea.Msg {
		err := SendPlaybackURL(serverIP, fullURL, shuffle)
		if err != nil {
			return playbackTriggeredMsg{success: false, err: err}
		}
		return playbackTriggeredMsg{success: true}
	}
}
