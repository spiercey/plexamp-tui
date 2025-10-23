package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func logDebug(msg string) {
	// Only log if debug mode is enabled
	if !debugMode {
		return
	}

	// Log to a file in the config directory for debugging
	logPath, err := getConfigPath()
	if err != nil {
		return
	}
	logFile := filepath.Join(filepath.Dir(logPath), "debug.log")

	f, err := os.OpenFile(logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return
	}
	defer f.Close()

	timestamp := time.Now().Format("2006-01-02 15:04:05")
	fmt.Fprintf(f, "[%s] %s\n", timestamp, msg)
}

func getConfigPath() (string, error) {
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

// saveServerConfig uses the model to fully save the config
func (m *model) saveServerConfig() error {
	cfgPath, err := getConfigPath()
	if err != nil {
		return err
	}

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return err
	}

	cfg.ServerID = m.config.ServerID
	cfg.PlexServerAddr = m.config.PlexServerAddr
	cfg.SelectedPlayer = m.config.SelectedPlayer
	cfg.SelectedPlayerName = m.config.SelectedPlayerName
	cfg.PlexServerName = m.config.PlexServerName
	cfg.PlexLibraryID = m.config.PlexLibraryID
	cfg.PlexLibraryName = m.config.PlexLibraryName
	cfg.PlexLibraries = m.config.PlexLibraries

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return err
	}

	return nil
}
