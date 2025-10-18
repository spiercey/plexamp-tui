package main

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
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
	// Only log if debug mode is enabled
	if !debugMode {
		return
	}
	
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
