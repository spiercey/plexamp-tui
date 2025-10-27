package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// FavoriteItem represents a single favorite item
type FavoriteItem struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // "artist", "album", "playlist"
	MetadataKey string `json:"key"`
}

// Favorites holds the list of favorite items
type Favorites struct {
	Items []FavoriteItem `json:"items"`
}

// FavoritesManager handles favorites configuration
type FavoritesManager struct {
	favoritesPath string
}

// NewFavoritesManager creates a new FavoritesManager
func NewFavoritesManager() (*FavoritesManager, error) {
	path, err := getFavoritesPath()
	if err != nil {
		return nil, err
	}

	return &FavoritesManager{
		favoritesPath: path,
	}, nil
}

// GetFavoritesPath returns the path to the favorites file
func (fm *FavoritesManager) GetFavoritesPath() string {
	return fm.favoritesPath
}

// Load loads the favorites from disk
func (fm *FavoritesManager) Load() (*Favorites, error) {
	data, err := os.ReadFile(fm.favoritesPath)
	if errors.Is(err, os.ErrNotExist) {
		return fm.createDefaultFavorites()
	}
	if err != nil {
		return nil, err
	}

	var favs Favorites
	if err := json.Unmarshal(data, &favs); err != nil {
		return nil, err
	}

	return &favs, nil
}

// Save saves the favorites to disk
func (fm *FavoritesManager) Save(favs *Favorites) error {
	if err := os.MkdirAll(filepath.Dir(fm.favoritesPath), 0o755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(favs, "", "  ")
	if err != nil {
		return err
	}

	tempPath := fm.favoritesPath + ".tmp"
	if err := os.WriteFile(tempPath, data, 0o644); err != nil {
		return err
	}

	return os.Rename(tempPath, fm.favoritesPath)
}

// createDefaultFavorites creates a new default favorites configuration
func (fm *FavoritesManager) createDefaultFavorites() (*Favorites, error) {
	defaultFavs := &Favorites{
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

	if err := fm.Save(defaultFavs); err != nil {
		return nil, err
	}

	return defaultFavs, nil
}

// getFavoritesPath returns the path to the favorites file
func getFavoritesPath() (string, error) {
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
