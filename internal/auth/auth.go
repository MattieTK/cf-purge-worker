package auth

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"golang.org/x/term"
)

const (
	configDir  = ".config/cf-purge-worker"
	credsFile  = "credentials"
)

// Manager handles API key storage and retrieval
type Manager struct {
	configPath string
}

// NewManager creates a new auth manager
func NewManager() *Manager {
	homeDir, _ := os.UserHomeDir()
	return &Manager{
		configPath: filepath.Join(homeDir, configDir),
	}
}

// GetAPIKey retrieves the stored API key or prompts for it
func (m *Manager) GetAPIKey() (string, error) {
	// First check environment variable (for CI/CD)
	if key := os.Getenv("CLOUDFLARE_API_TOKEN"); key != "" {
		return key, nil
	}

	// Try to read from stored credentials
	key, err := m.readStoredKey()
	if err == nil && key != "" {
		return key, nil
	}

	// No stored key, prompt user
	return m.PromptForAPIKey()
}

// PromptForAPIKey prompts the user to enter their API key
func (m *Manager) PromptForAPIKey() (string, error) {
	fmt.Println("\n🔑 Cloudflare API Token required")
	fmt.Println("Create a token at: https://dash.cloudflare.com/profile/api-tokens")
	fmt.Println("\nRequired permissions:")
	fmt.Println("  • Workers Scripts: Edit")
	fmt.Println("  • Workers KV Storage: Edit")
	fmt.Println("  • Workers R2 Storage: Edit")
	fmt.Println("  • Workers D1: Edit")
	fmt.Println("  • Account Settings: Read")
	fmt.Print("\nEnter your API token: ")

	// Read password without echoing
	byteToken, err := term.ReadPassword(int(syscall.Stdin))
	fmt.Println() // New line after password input
	if err != nil {
		return "", fmt.Errorf("failed to read token: %w", err)
	}

	token := strings.TrimSpace(string(byteToken))
	if token == "" {
		return "", errors.New("token cannot be empty")
	}

	// Ask if they want to save it
	fmt.Print("\nSave this token for future use? [Y/n]: ")
	reader := bufio.NewReader(os.Stdin)
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))

	if response == "" || response == "y" || response == "yes" {
		if err := m.SaveAPIKey(token); err != nil {
			fmt.Printf("⚠️  Warning: Could not save token: %v\n", err)
		} else {
			fmt.Println("✓ Token saved securely")
		}
	}

	return token, nil
}

// SaveAPIKey saves the API key to disk
func (m *Manager) SaveAPIKey(key string) error {
	// Create config directory if it doesn't exist
	if err := os.MkdirAll(m.configPath, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write key to file with restricted permissions
	keyPath := filepath.Join(m.configPath, credsFile)
	if err := os.WriteFile(keyPath, []byte(key), 0600); err != nil {
		return fmt.Errorf("failed to write credentials: %w", err)
	}

	return nil
}

// readStoredKey reads the API key from disk
func (m *Manager) readStoredKey() (string, error) {
	keyPath := filepath.Join(m.configPath, credsFile)
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}

// DeleteStoredKey removes the stored API key
func (m *Manager) DeleteStoredKey() error {
	keyPath := filepath.Join(m.configPath, credsFile)
	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete credentials: %w", err)
	}
	return nil
}

// UpdateAPIKey prompts for a new API key and saves it
func (m *Manager) UpdateAPIKey() error {
	// Delete old key first
	_ = m.DeleteStoredKey()

	// Prompt for new key
	key, err := m.PromptForAPIKey()
	if err != nil {
		return err
	}

	// Save it
	return m.SaveAPIKey(key)
}
