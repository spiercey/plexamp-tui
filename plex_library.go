package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
)

// =====================
// Plex Library Types
// =====================

// PlexDirectory represents a generic directory item from Plex
type PlexDirectory struct {
	XMLName     xml.Name `xml:"Directory"`
	RatingKey   string   `xml:"ratingKey,attr"`
	Title       string   `xml:"title,attr"`
	Type        string   `xml:"type,attr"`
	ParentTitle string   `xml:"parentTitle,attr"` // For albums
	Year        string   `xml:"year,attr"`
}

// PlexArtist represents an artist from the Plex library
type PlexArtist struct {
	RatingKey string `xml:"ratingKey,attr"`
	Title     string `xml:"title,attr"`
	Type      string `xml:"type,attr"`
}

// PlexAlbum represents an album from the Plex library
type PlexAlbum struct {
	RatingKey   string `xml:"ratingKey,attr"`
	Title       string `xml:"title,attr"`
	ParentTitle string `xml:"parentTitle,attr"` // Artist name
	Year        string `xml:"year,attr"`
	Type        string `xml:"type,attr"`
}

// PlexPlaylist represents a playlist from the Plex library
type PlexPlaylist struct {
	RatingKey string `xml:"ratingKey,attr"`
	Title     string `xml:"title,attr"`
	Type      string `xml:"playlistType,attr"`
}

// PlexMediaContainer is the root element for Plex API responses
type PlexMediaContainer struct {
	XMLName     xml.Name        `xml:"MediaContainer"`
	Size        int             `xml:"size,attr"`
	Directories []PlexDirectory `xml:"Directory"`
}

type PlexPlaylistContainer struct {
	XMLName   xml.Name       `xml:"MediaContainer"`
	Size      int            `xml:"size,attr"`
	Playlists []PlexPlaylist `xml:"Playlist"`
}

// =====================
// Library Fetching
// =====================

// FetchArtists retrieves all artists from the Plex library
func FetchArtists(serverAddr, libraryID, token string) ([]PlexArtist, error) {
	urlStr := fmt.Sprintf("http://%s/library/sections/%s/all?type=8&X-Plex-Token=%s",
		serverAddr, libraryID, url.QueryEscape(token))

	logDebug(fmt.Sprintf("Fetching artists from: %s", urlStr))

	resp, err := http.Get(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch artists: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logDebug(fmt.Sprintf("Server returned status %d", resp.StatusCode))
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logDebug(fmt.Sprintf("Failed to read response: %v", err))
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var container PlexMediaContainer
	if err := xml.Unmarshal(body, &container); err != nil {
		logDebug(fmt.Sprintf("Failed to parse XML: %v", err))
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	var artists []PlexArtist
	for _, dir := range container.Directories {
		if dir.Type == "artist" {
			artists = append(artists, PlexArtist{
				RatingKey: dir.RatingKey,
				Title:     dir.Title,
				Type:      dir.Type,
			})
		}
	}

	logDebug(fmt.Sprintf("Fetched %d artists", len(artists)))

	// Sort artists alphabetically by title
	sort.Slice(artists, func(i, j int) bool {
		return artists[i].Title < artists[j].Title
	})

	return artists, nil
}

// FetchAlbums retrieves all albums from the Plex library
func FetchAlbums(serverAddr, libraryID, token string) ([]PlexAlbum, error) {
	urlStr := fmt.Sprintf("http://%s/library/sections/%s/all?type=9&X-Plex-Token=%s",
		serverAddr, libraryID, url.QueryEscape(token))

	logDebug(fmt.Sprintf("Fetching albums from: %s", urlStr))

	resp, err := http.Get(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch albums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logDebug(fmt.Sprintf("Server returned status %d", resp.StatusCode))
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		logDebug(fmt.Sprintf("Failed to read response: %v", err))
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var container PlexMediaContainer
	if err := xml.Unmarshal(body, &container); err != nil {
		logDebug(fmt.Sprintf("Failed to parse XML: %v", err))
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	var albums []PlexAlbum
	for _, dir := range container.Directories {
		if dir.Type == "album" {
			albums = append(albums, PlexAlbum{
				RatingKey:   dir.RatingKey,
				Title:       dir.Title,
				ParentTitle: dir.ParentTitle,
				Year:        dir.Year,
				Type:        dir.Type,
			})
		}
	}

	logDebug(fmt.Sprintf("Fetched %d albums", len(albums)))

	// Sort albums alphabetically by title
	sort.Slice(albums, func(i, j int) bool {
		return albums[i].ParentTitle < albums[j].ParentTitle
	})

	return albums, nil
}

// FetchArtistAlbums retrieves albums for a specific artist
func FetchArtistAlbums(serverAddr, artistRatingKey, token string) ([]PlexAlbum, error) {
	urlStr := fmt.Sprintf("http://%s/library/metadata/%s/children?X-Plex-Token=%s",
		serverAddr, artistRatingKey, url.QueryEscape(token))

	logDebug(fmt.Sprintf("Fetching albums for artist %s from: %s", artistRatingKey, urlStr))

	resp, err := http.Get(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch artist albums: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var container PlexMediaContainer
	if err := xml.Unmarshal(body, &container); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	return []PlexAlbum{}, nil
	// logDebug(fmt.Sprintf("Fetched %d albums for artist", len(container.Albums)))

	// return container.Albums, nil
}

// FetchPlaylists retrieves all playlists from the Plex library, using url format: curl "http://<serverAddr>:<port>/playlists?X-Plex-Token=<token>"
func FetchPlaylists(serverAddr, token string) ([]PlexPlaylist, error) {
	urlStr := fmt.Sprintf("http://%s/playlists?X-Plex-Token=%s", serverAddr, url.QueryEscape(token))

	logDebug(fmt.Sprintf("Fetching playlists from: %s", urlStr))

	resp, err := http.Get(urlStr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch playlists: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	var container PlexPlaylistContainer
	if err := xml.Unmarshal(body, &container); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	return container.Playlists, nil
}
