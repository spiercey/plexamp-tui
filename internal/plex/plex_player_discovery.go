package plex

import (
	"encoding/xml"
	"fmt"
	"io"
	"net"
	"net/http"
	"sort"
	"sync"
	"time"
)

// Local players answer the Plex player protocol on this port.
const localPlayerPort = "32500"

const (
	// A sweep only probes networks no larger than a /24 so that wide interfaces
	// (docker bridges are commonly /16 or /20) can never trigger a huge scan.
	minSweepPrefixLen = 24
	sweepConcurrency  = 128
	sweepProbeTimeout = 800 * time.Millisecond
)

// playerResourceContainer is the /resources response served by Plexamp, Caldera
// and other implementations of the Plex player protocol.
type playerResourceContainer struct {
	XMLName xml.Name         `xml:"MediaContainer"`
	Players []playerResource `xml:"Player"`
}

type playerResource struct {
	MachineIdentifier    string `xml:"machineIdentifier,attr"`
	Title                string `xml:"title,attr"`
	Product              string `xml:"product,attr"`
	ProtocolCapabilities string `xml:"protocolCapabilities,attr"`
}

// GetAllPlayers returns the players registered with plex.tv merged with players
// discovered on the local network. Purely local players (such as Caldera Music)
// never register with the Plex cloud, so the sweep is the only way to see them.
func (p *PlexClient) GetAllPlayers() ([]PlexConnectionSelection, error) {
	cloudPlayers, err := p.GetPlexPlayers()
	if err != nil {
		p.logger.Debug(fmt.Sprintf("Cloud player lookup failed, falling back to local discovery: %v", err))
	}

	seen := make(map[string]struct{}, len(cloudPlayers))
	for _, player := range cloudPlayers {
		seen[player.ClientIdentifier] = struct{}{}
	}

	players := cloudPlayers
	for _, player := range p.DiscoverLocalPlayers() {
		if _, exists := seen[player.ClientIdentifier]; exists {
			continue
		}
		seen[player.ClientIdentifier] = struct{}{}
		players = append(players, player)
	}

	// Only surface the cloud error when discovery also came up empty, so a
	// plex.tv outage still leaves local players usable.
	if err != nil && len(players) == 0 {
		return nil, err
	}

	return players, nil
}

// DiscoverLocalPlayers probes every address on the machine's local networks for
// the player protocol. Plex's GDM broadcast is not used because some players
// (Caldera Music 1.1.0) only answer unicast GDM, which costs the same sweep.
func (p *PlexClient) DiscoverLocalPlayers() []PlexConnectionSelection {
	hosts := sweepHosts()
	if len(hosts) == 0 {
		p.logger.Debug("No local networks eligible for player discovery")
		return nil
	}

	p.logger.Debug(fmt.Sprintf("Probing %d local addresses for players", len(hosts)))

	client := &http.Client{Timeout: sweepProbeTimeout}

	var (
		mu      sync.Mutex
		found   []PlexConnectionSelection
		wg      sync.WaitGroup
		hostCh  = make(chan string)
		workers = min(sweepConcurrency, len(hosts))
	)

	for i := 0; i < workers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for host := range hostCh {
				players := probePlayer(client, host)
				if len(players) == 0 {
					continue
				}
				mu.Lock()
				found = append(found, players...)
				mu.Unlock()
			}
		}()
	}

	for _, host := range hosts {
		hostCh <- host
	}
	close(hostCh)
	wg.Wait()

	sort.Slice(found, func(i, j int) bool { return found[i].Name < found[j].Name })

	for _, player := range found {
		p.logger.Debug(fmt.Sprintf("Discovered local player: %s at %s", player.Name, player.Address))
	}

	return found
}

// probePlayer asks a single host for its player resources, returning nothing if
// the host is absent, silent, or serving something else.
func probePlayer(client *http.Client, host string) []PlexConnectionSelection {
	url := fmt.Sprintf("http://%s:%s/resources", host, localPlayerPort)

	resp, err := client.Get(url)
	if err != nil {
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 64<<10))
	if err != nil {
		return nil
	}

	var container playerResourceContainer
	if err := xml.Unmarshal(body, &container); err != nil {
		return nil
	}

	var players []PlexConnectionSelection
	for _, player := range container.Players {
		if player.MachineIdentifier == "" {
			continue
		}
		name := player.Title
		if name == "" {
			name = player.Product
		}
		players = append(players, PlexConnectionSelection{
			Name:             name,
			ClientIdentifier: player.MachineIdentifier,
			Scheme:           "http",
			Address:          host,
			Local:            "1",
			Port:             localPlayerPort,
			URI:              fmt.Sprintf("http://%s:%s", host, localPlayerPort),
		})
	}

	return players
}

// sweepHosts lists the addresses to probe, drawn from every up, non-loopback
// IPv4 interface carrying a network of /24 or smaller.
func sweepHosts() []string {
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil
	}

	seen := make(map[string]struct{})
	var hosts []string

	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			ipNet, ok := addr.(*net.IPNet)
			if !ok || ipNet.IP.To4() == nil {
				continue
			}
			ones, bits := ipNet.Mask.Size()
			if bits != 32 || ones < minSweepPrefixLen {
				continue
			}
			for _, host := range networkHosts(ipNet) {
				if _, exists := seen[host]; exists {
					continue
				}
				seen[host] = struct{}{}
				hosts = append(hosts, host)
			}
		}
	}

	return hosts
}

// networkHosts expands a network into its usable addresses, excluding the
// network and broadcast addresses.
func networkHosts(ipNet *net.IPNet) []string {
	base := ipNet.IP.Mask(ipNet.Mask).To4()
	if base == nil {
		return nil
	}
	mask := net.IP(ipNet.Mask).To4()
	if mask == nil {
		return nil
	}

	var hosts []string
	current := make(net.IP, len(base))
	copy(current, base)

	for {
		// Skip the network and broadcast addresses.
		if !isNetworkAddress(current, base) && !isBroadcastAddress(current, base, mask) {
			hosts = append(hosts, current.String())
		}

		next := make(net.IP, len(current))
		copy(next, current)
		incrementIP(next)
		if !ipNet.Contains(next) {
			break
		}
		current = next
	}

	return hosts
}

func isNetworkAddress(ip, base net.IP) bool {
	return ip.Equal(base)
}

func isBroadcastAddress(ip, base, mask net.IP) bool {
	broadcast := make(net.IP, len(base))
	for i := range base {
		broadcast[i] = base[i] | ^mask[i]
	}
	return ip.Equal(broadcast)
}

func incrementIP(ip net.IP) {
	for i := len(ip) - 1; i >= 0; i-- {
		ip[i]++
		if ip[i] != 0 {
			return
		}
	}
}
