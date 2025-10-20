package main

import (
	"encoding/json"
	"encoding/xml"
	"errors"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// =====================
// Config management
// =====================

type Config struct {
	Instances []string `json:"instances"`
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

	if len(cfg.Instances) == 0 {
		return nil, fmt.Errorf("no instances defined in config file")
	}

	return &cfg, nil
}

func saveDefaultConfig(path string) error {
	os.MkdirAll(filepath.Dir(path), 0755)

	defaultCfg := Config{
		Instances: []string{
			"127.0.0.1",
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
	list            list.Model
	playbackList    list.Model
	selected        string
	status          string
	width           int
	height          int
	isPlaying       bool
	lastCommand     string
	currentTrack    string
	volume          int
	durationMs      int
	positionMs      int
	lastUpdate      time.Time
	usingDefaultCfg bool
	shuffle         bool // Tracks shuffle state

	timelineRequestID int

	// Panel mode: "servers" or "playback"
	panelMode      string
	playbackConfig *PlaybackConfig
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
	flag.Parse()

	debugMode = *debugFlag

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

	var items []list.Item
	for _, instance := range cfg.Instances {
		items = append(items, item(instance))
	}

	l := list.New(items, list.NewDefaultDelegate(), 0, 0)
	l.Title = "Select Instance"
	if len(items) > 0 {
		l.Select(0)
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
		list:            l,
		playbackList:    playbackList,
		selected:        string(items[0].(item)),
		usingDefaultCfg: usingDefault || items[0].(item) == "127.0.0.1",
		playbackConfig:  playbackCfg,
		panelMode:       "playback",
		shuffle:         true, // Default shuffle to ON
	}

	p := tea.NewProgram(m, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error:", err)
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
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		// Set list sizes for 2-column layout (left panel takes half width)
		m.list.SetSize(msg.Width/2-4, msg.Height-4)
		m.playbackList.SetSize(msg.Width/2-4, msg.Height-4)
		return m, nil

	case tea.KeyMsg:
		// Handle playback selection (when in playback mode)
		if m.panelMode == "playback" {
			switch msg.String() {
			case "enter":
				// Select playback item - don't switch back to servers
				if selected, ok := m.playbackList.SelectedItem().(item); ok {
					// Find the matching playback config item
					for _, pb := range m.playbackConfig.Items {
						if pb.Name == string(selected) {
							// Modify URL based on shuffle state
							modifiedURL := pb.URL
							u, err := url.Parse(pb.URL)
							if err == nil {
								q := u.Query()
								if m.shuffle {
									q.Set("shuffle", "1")
								} else {
									q.Del("shuffle")
								}
								u.RawQuery = q.Encode()
								modifiedURL = u.String()
							}
							logDebug(fmt.Sprintf("Selected playback: %s -> %s (shuffle: %v)", pb.Name, modifiedURL, m.shuffle))
							return m, m.triggerPlaybackCmd(modifiedURL)
						}
					}
				}
				return m, nil
			}
		}

		// Main app key handlers (only processed when popup is NOT open)
		switch msg.String() {
		case "ctrl+c", "q":
			return m, tea.Quit

		case "enter":
			selected, ok := m.list.SelectedItem().(item)
			if ok {
				m.selected = string(selected)
				m.status = fmt.Sprintf("Selected: %s", m.selected)
				// Reset playback info when switching
				m.currentTrack = ""
				m.isPlaying = false
				m.volume = 0
				m.durationMs = 0
				m.positionMs = 0
				m.lastUpdate = time.Time{}
				m.timelineRequestID++
				return m, m.pollTimeline()
			}

		case "p":
			if m.isPlaying {
				m.sendCommand("playback/pause")
				m.isPlaying = false
				m.lastCommand = "Pause"
			} else {
				m.sendCommand("playback/play")
				m.isPlaying = true
				m.lastCommand = "Play"
			}

		case "n":
			m.sendCommand("playback/skipNext")
			m.lastCommand = "Next"

		case "b":
			m.sendCommand("playback/skipPrevious")
			m.lastCommand = "Previous"

		case "+", "]":
			newVol := m.volume + 5
			if newVol > 100 {
				newVol = 100
			}
			m.setVolume(newVol)
			m.lastCommand = fmt.Sprintf("Volume %d", newVol)

		case "-", "[":
			newVol := m.volume - 5
			if newVol < 0 {
				newVol = 0
			}
			m.setVolume(newVol)
			m.lastCommand = fmt.Sprintf("Volume %d", newVol)

		case "s", "tab":
			// Toggle between servers and playback panels
			if m.playbackConfig != nil && len(m.playbackConfig.Items) > 0 {
				if m.panelMode == "servers" {
					logDebug(fmt.Sprintf("Switching to playback panel with %d items", len(m.playbackConfig.Items)))
					m.panelMode = "playback"
				} else {
					logDebug("Switching to servers panel")
					m.panelMode = "servers"
				}
			}
			return m, nil

		case "h":
			// Toggle shuffle state
			m.shuffle = !m.shuffle
			if m.shuffle {
				m.lastCommand = "Shuffle: ON"
			} else {
				m.lastCommand = "Shuffle: OFF"
			}
			return m, nil
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
	}

	// Update the appropriate list based on panel mode
	var cmd tea.Cmd
	if m.panelMode == "playback" {
		m.playbackList, cmd = m.playbackList.Update(msg)
	} else {
		m.list, cmd = m.list.Update(msg)
	}
	return m, cmd
}

func (m model) View() string {
	border := lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(0, 1)
	title := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("#00ffff")).Render("🎧 Plexamp Control")

	// Build left panel content
	var leftPanelContent string
	if m.panelMode == "playback" {
		leftPanelContent = m.playbackList.View()
	} else {
		leftPanelContent = m.list.View()
	}

	// Left panel
	leftPanel := border.Width(m.width/2 - 2).Render(leftPanelContent)

	// Right side has two stacked panels
	playbackPanel := border.Width(m.width/2 - 2).Render(m.playbackStatusView())
	controlsPanel := border.Width(m.width/2 - 2).Render(m.appControlsView())
	rightSide := lipgloss.JoinVertical(lipgloss.Left, playbackPanel, controlsPanel)

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

	selected := "None"
	if m.selected != "" {
		selected = m.selected
	}

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

	body += fmt.Sprintf(
		"%s: %s\n%s: %s\n%s: %s\n",
		info.Render("Server"), value.Render(selected),
		info.Render("Shuffle"), shuffleValue,
		info.Render("Last Command"), value.Render(m.lastCommand),
	)

	shuffleStatus := "OFF"
	if m.shuffle {
		shuffleStatus = "ON"
	}

	controlsText := fmt.Sprintf("Controls:\n  ↑/↓ navigate\n  Enter select\n  p Play/Pause\n  n Next\n  b Back\n  +/- Volume\n  s/Tab Panel\n  h Shuffle (%s)\n  q Quit", shuffleStatus)
	controls := lipgloss.NewStyle().MarginTop(1).Foreground(lipgloss.Color("#8888ff")).Render(controlsText)

	return fmt.Sprintf("%s\n%s", body, controls)
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
	return func() tea.Msg {
		err := triggerPlayback(serverIP, fullURL)
		if err != nil {
			return playbackTriggeredMsg{success: false, err: err}
		}
		return playbackTriggeredMsg{success: true}
	}
}
