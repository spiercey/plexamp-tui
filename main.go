package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// =====================
// Config management
// =====================

type Config struct {
	ServerID           string        `json:"server_id"`            // Plex server ID for building playback URLs
	PlexServerAddr     string        `json:"plex_server_addr"`     // Plex server address for API calls (e.g., "jakku.lan:32400")
	PlexServerName     string        `json:"plex_server_name"`     // Plex server name for displayj
	PlexLibraryID      string        `json:"plex_library_id"`      // Music library ID for browsing
	SelectedPlayer     string        `json:"selected_player"`      // Selected player for playback
	SelectedPlayerName string        `json:"selected_player_name"` // Selected player name for display
	PlexLibraryName    string        `json:"plex_library_name"`    // Music library name for display
	PlexLibraries      []PlexLibrary `json:"plex_libraries"`       // List of Plex libraries
}

func configPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "plexamp-tui", "config.json"), nil
}

func loadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	logDebug(fmt.Sprintf("Loaded config: %v", cfg))
	return &cfg, nil
}

func saveDefaultConfig(path string) error {
	os.MkdirAll(filepath.Dir(path), 0755)

	defaultCfg := Config{
		ServerID:           "YOUR_SERVER_ID_HERE",
		PlexServerAddr:     "127.0.0.1:32400",
		PlexServerName:     "YOUR_SERVER_NAME_HERE",
		PlexLibraryID:      "15",
		SelectedPlayer:     "127.0.0.1",
		SelectedPlayerName: "YOUR_PLAYER_NAME_HERE",
		PlexLibraryName:    "YOUR_LIBRARY_NAME_HERE",
		PlexLibraries: []PlexLibrary{
			{
				Key:   "15",
				Title: "YOUR_LIBRARY_NAME_HERE",
				Type:  "artist",
			},
		},
	}

	data, _ := json.MarshalIndent(defaultCfg, "", "  ")
	return os.WriteFile(path, data, 0644)
}

// =====================
// TUI Types
// =====================

type item string

func (i item) Title() string       { return string(i) }
func (i item) Description() string { return "" }
func (i item) FilterValue() string { return string(i) }

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
	playbackConfig *PlaybackConfig
	config         *Config // Store config for server ID access

	// Edit mode fields
	editMode       string // "server" or "playback"
	editIndex      int    // Index of item being edited
	editInputs     []textinput.Model
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

type trackMsg string
type errMsg struct{ err error }
type pollMsg struct{}

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

// Global debug flag
var debugMode bool

// =====================
// Main
// =====================

func main() {
	// CLI flags
	configFlag := flag.String("config", "", "Path to configuration file (optional)")
	debugFlag := flag.Bool("debug", false, "Enable debug logging")
	authFlag := flag.Bool("auth", false, "Authenticate with Plex.tv")
	flag.Parse()

	debugMode = *debugFlag

	// Handle Plex authentication
	if *authFlag {
		fmt.Println("Starting Plex authentication...")
		_, err := authenticateWithPlex()
		if err != nil {
			fmt.Printf("Authentication failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("\nAuthentication complete! You can now run plexamp-tui normally.")
		return
	}

	var cfg *Config
	var cfgPath string
	var err error

	if *configFlag != "" {
		cfgPath = *configFlag
	} else {
		cfgPath, err = configPath()
		if err != nil {
			fmt.Println("Error determining config path:", err)
			os.Exit(1)
		}
	}

	cfg, err = loadConfig(cfgPath)
	usingDefault := false
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			fmt.Printf("No config found, creating default one at %s\n", cfgPath)
			if err := saveDefaultConfig(cfgPath); err != nil {
				fmt.Println("Error creating default config:", err)
				os.Exit(1)
			}
			cfg, err = loadConfig(cfgPath)
			if err != nil {
				fmt.Println("Error reloading config:", err)
				os.Exit(1)
			}
			usingDefault = true
		} else {
			fmt.Println("Error loading config:", err)
			os.Exit(1)
		}
	}

	// Load playback config
	playbackCfgPath, _ := playbackConfigPath()
	playbackCfg, err := loadPlaybackConfig(playbackCfgPath)
	if err != nil && os.IsNotExist(err) {
		fmt.Printf("No playback config found, creating default one at %s\n", playbackCfgPath)
		if err := saveDefaultPlaybackConfig(playbackCfgPath); err != nil {
			fmt.Println("Warning: Could not create default playback config:", err)
		}
		playbackCfg, _ = loadPlaybackConfig(playbackCfgPath)
	}

	// Create playback list
	var playbackItems []list.Item
	if playbackCfg != nil {
		for _, pb := range playbackCfg.Items {
			playbackItems = append(playbackItems, item(pb.Name))
		}
	}
	playbackList := list.New(playbackItems, list.NewDefaultDelegate(), 0, 0)
	playbackList.Title = "Select Playback"

	m := model{
		playbackList:      playbackList,
		artistList:        list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
		albumList:         list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
		playlistList:      list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
		serverList:        list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
		playerList:        list.New([]list.Item{}, list.NewDefaultDelegate(), 0, 0),
		selected:          cfg.SelectedPlayer,
		usingDefaultCfg:   usingDefault,
		playbackConfig:    playbackCfg,
		config:            cfg,
		panelMode:         "playback",
		shuffle:           true, // Default shuffle to ON
		plexAuthenticated: verifyPlexAuthentication(),
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error:", err)
	}

	serverInfo, err := GetPlexServerInformation()
	if err != nil {
		logDebug(fmt.Sprintf("Error getting server information: %v", err))
		os.Exit(1)
	}
	logDebug(fmt.Sprintf("Server information: %v", serverInfo))
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
			m.saveServerConfig()
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
			//check if new library list has our configured library
			for _, lib := range msg.libraries {
				if lib.Title == m.config.PlexLibraryName {
					found = true
					break
				}
			}

			if !found {
				m.config.PlexLibraryName = msg.libraries[0].Title
				m.config.PlexLibraryID = msg.libraries[0].Key
			}

			m.saveServerConfig()
			m.lastCommand = "Server Selected"
			m.status = ""
			m.panelMode = "playback" // Return to playback view after selection
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Set list sizes for 2-column layout (left panel takes half width)
		m.playbackList.SetSize(msg.Width/2-4, msg.Height-4)
		m.artistList.SetSize(msg.Width/2-4, msg.Height-4)
		m.albumList.SetSize(msg.Width/2-4, msg.Height-4)
		m.playlistList.SetSize(msg.Width/2-4, msg.Height-4)
		m.serverList.SetSize(msg.Width/2-4, msg.Height-4)
		m.playerList.SetSize(msg.Width/2-4, msg.Height-4)
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

		// Handle playback selection (when in playback mode)
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

			case "enter":
				// Select playback item - don't switch back to servers
				if selected, ok := m.playbackList.SelectedItem().(item); ok {
					// Find the matching playback config item
					for _, pb := range m.playbackConfig.Items {
						if pb.Name == string(selected) {
							logDebug(fmt.Sprintf("Selected playback: %s -> %s (shuffle: %v)", pb.Name, pb.URL, m.shuffle))
							return m, m.triggerPlaybackCmd(pb.URL)
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

		case "3":
			// Open artist browse (if authenticated)
			if m.plexAuthenticated && m.config != nil {
				m.initArtistBrowse()
				return m, m.fetchArtistsCmd()
			} else {
				m.status = "Plex authentication required (run with --auth)"
			}
			return m, nil

		case "4":
			// Open album browse (if authenticated)
			if m.plexAuthenticated && m.config != nil {
				m.initAlbumBrowse()
				return m, m.fetchAlbumsCmd()
			} else {
				m.status = "Plex authentication required (run with --auth)"
			}
			return m, nil

		case "5":
			// Open playlist browse (if authenticated)
			if m.plexAuthenticated && m.config != nil {
				m.initPlaylistBrowse()
				return m, m.fetchPlaylistsCmd()
			} else {
				m.status = "Plex authentication required (run with --auth)"
			}
			return m, nil

		case "6":
			// Open server browse (if authenticated)
			if m.plexAuthenticated && m.config != nil {
				m.initServerBrowse()
				return m, m.fetchServersCmd()
			} else {
				m.status = "Plex authentication required (run with --auth)"
			}
			return m, nil

		case "7":
			// Open player browse (if authenticated)
			if m.plexAuthenticated && m.config != nil {
				m.initPlayerBrowse()
				return m, m.fetchPlayersCmd()
			} else {
				m.status = "Plex authentication required (run with --auth)"
			}
			return m, nil

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
	if m.panelMode == "playback" {
		leftPanelContent = m.playbackList.View()
	} else if m.panelMode == "plex-artists" {
		leftPanelContent = m.artistList.View()
	} else if m.panelMode == "plex-albums" {
		leftPanelContent = m.albumList.View()
	} else if m.panelMode == "plex-playlists" {
		leftPanelContent = m.playlistList.View()
	} else if m.panelMode == "plex-servers" {
		leftPanelContent = m.serverList.View()
	} else if m.panelMode == "plex-players" {
		leftPanelContent = m.playerList.View()
	}

	// Left panel
	leftPanel := border.Width(m.width/2 - 2).Render(leftPanelContent)

	// Right side has two stacked panels
	playbackPanel := border.Width(m.width/2 - 2).Render(m.playbackStatusView())
	libraryPanel := border.Width(m.width/2 - 2).Render(m.libraryControlsView())
	controlsPanel := border.Width(m.width/2 - 2).Render(m.appControlsView())
	rightSide := lipgloss.JoinVertical(lipgloss.Left, playbackPanel, libraryPanel, controlsPanel)

	content := lipgloss.JoinHorizontal(lipgloss.Top, leftPanel, rightSide)
	return lipgloss.JoinVertical(lipgloss.Left, title, content)
}

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

func (m model) appControlsView() string {
	info := lipgloss.NewStyle().Foreground(lipgloss.Color("#aaaaaa"))
	value := lipgloss.NewStyle().Foreground(lipgloss.Color("#00ffcc")).Bold(true)

	body := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#ffaa00")).Render("App Info") + "\n\n"

	if m.usingDefaultCfg {
		body += lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5555")).Render(
			"⚠️ Using default config\n\n")
	}

	// Shuffle status with color
	var shuffleValue string
	if m.shuffle {
		shuffleValue = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Bold(true).Render("ON")
	} else {
		shuffleValue = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5555")).Bold(true).Render("OFF")
	}

	// Plex authentication status with color
	var authValue string
	if m.plexAuthenticated {
		authValue = lipgloss.NewStyle().Foreground(lipgloss.Color("#00ff00")).Bold(true).Render("✓ Authenticated")
	} else {
		authValue = lipgloss.NewStyle().Foreground(lipgloss.Color("#ff5555")).Bold(true).Render("✗ Not Authenticated")
	}

	body += fmt.Sprintf(
		"%s: %s\n%s: %s\n%s: %s\n%s: %s\n%s: %s\n",
		info.Render("Server"), value.Render(m.config.PlexServerName),
		info.Render("Player"), value.Render(m.config.SelectedPlayerName),
		info.Render("Plex"), authValue,
		info.Render("Shuffle"), shuffleValue,
		info.Render("Last Command"), value.Render(m.lastCommand),
	)

	shuffleStatus := "OFF"
	if m.shuffle {
		shuffleStatus = "ON"
	}

	plexControls := ""
	if m.plexAuthenticated {
		plexControls = "\n  1 Servers  2 Libraries  3 Artists  4 Albums"
	}

	controlsText := fmt.Sprintf("Controls:\n  ↑/↓ navigate\n  Enter select\n  a Add  e Edit\n  p Play/Pause\n  n Next\n  b Back\n  +/- Volume\n  s/Tab Panel\n  h Shuffle (%s)%s\n  q Quit", shuffleStatus, plexControls)
	controls := lipgloss.NewStyle().MarginTop(1).Foreground(lipgloss.Color("#8888ff")).Render(controlsText)

	return fmt.Sprintf("%s\n%s", body, controls)
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
				m.saveServerConfig()
			} else {
				m.config.PlexLibraryID = m.config.PlexLibraries[i+1].Key
				m.config.PlexLibraryName = m.config.PlexLibraries[i+1].Title
				m.saveServerConfig()
			}
			break
		}
	}
	return nil
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
