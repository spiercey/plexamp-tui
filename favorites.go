package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	tea "github.com/charmbracelet/bubbletea"
)

// =====================
// Playback Config
// =====================

type FavoriteItem struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "artist", "album", "track", "playlist", "station"
	MetadataKey string `json:"key"`
}

type Favorites struct {
	Items []FavoriteItem `json:"items"`
}

func favoritesConfigPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "plexamp-tui", "favorites.json"), nil
}

func loadFavoritesConfig(path string) (*Favorites, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, os.ErrNotExist
	}
	if err != nil {
		return nil, err
	}

	var cfg Favorites
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}

func saveDefaultFavoritesConfig(path string) error {
	os.MkdirAll(filepath.Dir(path), 0755)

	defaultCfg := Favorites{
		Items: []FavoriteItem{
			{
				Name:        "Example Artist",
				Type:        "artist",
				MetadataKey: "12345",
			},
			{
				Name:        "Example Album",
				Type:        "album",
				MetadataKey: "12345",
			},
			{
				Name:        "Example Playlist",
				Type:        "playlist",
				MetadataKey: "12345",
			},
		},
	}

	data, _ := json.MarshalIndent(defaultCfg, "", "  ")
	return os.WriteFile(path, data, 0644)
}

// =====================
// Playback Trigger
// =====================

func (m *model) triggerFavoritePlayback(item FavoriteItem) tea.Cmd {
	logDebug(fmt.Sprintf("Triggering playback for %s", item.Name))
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

	switch item.Type {
	case "artist":
		logDebug(fmt.Sprintf("Playing artist: %s", item.Name))
		return func() tea.Msg { return m.playArtistCmd(item.MetadataKey)() }
	case "album":
		logDebug(fmt.Sprintf("Playing album: %s", item.Name))
		return func() tea.Msg { return m.playAlbumCmd(item.MetadataKey)() }
	case "playlist":
		logDebug(fmt.Sprintf("Playing playlist: %s", item.Name))
		return func() tea.Msg { return m.playPlaylistCmd(item.MetadataKey)() }
	case "station":
		logDebug(fmt.Sprintf("Playing station: %s", item.Name))
		return func() tea.Msg { return m.playArtistRadioCmd(item.MetadataKey)() }
	default:
		logDebug(fmt.Sprintf("Unknown type: %s", item.Type))
		return func() tea.Msg {
			return artistPlaybackMsg{success: false, err: fmt.Errorf("unknown type: %s", item.Type)}
		}
	}
}
