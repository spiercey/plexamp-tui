// internal/config/favorites.go
package config

import (
	"encoding/json"
	"errors"
	"os"
	"time"

	"plexamp-tui/internal/database"
)

// FavoriteItem represents a single favorite item
type FavoriteItem struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	MetadataKey string    `json:"key"`
	CreatedAt   time.Time `json:"created_at"`
}

// Favorites holds the list of favorite items
type Favorites struct {
	Items []FavoriteItem `json:"items"`
}

// FavoritesManager handles favorites configuration
type FavoritesManager struct {
	db *database.Database
}

var FavsManager *FavoritesManager

// NewFavoritesManager creates a new FavoritesManager
func NewFavoritesManager(db *database.Database) (*FavoritesManager, error) {
	FavsManager = &FavoritesManager{
		db: db,
	}
	return FavsManager, nil
}

// Add adds a new favorite item
func (fm *FavoritesManager) Add(item FavoriteItem) error {
	_, err := fm.db.DB.Exec(`
		INSERT INTO favorites (name, type, metadata_key)
		VALUES (?, ?, ?)
		ON CONFLICT(type, metadata_key) DO UPDATE SET name = excluded.name
	`, item.Name, item.Type, item.MetadataKey)
	return err
}

// Remove removes a favorite item by type and metadata key
func (fm *FavoritesManager) Remove(itemType, metadataKey string) error {
	_, err := fm.db.DB.Exec(`
		DELETE FROM favorites 
		WHERE type = ? AND metadata_key = ?
	`, itemType, metadataKey)
	return err
}

// List returns all favorite items
func (fm *FavoritesManager) List() ([]FavoriteItem, error) {
	rows, err := fm.db.DB.Query(`
		SELECT id, name, type, metadata_key, created_at 
		FROM favorites 
		ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var items []FavoriteItem
	for rows.Next() {
		var item FavoriteItem
		if err := rows.Scan(&item.ID, &item.Name, &item.Type, &item.MetadataKey, &item.CreatedAt); err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, rows.Err()
}

// Save is kept for backward compatibility but now uses the database
func (fm *FavoritesManager) Save(favorites *Favorites) error {
	// This is a no-op now since we're using the database directly
	// Existing code can still call Save() but it won't do anything
	return nil
}

// Load is kept for backward compatibility
func (fm *FavoritesManager) Load() (*Favorites, error) {
	items, err := fm.List()
	if err != nil {
		return nil, err
	}
	return &Favorites{Items: items}, nil
}

// MigrateFromJSON migrates data from JSON to SQLite
func (fm *FavoritesManager) MigrateFromJSON(jsonPath string) error {
	// Check if database is empty
	var count int
	err := fm.db.DB.QueryRow("SELECT COUNT(*) FROM favorites").Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		// Already migrated
		return nil
	}

	// Read from JSON file
	data, err := os.ReadFile(jsonPath)
	if errors.Is(err, os.ErrNotExist) {
		return nil // No JSON file to migrate
	}
	if err != nil {
		return err
	}

	var favs Favorites
	if err := json.Unmarshal(data, &favs); err != nil {
		return err
	}

	// Insert into database
	tx, err := fm.db.DB.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(`
		INSERT INTO favorites (name, type, metadata_key, created_at)
		VALUES (?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, item := range favs.Items {
		if _, err := stmt.Exec(item.Name, item.Type, item.MetadataKey, time.Now()); err != nil {
			return err
		}
	}

	return tx.Commit()
}
