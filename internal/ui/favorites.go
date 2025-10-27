package ui

import (
	"fmt"

	"plexamp-tui/internal/config"

	tea "github.com/charmbracelet/bubbletea"
)

// =====================
// Playback Trigger
// =====================

func (m *model) triggerFavoriteRadioPlayback(item config.FavoriteItem) tea.Cmd {
	log.Debug(fmt.Sprintf("Triggering radio playback for %s", item.Name))
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

	m.lastCommand = fmt.Sprintf("Playing radio for %s", item.Name)

	return func() tea.Msg { return m.playArtistRadioCmd(item.MetadataKey)() }
}

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

	m.lastCommand = fmt.Sprintf("Playing %s", item.Name)
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
	default:
		log.Debug(fmt.Sprintf("Unknown type: %s", item.Type))
		return func() tea.Msg {
			return artistPlaybackMsg{success: false, err: fmt.Errorf("unknown type: %s", item.Type)}
		}
	}
}

func (m *model) addRemoveFavorite(name string, k string, t string) (tea.Model, tea.Cmd) {
	log.Debug(fmt.Sprintf("Toggling favorite for %s", name))
	favSet := m.getCurrentFavSet()
	if _, exists := favSet[k]; exists {
		log.Debug(fmt.Sprintf("Removing favorite: %s", name))
		// Delete selected playback item
		index := m.playbackList.Index()
		m.deletePlaybackItem(index)
		return m, nil
	}
	log.Debug(fmt.Sprintf("Adding favorite: %s", name))
	m.savePlaybackItem(name, k, t)
	return m, nil
}

func (m *model) getCurrentFavSet() map[string]struct{} {
	favSet := make(map[string]struct{})
	for _, pItem := range m.playbackList.Items() {
		pItem := pItem.(item)
		favSet[pItem.GetMetadataKey()] = struct{}{}
	}
	return favSet
}
