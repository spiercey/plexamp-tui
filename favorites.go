package main

import (
	"fmt"
	"plexamp-tui/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

// =====================
// Playback Trigger
// =====================

func (m *model) triggerFavoritePlayback(item config.FavoriteItem) tea.Cmd {
	log.Debug(fmt.Sprintf("Triggering playback for %s", item.Name))
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
		log.Debug(fmt.Sprintf("Playing artist: %s", item.Name))
		return func() tea.Msg { return m.playArtistCmd(item.MetadataKey)() }
	case "album":
		log.Debug(fmt.Sprintf("Playing album: %s", item.Name))
		return func() tea.Msg { return m.playAlbumCmd(item.MetadataKey)() }
	case "playlist":
		log.Debug(fmt.Sprintf("Playing playlist: %s", item.Name))
		return func() tea.Msg { return m.playPlaylistCmd(item.MetadataKey)() }
	case "station":
		log.Debug(fmt.Sprintf("Playing station: %s", item.Name))
		return func() tea.Msg { return m.playArtistRadioCmd(item.MetadataKey)() }
	default:
		log.Debug(fmt.Sprintf("Unknown type: %s", item.Type))
		return func() tea.Msg {
			return artistPlaybackMsg{success: false, err: fmt.Errorf("unknown type: %s", item.Type)}
		}
	}
}
