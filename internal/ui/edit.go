package ui

import (
	"fmt"

	"plexamp-tui/internal/config"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type typeItem string

func (i typeItem) Title() string       { return string(i) }
func (i typeItem) Description() string { return "" }
func (i typeItem) FilterValue() string { return string(i) }

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

		// Create list for type selection
		typeItems := []list.Item{
			typeItem("Artist"),
			typeItem("Album"),
			typeItem("Playlist"),
		}

		// Initialize the list with default settings
		typeSelect := list.New(typeItems, list.NewDefaultDelegate(), 30, 10)
		typeSelect.Title = "Select Type"
		typeSelect.SetShowStatusBar(false)
		typeSelect.SetFilteringEnabled(false)

		// Set default value if editing
		if index >= 0 && m.playbackConfig != nil && index < len(m.playbackConfig.Items) {
			switch m.playbackConfig.Items[index].Type {
			case "artist":
				typeSelect.Select(0)
			case "album":
				typeSelect.Select(1)
			case "playlist":
				typeSelect.Select(2)
			}
		}

		m.typeSelect = typeSelect

		// Get current values (only if editing, not adding)
		if index >= 0 && m.playbackConfig != nil && index < len(m.playbackConfig.Items) {
			nameInput.SetValue(m.playbackConfig.Items[index].Name)
			metadataKeyInput.SetValue(m.playbackConfig.Items[index].MetadataKey)
		}

		m.editInputs = []textinput.Model{nameInput, metadataKeyInput}
	}
}

// handleEditUpdate processes updates in edit mode
func (m *model) handleEditUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch key := msg.String(); key {
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

		case "tab":
			// Move focus to next element
			m.editFocusIndex = (m.editFocusIndex + 1) % 3 // We have 3 focusable elements now
			m.updateFocus()
			return m, nil

		case "shift+tab":
			// Move focus to previous element
			m.editFocusIndex--
			if m.editFocusIndex < 0 {
				m.editFocusIndex = 2 // Wrap around to the last element
			}
			m.updateFocus()
			return m, nil

		// Handle arrow key navigation for type selection (only when type selector is focused)
		case "left", "h":
			if m.editFocusIndex == 1 { // Only process if type selector is focused
				currentIndex := m.typeSelect.Index()
				if currentIndex > 0 {
					m.typeSelect.Select(currentIndex - 1)
				}
				return m, nil
			}

		case "right", "l":
			if m.editFocusIndex == 1 { // Only process if type selector is focused
				currentIndex := m.typeSelect.Index()
				if currentIndex < len(m.typeSelect.Items())-1 {
					m.typeSelect.Select(currentIndex + 1)
				}
				return m, nil
			}

		// For all other keys, let the input handling below take care of it
		default:
			// No special handling needed here - let the input processing below handle it
		}
	}

	var cmd tea.Cmd

	// Handle input based on focus
	switch m.editFocusIndex {
	case 0: // Name input
		m.editInputs[0], cmd = m.editInputs[0].Update(msg)
	case 1: // Type selector
		var listCmd tea.Cmd
		m.typeSelect, listCmd = m.typeSelect.Update(msg)
		cmd = listCmd
	case 2: // Metadata Key input
		m.editInputs[1], cmd = m.editInputs[1].Update(msg)
	}

	return m, cmd
}

// updateFocus updates the focus state of all input fields and the type select
func (m *model) updateFocus() {
	// First blur all inputs
	for i := range m.editInputs {
		m.editInputs[i].Blur()
	}

	switch m.editFocusIndex {
	case 0: // Name input
		m.editInputs[0].Focus()
	case 1: // Type select - no input to focus, handled by typeSelect
		if m.typeSelect.Index() >= 0 && m.typeSelect.Index() < len(m.typeSelect.Items()) {
			m.typeSelect.Select(m.typeSelect.Index())
		}
	case 2: // Metadata Key input
		m.editInputs[1].Focus()
	}
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
	newMetadataKey := m.editInputs[1].Value()

	// Get the selected type from the dropdown
	var selectedType string
	if selectedItem, ok := m.typeSelect.SelectedItem().(typeItem); ok {
		switch string(selectedItem) {
		case "Artist":
			selectedType = "artist"
		case "Album":
			selectedType = "album"
		case "Playlist":
			selectedType = "playlist"
		}
	}

	if newName == "" {
		return fmt.Errorf("name cannot be empty")
	}
	if selectedType == "" {
		return fmt.Errorf("please select a valid type")
	}
	if newMetadataKey == "" {
		return fmt.Errorf("metadata key cannot be empty")
	}

	favsManager, err := config.NewFavoritesManager()
	if err != nil {
		return err
	}
	cfg, err := favsManager.Load()
	if err != nil {
		return err
	}

	// Update or add the value
	if m.editIndex == -1 {
		// Adding new item
		cfg.Items = append(cfg.Items, config.FavoriteItem{
			Name:        newName,
			Type:        selectedType,
			MetadataKey: newMetadataKey,
		})
	} else if m.editIndex < len(cfg.Items) {
		// Editing existing item
		cfg.Items[m.editIndex].Name = newName
		cfg.Items[m.editIndex].Type = selectedType
		cfg.Items[m.editIndex].MetadataKey = newMetadataKey
	} else {
		return fmt.Errorf("invalid index")
	}

	favsManager.Save(cfg)

	// Update the list
	var items []list.Item
	for _, pb := range cfg.Items {
		items = append(items, item{pb.Name, pb.Type, pb.MetadataKey})
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
		// Title
		titleStyle := lipgloss.NewStyle().Bold(true).Underline(true)
		content += titleStyle.Render(fmt.Sprintf("%s Playback Item", action)) + "\n\n"

		// Name input
		nameLabel := "Name:"
		if m.editFocusIndex == 0 {
			nameLabel = "→ " + nameLabel
		}
		content += nameLabel + "\n"
		if len(m.editInputs) > 0 {
			content += m.editInputs[0].View() + "\n\n"
		}

		// Type selection
		typeLabel := "Type:"
		if m.editFocusIndex == 1 {
			typeLabel = "→ " + typeLabel
		}
		content += typeLabel + "\n"

		// Custom type selection display
		typeOptions := []string{"Artist", "Album", "Playlist"}
		typeContent := ""
		for i, option := range typeOptions {
			itemStyle := lipgloss.NewStyle().PaddingLeft(2)
			isSelected := i == m.typeSelect.Index()

			// Always show selected item with blue highlight
			if isSelected {
				itemStyle = itemStyle.Background(lipgloss.Color("62")).Bold(true)
			}

			if i > 0 {
				typeContent += " "
			}
			typeContent += itemStyle.Render(option)
		}
		content += typeContent + "\n\n"

		// Metadata key input
		metadataLabel := "Metadata Key:"
		if m.editFocusIndex == 2 {
			metadataLabel = "→ " + metadataLabel
		}
		content += metadataLabel + "\n"
		if len(m.editInputs) > 1 {
			content += m.editInputs[1].View() + "\n"
		}
	}

	helpStyle := lipgloss.NewStyle().Foreground(lipgloss.Color("241")).Render
	content += "\n\n" + helpStyle("Enter: Save • Esc: Cancel • ↑/↓: Navigate • Tab: Switch fields")

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
		items = append(items, item{Name: pb.Name, Type: pb.Type, MetadataKey: pb.MetadataKey})
	}
	m.playbackList.SetItems(items)

	// Save to file
	favsManager.Save(m.playbackConfig)

	return nil
}

func (m *model) savePlaybackItem(name string, k string, t string) error {
	m.playbackConfig.Items = append(m.playbackConfig.Items, config.FavoriteItem{Name: name, MetadataKey: k, Type: t})
	m.playbackList.SetItems(append(m.playbackList.Items(), item{Name: name, MetadataKey: k, Type: t}))
	favsManager.Save(m.playbackConfig)
	return nil
}
