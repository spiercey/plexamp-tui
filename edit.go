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

	if editType == "server" {
		// Single input for server hostname/IP
		ti := textinput.New()
		ti.Placeholder = "server.local or 192.168.1.100"
		ti.Focus()
		ti.CharLimit = 256
		ti.Width = 50

		// Get current value (only if editing, not adding)
		if index >= 0 && index < len(m.list.Items()) {
			if currentItem, ok := m.list.Items()[index].(item); ok {
				ti.SetValue(string(currentItem))
			}
		}

		m.editInputs = []textinput.Model{ti}
	} else if editType == "playback" {
		// Two inputs: name and URL
		nameInput := textinput.New()
		nameInput.Placeholder = "Playlist Name"
		nameInput.Focus()
		nameInput.CharLimit = 100
		nameInput.Width = 50

		urlInput := textinput.New()
		urlInput.Placeholder = "https://listen.plex.tv/player/playback/..."
		urlInput.CharLimit = 1000
		urlInput.Width = 50

		// Get current values (only if editing, not adding)
		if index >= 0 && m.playbackConfig != nil && index < len(m.playbackConfig.Items) {
			nameInput.SetValue(m.playbackConfig.Items[index].Name)
			urlInput.SetValue(m.playbackConfig.Items[index].URL)
		}

		m.editInputs = []textinput.Model{nameInput, urlInput}
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
			if err := m.saveEdit(); err != nil {
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

// saveEdit saves the edited values to the config files
func (m *model) saveEdit() error {
	if m.editMode == "server" {
		return m.saveServerEdit()
	} else if m.editMode == "playback" {
		return m.savePlaybackEdit()
	}
	return fmt.Errorf("unknown edit mode: %s", m.editMode)
}

// saveServerEdit saves changes to server config
func (m *model) saveServerEdit() error {
	if len(m.editInputs) == 0 {
		return fmt.Errorf("no input fields")
	}

	newValue := m.editInputs[0].Value()
	if newValue == "" {
		return fmt.Errorf("server cannot be empty")
	}

	// Load current config
	cfgPath, err := configPath()
	if err != nil {
		return err
	}

	cfg, err := loadConfig(cfgPath)
	if err != nil {
		return err
	}

	// Update or add the value
	if m.editIndex == -1 {
		// Adding new item
		cfg.Instances = append(cfg.Instances, newValue)
	} else if m.editIndex < len(cfg.Instances) {
		// Editing existing item
		cfg.Instances[m.editIndex] = newValue
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
	for _, instance := range cfg.Instances {
		items = append(items, item(instance))
	}
	m.list.SetItems(items)

	// Return to servers panel
	m.panelMode = "servers"
	m.editInputs = nil

	return nil
}

// savePlaybackEdit saves changes to playback config
func (m *model) savePlaybackEdit() error {
	if len(m.editInputs) < 2 {
		return fmt.Errorf("missing input fields")
	}

	newName := m.editInputs[0].Value()
	newURL := m.editInputs[1].Value()

	if newName == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if newURL == "" {
		return fmt.Errorf("URL cannot be empty")
	}

	// Load current config
	cfgPath, err := playbackConfigPath()
	if err != nil {
		return err
	}

	cfg, err := loadPlaybackConfig(cfgPath)
	if err != nil {
		return err
	}

	// Update or add the value
	if m.editIndex == -1 {
		// Adding new item
		cfg.Items = append(cfg.Items, PlaybackItem{
			Name: newName,
			URL:  newURL,
		})
	} else if m.editIndex < len(cfg.Items) {
		// Editing existing item
		cfg.Items[m.editIndex].Name = newName
		cfg.Items[m.editIndex].URL = newURL
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

	if m.editMode == "server" {
		content = fmt.Sprintf("%s Server\n\n", action)
		content += "Hostname/IP:\n"
		if len(m.editInputs) > 0 {
			content += m.editInputs[0].View() + "\n"
		}
	} else if m.editMode == "playback" {
		content = fmt.Sprintf("%s Playback Item\n\n", action)
		content += "Name:\n"
		if len(m.editInputs) > 0 {
			content += m.editInputs[0].View() + "\n\n"
		}
		content += "URL:\n"
		if len(m.editInputs) > 1 {
			content += m.editInputs[1].View() + "\n"
		}
	}

	content += "\n\nEnter to save • Esc to cancel • Tab to switch fields"

	return content
}
