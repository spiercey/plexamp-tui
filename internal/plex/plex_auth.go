package plex

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"time"
)

// =====================
// Plex Authentication
// =====================

const (
	PlexAPIURL   = "https://plex.tv/api/v2"
	PlexClientID = "plexamp-tui-" // Will be appended with a unique identifier
	PlexProduct  = "Plexamp TUI"
	PlexVersion  = "1.0.0"
	PlexPlatform = "Linux"
	PlexDevice   = "Terminal"
)

// PlexAuthConfig stores the Plex authentication token
type PlexAuthConfig struct {
	Token     string    `json:"token"`
	Username  string    `json:"username,omitempty"`
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

// PlexPinResponse represents the PIN response from Plex
type PlexPinResponse struct {
	ID       int    `json:"id"`
	Code     string `json:"code"`
	Product  string `json:"product"`
	Trusted  bool   `json:"trusted"`
	ClientID string `json:"clientIdentifier"`
	Location struct {
		Code string `json:"code"`
	} `json:"location"`
	ExpiresIn int       `json:"expiresIn"`
	CreatedAt time.Time `json:"createdAt"`
	ExpiresAt time.Time `json:"expiresAt"`
	AuthToken string    `json:"authToken"`
}

// PlexUser represents basic user info from Plex
type PlexUser struct {
	XMLName  xml.Name `xml:"user"`
	ID       string   `xml:"id,attr"`
	Username string   `xml:"username,attr"`
	Email    string   `xml:"email,attr"`
	Title    string   `xml:"title,attr"`
}

// plexAuthConfigPath returns the path to the Plex auth config file
func plexAuthConfigPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		base = filepath.Join(home, ".config")
	}
	return filepath.Join(base, "plexamp-tui", "plex_auth.json"), nil
}

// loadPlexAuthConfig loads the Plex authentication config
func loadPlexAuthConfig() (*PlexAuthConfig, error) {
	path, err := plexAuthConfigPath()
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // No auth config exists yet
		}
		return nil, err
	}

	var config PlexAuthConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// savePlexAuthConfig saves the Plex authentication config
func savePlexAuthConfig(config *PlexAuthConfig) error {
	path, err := plexAuthConfigPath()
	if err != nil {
		return err
	}

	// Ensure directory exists
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600) // 0600 for security (token is sensitive)
}

// getClientID returns a consistent client ID for this installation
func getClientID() string {
	// In a real implementation, you might want to generate and store a UUID
	// For now, we'll use a simple identifier
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}
	return PlexClientID + hostname
}

// createPlexHeaders creates common headers for Plex API requests
func createPlexHeaders() map[string]string {
	return map[string]string{
		"X-Plex-Client-Identifier": getClientID(),
		"X-Plex-Product":           PlexProduct,
		"X-Plex-Version":           PlexVersion,
		"X-Plex-Platform":          PlexPlatform,
		"X-Plex-Device":            PlexDevice,
		"Accept":                   "application/json",
	}
}

// requestPlexPIN requests a new PIN from Plex for authentication
func requestPlexPIN() (*PlexPinResponse, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Create the request
	req, err := http.NewRequest("POST", PlexAPIURL+"/pins?strong=true", nil)
	if err != nil {
		return nil, err
	}

	// Add headers
	headers := createPlexHeaders()
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("failed to create PIN: %s - %s", resp.Status, string(body))
	}

	// Parse response
	var pinResp PlexPinResponse
	if err := json.NewDecoder(resp.Body).Decode(&pinResp); err != nil {
		return nil, err
	}

	return &pinResp, nil
}

// checkPlexPIN checks if a PIN has been authorized
func checkPlexPIN(pinID int) (*PlexPinResponse, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Create the request
	url := fmt.Sprintf("%s/pins/%d", PlexAPIURL, pinID)
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	// Add headers
	headers := createPlexHeaders()
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to check PIN: %s", resp.Status)
	}

	// Parse response
	var pinResp PlexPinResponse
	if err := json.NewDecoder(resp.Body).Decode(&pinResp); err != nil {
		return nil, err
	}

	return &pinResp, nil
}

// getPlexUser fetches the current user's information
func getPlexUser(token string) (*PlexUser, error) {
	client := &http.Client{Timeout: 10 * time.Second}

	// Create the request
	req, err := http.NewRequest("GET", "https://plex.tv/users/account", nil)
	if err != nil {
		return nil, err
	}

	// Add headers
	headers := createPlexHeaders()
	headers["X-Plex-Token"] = token
	for key, value := range headers {
		req.Header.Set(key, value)
	}

	// Make the request
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to get user info: %s", resp.Status)
	}

	// Parse XML response
	var user PlexUser
	if err := xml.NewDecoder(resp.Body).Decode(&user); err != nil {
		return nil, err
	}

	return &user, nil
}

// AuthenticateWithPlex performs the full Plex authentication flow
func (p *PlexClient) AuthenticateWithPlex() (*PlexAuthConfig, error) {
	// Request a PIN
	pin, err := requestPlexPIN()
	if err != nil {
		return nil, fmt.Errorf("failed to request PIN: %w", err)
	}

	// Build the auth URL
	authURL := fmt.Sprintf("https://app.plex.tv/auth#?clientID=%s&code=%s&context[device][product]=%s",
		url.QueryEscape(getClientID()),
		pin.Code,
		url.QueryEscape(PlexProduct))

	fmt.Println("\n=== Plex Authentication ===")
	fmt.Printf("\nPlease visit the following URL to authorize this application:\n\n")
	fmt.Printf("  %s\n\n", authURL)
	fmt.Printf("Waiting for authorization (PIN: %s)...\n", pin.Code)

	// Poll for authorization (timeout after 5 minutes)
	timeout := time.After(5 * time.Minute)
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-timeout:
			return nil, fmt.Errorf("authentication timed out")

		case <-ticker.C:
			// Check if PIN has been authorized
			updatedPin, err := checkPlexPIN(pin.ID)
			if err != nil {
				continue // Keep trying
			}

			if updatedPin.AuthToken != "" {
				fmt.Println("\n✓ Authentication successful!")

				// Get user info
				user, err := getPlexUser(updatedPin.AuthToken)
				if err != nil {
					fmt.Printf("Warning: Could not fetch user info: %v\n", err)
				}

				// Create and save config
				config := &PlexAuthConfig{
					Token:     updatedPin.AuthToken,
					ExpiresAt: time.Now().Add(365 * 24 * time.Hour), // Tokens generally don't expire
				}

				if user != nil {
					config.Username = user.Username
					fmt.Printf("Logged in as: %s\n", user.Username)
				}

				if err := savePlexAuthConfig(config); err != nil {
					return nil, fmt.Errorf("failed to save auth config: %w", err)
				}

				return config, nil
			}
		}
	}
}

// isPlexAuthenticated checks if we have a valid Plex token
func isPlexAuthenticated() bool {
	config, err := loadPlexAuthConfig()
	if err != nil || config == nil || config.Token == "" {
		return false
	}
	return true
}

// GetPlexToken returns the stored Plex token, or empty string if not authenticated
func (p *PlexClient) GetPlexToken() string {
	config, err := loadPlexAuthConfig()
	if err != nil || config == nil {
		return ""
	}
	return config.Token
}

// VerifyPlexAuthentication checks if the stored token is valid by making a test API call
func (p *PlexClient) VerifyPlexAuthentication() bool {
	token := p.GetPlexToken()
	if token == "" {
		return false
	}

	// Try to get user info to verify token is valid
	_, err := getPlexUser(token)
	return err == nil
}
