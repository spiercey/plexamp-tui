package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"plexamp-tui/internal/config"
	"plexamp-tui/internal/database"
	"plexamp-tui/internal/logger"
	"plexamp-tui/internal/plex"
	"plexamp-tui/internal/ui"

	tea "github.com/charmbracelet/bubbletea"
)

// Replace the Config struct and related functions with:
var (
	log         *logger.Logger
	cfgManager  *config.Manager
	favsManager *config.FavoritesManager
	plexClient  *plex.PlexClient
)

// =====================
// Main
// =====================

func main() {
	// CLI flags
	var debug bool
	var err error
	configFlag := flag.String("config", "", "Path to configuration file (optional)")
	flag.BoolVar(&debug, "debug", false, "Enable debug logging")
	authFlag := flag.Bool("auth", false, "Authenticate with Plex.tv")
	flag.Parse()

	// Initialize config
	cfgManager, err = config.NewManager(*configFlag)
	if err != nil {
		log.Fatal("Failed to initialize config manager: %v", err)
	}

	cfg, err := cfgManager.Load()
	if err != nil {
		log.Fatal("Failed to load config: %v", err)
	}

	// Initialize logger
	log, err = logger.NewLogger(debug, cfgManager.GetLogPath())
	if err != nil {
		fmt.Println("Error initializing logger:", err)
		os.Exit(1)
	}
	defer log.Close()

	plexClient = plex.NewPlexClient(log)

	// Handle Plex authentication
	if *authFlag {
		fmt.Println("Starting Plex authentication...")
		_, err := plexClient.AuthenticateWithPlex()
		if err != nil {
			fmt.Printf("Authentication failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("\nAuthentication complete! You can now run plexamp-tui normally.")
		return
	}

	// Initialize database
	dbPath := filepath.Join(cfgManager.GetConfigDir(), "favorites.db")
	db, err := database.New(dbPath)
	if err != nil {
		log.Fatal("Failed to initialize database: %v", err)
	}
	defer db.Close()

	// Initialize favorites manager
	favsManager, err := config.NewFavoritesManager(db)
	if err != nil {
		log.Fatal("Failed to initialize favorites manager: %v", err)
	}

	// Migrate from JSON if needed
	jsonPath := filepath.Join(cfgManager.GetConfigDir(), "favorites.json")
	if err := favsManager.MigrateFromJSON(jsonPath); err != nil {
		log.Warn("Failed to migrate favorites from JSON: %v", err)
	}
	// Load favorites
	favs, err := favsManager.Load()
	if err != nil {
		log.Fatal("Failed to load favorites: %v", err)
	}

	uiManager := ui.NewUiManager(log, cfg, cfgManager, favs, plexClient, favsManager)

	p := tea.NewProgram(uiManager.Model, tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Println("Error:", err)
	}

	serverInfo, err := plexClient.GetPlexServerInformation()
	if err != nil {
		log.Debug(fmt.Sprintf("Error getting server information: %v", err))
		os.Exit(1)
	}
	log.Debug(fmt.Sprintf("Server information: %v", serverInfo))
}
