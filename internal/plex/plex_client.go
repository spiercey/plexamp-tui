package plex

import (
	"plexamp-tui/internal/logger"
)

type PlexClient struct {
	logger *logger.Logger
}

func NewPlexClient(logger *logger.Logger) *PlexClient {
	return &PlexClient{
		logger: logger,
	}
}
