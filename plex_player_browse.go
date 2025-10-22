package main

import (
	"fmt"

	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

type playerSelectMsg struct {
	success bool
	err     error
	player  playerItem
}

// playerItem represents a player in the list
type playerItem struct {
	title            string
	clientIdentifier string
	address          string
	local            string
	port             string
}

// playersFetchedMsg is a message containing fetched players
type playersFetchedMsg struct {
	players []PlexConnectionSelection
	err     error
}

// Title returns the playlist title
func (i playerItem) Title() string {
	return fmt.Sprintf("%s - %s", i.title, i.address)
}

// Description returns the playlist description (empty for now)
func (i playerItem) Description() string { return "" }

// FilterValue implements list.Item
func (i playerItem) FilterValue() string {
	return i.title + " " + i.clientIdentifier
}

// fetchPlayersCmd fetches players from the Plex server
func (m *model) fetchPlayersCmd() tea.Cmd {
	logDebug("Fetching players...")
	if m.config == nil {
		return func() tea.Msg {
			return playersFetchedMsg{err: fmt.Errorf("no config available")}
		}
	}

	token := getPlexToken()
	if token == "" {
		return func() tea.Msg {
			return playersFetchedMsg{err: fmt.Errorf("no Plex token found - run with --auth flag")}
		}
	}

	return func() tea.Msg {
		players, err := getPlexPlayers()
		return playersFetchedMsg{players: players, err: err}
	}
}

// initPlayerBrowse creates a new player browser
func (m *model) initPlayerBrowse() {
	m.panelMode = "plex-players"
	m.status = "Loading players..."

	// Create a new default delegate with custom styling
	delegate := list.NewDefaultDelegate()
	delegate.ShowDescription = false

	items := []list.Item{playerItem{title: "Loading players..."}}

	// Create the list with empty items for now
	m.playerList = list.New(items, delegate, 0, 0)
	m.playerList.Title = "Plex Players"
	m.playerList.SetShowFilter(true)
	m.playerList.SetShowStatusBar(false)
	m.playerList.SetFilteringEnabled(true)
	m.playerList.Styles.Title = titleStyle
	m.playerList.Styles.PaginationStyle = paginationStyle
	m.playerList.Styles.HelpStyle = helpStyle
	if m.width > 0 && m.height > 0 {
		m.playerList.SetSize(m.width/2-4, m.height-4)
	}
}
func (m *model) selectPlayerCmd(player playerItem) tea.Cmd {
	if m.config == nil {
		return func() tea.Msg {
			return playerSelectMsg{success: false, err: fmt.Errorf("no config available")}
		}
	}

	// Return a message with the player information
	return func() tea.Msg {
		return playerSelectMsg{
			success: true,
			player:  player,
		}
	}
}

func (m *model) handlePlayerBrowseUpdate(msg tea.Msg) (tea.Model, tea.Cmd) {
	logDebug(fmt.Sprintf("handlePlayerBrowseUpdate received message: %T", msg))

	// If we're in filtering mode, let the list handle the input
	if m.playerList.FilterState() == list.Filtering {
		var cmd tea.Cmd
		m.playerList, cmd = m.playerList.Update(msg)
		return m, cmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		key := msg.String()

		switch key {
		case "esc", "q":
			// Return to playback panel
			m.panelMode = "playback"
			m.status = ""
			return m, nil

		case "enter":
			// Select Server
			if selected, ok := m.playerList.SelectedItem().(playerItem); ok {
				logDebug(fmt.Sprintf("Selecting player: %s (clientIdentifier: %s)", selected.title, selected.clientIdentifier))
				m.lastCommand = fmt.Sprintf("Selecting %s", selected.title)
				return m, m.selectPlayerCmd(selected)
			}
			return m, nil

		case "R":
			// Refresh player list
			m.status = "Refreshing players..."
			return m, m.fetchPlayersCmd()

		default:

			// Otherwise try the common controls
			if cmd, handled := m.handleControl(key); handled {
				return m, cmd
			}
		}

	case playersFetchedMsg:
		logDebug(fmt.Sprintf("playersFetchedMsg received with %d players, error: %v", len(msg.players), msg.err))
		if msg.err != nil {
			errMsg := fmt.Sprintf("Error fetching players: %v", msg.err)
			m.status = errMsg
			logDebug(errMsg)
			return m, nil
		}

		// Convert servers to list items
		var items []list.Item
		for i, player := range msg.players {
			if i < 5 { // Only log first 5 servers to avoid log spam
				logDebug(fmt.Sprintf("Adding player %d: %s (ratingKey: %s)", i+1, player.Name, player.ClientIdentifier))
			}
			items = append(items, playerItem{
				title:            player.Name,
				clientIdentifier: player.ClientIdentifier,
				address:          player.Address,
				local:            player.Local,
				port:             player.Port,
			})
		}

		logDebug(fmt.Sprintf("Creating new list with %d items", len(items)))
		// Create a new list with the fetched items
		// Preserve the current filter state
		filterState := m.playerList.FilterState()
		filterValue := m.playerList.FilterValue()

		// Create a new default delegate with custom styling
		delegate := list.NewDefaultDelegate()
		delegate.ShowDescription = false // Don't show description

		// Create new list with existing items
		m.playerList.SetItems(items)
		m.playerList.ResetSelected()

		// Restore filter state if there was one
		if filterState == list.Filtering {
			m.playerList.ResetFilter()
			m.playerList.FilterInput.SetValue(filterValue)
		}
		m.status = fmt.Sprintf("Loaded %d players", len(msg.players))
		logDebug(fmt.Sprintf("Updated model with new player list. List has %d items", m.playerList.VisibleItems()))

		// Force a redraw
		return m, tea.Batch(tea.ClearScreen, func() tea.Msg { return nil })

	case playerSelectMsg:
		if msg.success {
			m.lastCommand = "Player Selected"
			m.status = "Player selected successfully"
		} else {
			m.lastCommand = "Player Selection Failed"
			m.status = fmt.Sprintf("Player selection error: %v", msg.err)
		}
		// Return the updated model and no command
		return m, nil
	}

	// Update the player list and get the command
	var listCmd tea.Cmd
	m.playerList, listCmd = m.playerList.Update(msg)
	// Return the current model (as a pointer) and the command
	return m, listCmd
}

// View renders the player browser
func (m *model) ViewPlayer() string {
	return m.playerList.View() + "\n" + m.status
}
