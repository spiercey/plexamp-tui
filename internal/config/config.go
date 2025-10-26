// config/config.go
package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// Config holds the application configuration
type Config struct {
	ServerID           string        `json:"server_id"`            // Plex server ID for building playback URLs
	PlexServerAddr     string        `json:"plex_server_addr"`     // Plex server address for API calls
	PlexServerName     string        `json:"plex_server_name"`     // Plex server name for display
	PlexLibraryID      string        `json:"plex_library_id"`      // Music library ID for browsing
	SelectedPlayer     string        `json:"selected_player"`      // Selected player for playback
	SelectedPlayerName string        `json:"selected_player_name"` // Selected player name for display
	PlexLibraryName    string        `json:"plex_library_name"`    // Music library name for display
	PlexLibraries      []PlexLibrary `json:"plex_libraries"`       // List of Plex libraries
}

// PlexLibrary represents a Plex media library
type PlexLibrary struct {
	Key   string `json:"key"`
	Title string `json:"title"`
	Type  string `json:"type"`
}

// Manager handles configuration loading and saving
type Manager struct {
	configPath   string
	config       *Config
	UsingDefault bool
}

// NewManager creates a new configuration manager
func NewManager(configPath string) (*Manager, error) {
	if configPath == "" {
		//use default config path
		configPath, _ = getDefaultConfigPath()
	}
	mgr := &Manager{
		configPath:   configPath,
		UsingDefault: false,
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return nil, err
	}

	return mgr, nil
}

// Load loads the configuration from disk
func (m *Manager) Load() (*Config, error) {
	data, err := os.ReadFile(m.configPath)
	if errors.Is(err, os.ErrNotExist) {
		m.UsingDefault = true
		return m.createDefaultConfig()
	}
	if err != nil {
		return nil, err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}

	m.config = &cfg
	return &cfg, nil
}

// Save saves the current configuration to disk
func (m *Manager) Save(cfg *Config) error {
	m.config = cfg
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// GetConfig returns the current configuration
func (m *Manager) GetConfig() *Config {
	return m.config
}

// createDefaultConfig creates a new default configuration
func (m *Manager) createDefaultConfig() (*Config, error) {
	defaultCfg := &Config{
		ServerID:           "SELECT_SERVER",
		PlexServerAddr:     "127.0.0.1:32400",
		PlexServerName:     "SELECT_SERVER",
		PlexLibraryID:      "15",
		SelectedPlayer:     "127.0.0.1",
		SelectedPlayerName: "SELECT_PLAYER",
		PlexLibraryName:    "SELECT_LIBRARY",
		PlexLibraries: []PlexLibrary{
			{
				Key:   "15",
				Title: "SELECT_LIBRARY",
				Type:  "artist",
			},
		},
	}

	if err := m.Save(defaultCfg); err != nil {
		return nil, err
	}

	return defaultCfg, nil
}

// GetConfigPath returns the path to the configuration file
func (m *Manager) GetConfigPath() string {
	return m.configPath
}

func getDefaultConfigPath() (string, error) {
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

// GetLogPath returns the path to the log file
func (m *Manager) GetLogPath() string {
	return filepath.Join(filepath.Dir(m.configPath), "plexamp-tui.log")
}
