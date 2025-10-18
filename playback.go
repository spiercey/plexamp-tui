package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// =====================
// Playback Config
// =====================

type PlaybackItem struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

type PlaybackConfig struct {
	Items []PlaybackItem `json:"items"`
}

func playbackConfigPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "plexamp-tui", "playback.json"), nil
}

func loadPlaybackConfig(path string) (*PlaybackConfig, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}

	var cfg PlaybackConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func saveDefaultPlaybackConfig(path string) error {
	os.MkdirAll(filepath.Dir(path), 0755)

	defaultCfg := PlaybackConfig{
		Items: []PlaybackItem{
			{
				Name: "Example Station",
				URL:  "https://listen.plex.tv/player/playback/playMedia?address=YOUR_SERVER&machineIdentifier=YOUR_MACHINE_ID&key=/library/metadata/12345&type=music",
			},
		},
	}

	data, _ := json.MarshalIndent(defaultCfg, "", "  ")
	return os.WriteFile(path, data, 0644)
}

// =====================
// Playback Popup Model
// =====================

type playbackPopup struct {
	list     list.Model
	items    []PlaybackItem
	width    int
	height   int
	selected string
}

type playbackItem struct {
	name string
	url  string
}

func (i playbackItem) Title() string       { return i.name }
func (i playbackItem) Description() string { return "" }
func (i playbackItem) FilterValue() string { return i.name }

func newPlaybackPopup(items []PlaybackItem, width, height int) playbackPopup {
	var listItems []list.Item
	for _, item := range items {
		listItems = append(listItems, playbackItem{name: item.Name, url: item.URL})
	}

	l := list.New(listItems, list.NewDefaultDelegate(), width-4, height-6)
	l.Title = "Select Playback"
	l.SetShowHelp(false)
	l.SetShowStatusBar(false)
	l.SetFilteringEnabled(false)

	return playbackPopup{
		list:   l,
		items:  items,
		width:  width,
		height: height,
	}
}

func (p playbackPopup) Init() tea.Cmd {
	return nil
}

func (p playbackPopup) Update(msg tea.Msg) (playbackPopup, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		keyStr := msg.String()
		logDebug(fmt.Sprintf("Popup received key: '%s' (type: %d)", keyStr, msg.Type))
		// Handle selection keys BEFORE passing to list
		// Using 'p' for play since Enter isn't being received
		if keyStr == "enter" || keyStr == "" || keyStr == "p" || msg.Type == tea.KeyEnter {
			if selected, ok := p.list.SelectedItem().(playbackItem); ok {
				logDebug(fmt.Sprintf("Selected item: %s -> %s", selected.name, selected.url))
				p.selected = selected.url
				return p, nil
			} else {
				logDebug("No item selected or type assertion failed")
			}
			// Don't pass to the list
			return p, nil
		}
	}

	// Pass other keys to the list for navigation
	var cmd tea.Cmd
	p.list, cmd = p.list.Update(msg)
	return p, cmd
}

func (p playbackPopup) View() string {
	border := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("#00ffff")).
		Padding(1, 2).
		Width(p.width - 4).
		Height(p.height - 4)

	help := lipgloss.NewStyle().
		Foreground(lipgloss.Color("#8888ff")).
		MarginTop(1).
		Render("↑/↓ navigate • p to play • Esc cancel")

	content := lipgloss.JoinVertical(lipgloss.Left, p.list.View(), help)
	return border.Render(content)
}

// =====================
// Playback Trigger
// =====================

func triggerPlayback(serverIP, fullURL string) error {
	// Extract the path and query from the listen.plex.tv URL
	// The URL format is: https://listen.plex.tv/player/playback/playMedia?uri=...
	// We need to construct: http://serverIP:32500/player/playback/playMedia?uri=...
	
	// Remove the domain part and keep everything after it
	localURL := strings.Replace(fullURL, "https://listen.plex.tv", fmt.Sprintf("http://%s:32500", serverIP), 1)
	localURL = strings.Replace(localURL, "http://listen.plex.tv", fmt.Sprintf("http://%s:32500", serverIP), 1)

	// Log to file for debugging
	logDebug(fmt.Sprintf("Triggering playback to: %s", localURL))
	
	resp, err := http.Get(localURL)
	if err != nil {
		logDebug(fmt.Sprintf("Request error: %v", err))
		return fmt.Errorf("failed to connect to %s: %w", serverIP, err)
	}
	defer resp.Body.Close()

	logDebug(fmt.Sprintf("Response status: %d", resp.StatusCode))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	return nil
}

func logDebug(msg string) {
	// Log to a file in the config directory for debugging
	logPath, err := playbackConfigPath()
	if err != nil {
		return
	}
	logFile := filepath.Join(filepath.Dir(logPath), "playback.log")
	
	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()
	
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(f, "[%s] %s\n", timestamp, msg)
}
