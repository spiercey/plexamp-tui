package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// =====================
// Edit Mode Functions
// =====================

// initEditMode sets up the edit mode for either server or playback items
// index of -1 means adding a new item
func (m *model) initEditMode(editType string, index int) {
	m.panelMode = "edit"
	m.editMode = editType
	m.editIndex = index
	m.editFocusIndex = 0
	if editType == "playback" {
		// Two inputs: name and URL
		nameInput := textinput.New()
		nameInput.Placeholder = "Playlist Name"
		nameInput.Focus()
		nameInput.CharLimit = 100
		nameInput.Width = 50

		metadataKeyInput := textinput.New()
		metadataKeyInput.Placeholder = "Metadata Key"
		metadataKeyInput.CharLimit = 1000
		metadataKeyInput.Width = 50

		typeInput := textinput.New()
		typeInput.Placeholder = "artist"
		typeInput.CharLimit = 100
		typeInput.Width = 50

		// Get current values (only if editing, not adding)
		if index >= 0 && m.playbackConfig != nil && index < len(m.playbackConfig.Items) {
			nameInput.SetValue(m.playbackConfig.Items[index].Name)
			metadataKeyInput.SetValue(m.playbackConfig.Items[index].MetadataKey)
			typeInput.SetValue(m.playbackConfig.Items[index].Type)
		}

		m.editInputs = []textinput.Model{nameInput, typeInput, metadataKeyInput}
	}
}

// handleEditUpdate processes updates in edit mode
func (m *model) handleEditUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c":
			return m, tea.Quit

		case "esc":
			// Cancel edit and return to previous mode
			m.cancelEdit()
			return m, nil

		case "enter":
			// Save changes
			if err := m.savePlaybackEdit(); err != nil {
				m.lastCommand = fmt.Sprintf("Save failed: %v", err)
			} else {
				m.lastCommand = "Saved successfully"
			}
			return m, nil

		case "tab", "shift+tab":
			// Switch focus between inputs (only for playback with multiple fields)
			if len(m.editInputs) > 1 {
				if msg.String() == "tab" {
					m.editFocusIndex = (m.editFocusIndex + 1) % len(m.editInputs)
				} else {
					m.editFocusIndex--
					if m.editFocusIndex < 0 {
						m.editFocusIndex = len(m.editInputs) - 1
					}
				}

				// Update focus
				for i := range m.editInputs {
					if i == m.editFocusIndex {
						m.editInputs[i].Focus()
					} else {
						m.editInputs[i].Blur()
					}
				}
			}
			return m, nil
		}
	}

	// Update the focused input
	var cmd tea.Cmd
	if m.editFocusIndex < len(m.editInputs) {
		m.editInputs[m.editFocusIndex], cmd = m.editInputs[m.editFocusIndex].Update(msg)
	}
	return m, cmd
}

// cancelEdit returns to the previous panel mode
func (m *model) cancelEdit() {
	if m.editMode == "server" {
		m.panelMode = "servers"
	} else {
		m.panelMode = "playback"
	}
	m.editInputs = nil
}

// savePlaybackEdit saves changes to playback config
func (m *model) savePlaybackEdit() error {
	if len(m.editInputs) < 2 {
		return fmt.Errorf("missing input fields")
	}

	newName := m.editInputs[0].Value()
	newType := m.editInputs[1].Value()
	newMetadataKey := m.editInputs[2].Value()

	if newName == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if newType == "" {
		return fmt.Errorf("URL cannot be empty")
	}
	if newMetadataKey == "" {
		return fmt.Errorf("metadata key cannot be empty")
	}

	// Load current config
	cfgPath, err := favoritesConfigPath()
	if err != nil {
		return err
	}

	cfg, err := loadFavoritesConfig(cfgPath)
	if err != nil {
		return err
	}

	// Update or add the value
	if m.editIndex == -1 {
		// Adding new item
		cfg.Items = append(cfg.Items, FavoriteItem{
			Name:        newName,
			Type:        newType,
			MetadataKey: newMetadataKey,
		})
	} else if m.editIndex < len(cfg.Items) {
		// Editing existing item
		cfg.Items[m.editIndex].Name = newName
		cfg.Items[m.editIndex].Type = newType
		cfg.Items[m.editIndex].MetadataKey = newMetadataKey
	} else {
		return fmt.Errorf("invalid index")
	}

	// Save to file
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return err
	}

	// Update the list
	var items []list.Item
	for _, pb := range cfg.Items {
		items = append(items, item(pb.Name))
	}
	m.playbackList.SetItems(items)
	m.playbackConfig = cfg

	// Return to playback panel
	m.panelMode = "playback"
	m.editInputs = nil

	return nil
}

// editPanelView renders the edit panel
func (m model) editPanelView() string {
	var content string
	action := "Edit"
	if m.editIndex == -1 {
		action = "Add"
	}

	if m.editMode == "playback" {
		content = fmt.Sprintf("%s Playback Item\n\n", action)
		content += "Name:\n"
		if len(m.editInputs) > 0 {
			content += m.editInputs[0].View() + "\n\n"
		}
		content += "Type:\n"
		if len(m.editInputs) > 1 {
			content += m.editInputs[1].View() + "\n"
		}
		content += "Metadata Key:\n"
		if len(m.editInputs) > 2 {
			content += m.editInputs[2].View() + "\n"
		}
	}

	content += "\n\nEnter to save • Esc to cancel • Tab to switch fields"

	return content
}

// deletePlaybackItem removes a playback item from the config
func (m *model) deletePlaybackItem(index int) error {
	if index >= 0 && m.playbackConfig != nil && index < len(m.playbackConfig.Items) {
		m.playbackConfig.Items = append(m.playbackConfig.Items[:index], m.playbackConfig.Items[index+1:]...)
	}

	// Update the list
	var items []list.Item
	for _, pb := range m.playbackConfig.Items {
		items = append(items, item(pb.Name))
	}
	m.playbackList.SetItems(items)

	// Save to file
	data, err := json.MarshalIndent(m.playbackConfig, "", "  ")
	if err != nil {
		return err
	}

	cfgPath, err := favoritesConfigPath()
	if err != nil {
		return err
	}

	if err := os.WriteFile(cfgPath, data, 0644); err != nil {
		return err
	}

	return nil
}
