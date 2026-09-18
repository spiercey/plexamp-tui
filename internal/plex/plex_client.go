package plex

import (
	"plexamp-tui/internal/logger"
)

type PlexClient struct {
	logger *logger.Logger

	// scanLocalPlayers enables the local network sweep for players. Off by
	// default: plex.tv lists every player that checks in, and the sweep costs
	// over a second per refresh.
	scanLocalPlayers bool
}

func NewPlexClient(logger *logger.Logger) *PlexClient {
	return &PlexClient{
		logger: logger,
	}
}

// SetLocalPlayerScan enables or disables the local network sweep performed by
// GetAllPlayers.
func (p *PlexClient) SetLocalPlayerScan(enabled bool) {
	p.scanLocalPlayers = enabled
}
