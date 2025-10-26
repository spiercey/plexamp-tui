package plex

import (
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"strings"
)

//curl "https://plex.tv/api/resources?includeHttps=1&includeRelay=1&X-Plex-Token=<token>"

const (
	plexCloudBaseURL = "https://plex.tv"
)

type PlexDeviceInfo struct {
	Name                 string           `xml:"name,attr"`
	Product              string           `xml:"product,attr"`
	ProductVersion       string           `xml:"productVersion,attr"`
	Platform             string           `xml:"platform,attr"`
	PlatformVersion      string           `xml:"platformVersion,attr"`
	Device               string           `xml:"device,attr"`
	ClientIdentifier     string           `xml:"clientIdentifier,attr"`
	CreatedAt            string           `xml:"createdAt,attr"`
	LastSeenAt           string           `xml:"lastSeenAt,attr"`
	Provides             string           `xml:"provides,attr"`
	Owned                string           `xml:"owned,attr"`
	SearchEnabled        string           `xml:"searchEnabled,attr"`
	PublicAddress        string           `xml:"publicAddress,attr"`
	PublicAddressMatches string           `xml:"publicAddressMatches,attr"`
	Presence             string           `xml:"presence,attr"`
	Connections          []PlexConnection `xml:"Connection"`
}

type PlexConnection struct {
	Protocol string `xml:"protocol,attr"`
	Address  string `xml:"address,attr"`
	Port     string `xml:"port,attr"`
	URI      string `xml:"uri,attr"`
	Local    string `xml:"local,attr"`
	Relay    string `xml:"relay,attr"`
}

type PlexDeviceContainer struct {
	XMLName xml.Name         `xml:"MediaContainer"`
	Size    int              `xml:"size,attr"`
	Devices []PlexDeviceInfo `xml:"Device"`
}

type PlexConnectionSelection struct {
	Name             string `xml:"name,attr"`
	ClientIdentifier string `xml:"clientIdentifier,attr"`
	Address          string `xml:"address,attr"`
	Local            string `xml:"local,attr"`
	Port             string `xml:"port,attr"`
	URI              string `xml:"uri,attr"`
}

func (p *PlexClient) GetPlexServerInformation() ([]PlexConnectionSelection, error) {
	token := p.GetPlexToken()
	urlStr := fmt.Sprintf("%s/api/resources?includeHttps=1&includeRelay=1&X-Plex-Token=%s", plexCloudBaseURL, token)

	resp, err := http.Get(urlStr)
	if err != nil {
		p.logger.Debug(fmt.Sprintf("Request error: %v", err))
		return nil, fmt.Errorf("failed to connect to %s: %w", plexCloudBaseURL, err)
	}
	defer resp.Body.Close()

	p.logger.Debug(fmt.Sprintf("Response status: %d", resp.StatusCode))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var container PlexDeviceContainer
	if err := xml.Unmarshal(body, &container); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	var servers []PlexConnectionSelection
	for _, device := range container.Devices {
		if !strings.Contains(device.Provides, "server") {
			continue
		}
		for _, connection := range device.Connections {
			serverConnection := PlexConnectionSelection{
				Name:             device.Name,
				ClientIdentifier: device.ClientIdentifier,
				Address:          connection.Address,
				Local:            connection.Local,
				Port:             connection.Port,
				URI:              connection.URI,
			}
			servers = append(servers, serverConnection)
		}
	}

	return servers, nil
}

func (p *PlexClient) GetPlexPlayers() ([]PlexConnectionSelection, error) {
	token := p.GetPlexToken()
	urlStr := fmt.Sprintf("%s/api/resources?includeHttps=1&includeRelay=1&X-Plex-Token=%s", plexCloudBaseURL, token)

	resp, err := http.Get(urlStr)
	if err != nil {
		p.logger.Debug(fmt.Sprintf("Request error: %v", err))
		return nil, fmt.Errorf("failed to connect to %s: %w", plexCloudBaseURL, err)
	}
	defer resp.Body.Close()

	p.logger.Debug(fmt.Sprintf("Response status: %d", resp.StatusCode))

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return nil, fmt.Errorf("server returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var container PlexDeviceContainer
	if err := xml.Unmarshal(body, &container); err != nil {
		return nil, fmt.Errorf("failed to parse XML: %w", err)
	}

	var servers []PlexConnectionSelection
	for _, device := range container.Devices {
		if !strings.Contains(device.Provides, "player") {
			continue
		}
		for _, connection := range device.Connections {
			serverConnection := PlexConnectionSelection{
				Name:             device.Name,
				ClientIdentifier: device.ClientIdentifier,
				Address:          connection.Address,
				Local:            connection.Local,
				Port:             connection.Port,
				URI:              connection.URI,
			}
			servers = append(servers, serverConnection)
		}
	}

	return servers, nil
}
