package main

import (
	"fmt"
	"net/http"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// =====================
// URL Builder for Plex Playback
// =====================

const (
	plexListenBaseURL = "https://listen.plex.tv"
	plexURIPrefix     = "server://%s/com.plexapp.plugins.library/library/metadata/%s"
)

// PlaybackURLBuilder handles building and sending Plex playback URLs
type PlaybackURLBuilder struct {
	serverID string
}

// NewPlaybackURLBuilder creates a new URL builder with the given server ID
func NewPlaybackURLBuilder(serverID string) *PlaybackURLBuilder {
	return &PlaybackURLBuilder{
		serverID: serverID,
	}
}

// BuildPlaylistURL builds a URL for playing a playlist
func (b *PlaybackURLBuilder) BuildPlaylistURL(metadataID string) string {
	uri := fmt.Sprintf(plexURIPrefix, b.serverID, metadataID)
	u := fmt.Sprintf("%s/player/playback/createPlayQueue?source=%s&uri=%s&playlistID=%s&type=audio", plexListenBaseURL, url.QueryEscape(b.serverID), url.QueryEscape(uri), metadataID)
	return u
}

// BuildPlayQueueURL builds a URL for creating a play queue
// This is used for playing albums, tracks, etc.
func (b *PlaybackURLBuilder) BuildPlayQueueURL(metadataID string) string {
	uri := fmt.Sprintf(plexURIPrefix, b.serverID, metadataID)
	u := fmt.Sprintf("%s/player/playback/createPlayQueue?uri=%s", plexListenBaseURL, url.QueryEscape(uri))
	return u
}

// BuildArtistRadioURL builds a URL for playing artist radio/station
// This requires a station UUID in addition to the metadata ID
func (b *PlaybackURLBuilder) BuildArtistRadioURL(metadataID, stationUUID string) string {
	uri := fmt.Sprintf(plexURIPrefix+"/station/%s", b.serverID, metadataID, stationUUID)
	u := fmt.Sprintf("%s/player/playback/playMedia?type=10&type=audio&uri=%s",
		plexListenBaseURL, url.QueryEscape(uri))
	return u
}

// ApplyShuffle modifies a URL to add or remove the shuffle parameter
func ApplyShuffle(urlStr string, shuffle bool) (string, error) {
	u, err := url.Parse(urlStr)
	if err != nil {
		return urlStr, err
	}

	q := u.Query()
	if shuffle {
		q.Set("shuffle", "1")
	} else {
		q.Del("shuffle")
	}
	u.RawQuery = q.Encode()

	return u.String(), nil
}

// SendPlaybackURL sends a playback URL to the local Plexamp server
// It takes the full listen.plex.tv URL and converts it to a local server request
func SendPlaybackURL(serverIP, fullURL string, shuffle bool) error {
	// Apply shuffle if needed
	modifiedURL := fullURL
	if shuffleURL, err := ApplyShuffle(fullURL, shuffle); err == nil {
		modifiedURL = shuffleURL
	}

	// Convert listen.plex.tv URL to local server URL
	localURL := strings.Replace(modifiedURL, "https://listen.plex.tv", fmt.Sprintf("http://%s:32500", serverIP), 1)
	localURL = strings.Replace(localURL, "http://listen.plex.tv", fmt.Sprintf("http://%s:32500", serverIP), 1)

	logDebug(fmt.Sprintf("Sending playback URL: %s", localURL))

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

// PlayMetadata plays a specific metadata item (track, album, artist, etc.)
// This is a convenience function that builds the URL and sends it
func PlayMetadata(serverIP, serverID, metadataID string, shuffle bool) error {
	builder := NewPlaybackURLBuilder(serverID)
	playbackURL := builder.BuildPlayQueueURL(metadataID)
	return SendPlaybackURL(serverIP, playbackURL, shuffle)
}

// PlayArtistRadio plays an artist radio station
// This is a convenience function that builds the URL and sends it
// It generates a new UUID for each call to ensure a fresh radio station
func PlayArtistRadio(serverIP, serverID, metadataID string, shuffle bool) error {
	// Generate a new UUID for the station
	stationUUID := uuid.New().String()
	builder := NewPlaybackURLBuilder(serverID)
	playbackURL := builder.BuildArtistRadioURL(metadataID, stationUUID)
	return SendPlaybackURL(serverIP, playbackURL, shuffle)
}

// PlayPlaylist plays a specific playlist
// This is a convenience function that builds the URL and sends it
func PlayPlaylist(serverIP, serverID, metadataID string, shuffle bool) error {
	builder := NewPlaybackURLBuilder(serverID)
	playbackURL := builder.BuildPlaylistURL(metadataID)
	return SendPlaybackURL(serverIP, playbackURL, shuffle)
}
