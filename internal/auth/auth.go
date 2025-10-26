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
	configDir     = ".config/cf-delete-worker"
	credsFile     = "credentials"
	emailFile     = "email"
)

// Credentials holds API authentication info
type Credentials struct {
	Token string // API Token or Global API Key
	Email string // Email address (only for Global API Key)
}

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

// GetCredentials retrieves the stored credentials or prompts for them
func (m *Manager) GetCredentials() (*Credentials, error) {
	// First check environment variables (for CI/CD)
	if token := os.Getenv("CLOUDFLARE_API_TOKEN"); token != "" {
		return &Credentials{Token: token}, nil
	}

	// Check for Global API Key in environment
	if key := os.Getenv("CLOUDFLARE_API_KEY"); key != "" {
		email := os.Getenv("CLOUDFLARE_EMAIL")
		if email == "" {
			return nil, errors.New("CLOUDFLARE_API_KEY requires CLOUDFLARE_EMAIL to be set")
		}
		return &Credentials{Token: key, Email: email}, nil
	}

	// Try to read from stored credentials
	creds, err := m.readStoredCredentials()
	if err == nil && creds.Token != "" {
		return creds, nil
	}

	// No stored credentials, prompt user
	return m.PromptForCredentials()
}

// GetAPIKey retrieves just the API token (for backward compatibility)
func (m *Manager) GetAPIKey() (string, error) {
	creds, err := m.GetCredentials()
	if err != nil {
		return "", err
	}
	return creds.Token, nil
}

// PromptForCredentials prompts the user to enter their API credentials
func (m *Manager) PromptForCredentials() (*Credentials, error) {
	fmt.Println("\n🔑 Cloudflare API Authentication")
	fmt.Println("\nChoose authentication method:")
	fmt.Println("  1. API Token (recommended)")
	fmt.Println("     Create at: https://dash.cloudflare.com/profile/api-tokens")
	fmt.Println("  2. Global API Key (legacy)")
	fmt.Println("     Find at: https://dash.cloudflare.com/profile/api-tokens")

	reader := bufio.NewReader(os.Stdin)
	fmt.Print("\nSelect method [1/2]: ")
	methodChoice, _ := reader.ReadString('\n')
	methodChoice = strings.TrimSpace(methodChoice)

	var creds Credentials

	if methodChoice == "2" {
		// Global API Key method
		fmt.Println("\n📧 Global API Key Authentication")
		fmt.Print("Enter your email address: ")
		email, _ := reader.ReadString('\n')
		creds.Email = strings.TrimSpace(email)

		if creds.Email == "" {
			return nil, errors.New("email cannot be empty")
		}

		fmt.Print("Enter your Global API Key: ")
		byteKey, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return nil, fmt.Errorf("failed to read API key: %w", err)
		}

		creds.Token = strings.TrimSpace(string(byteKey))
		if creds.Token == "" {
			return nil, errors.New("API key cannot be empty")
		}
	} else {
		// API Token method (default)
		fmt.Println("\n🔐 API Token Authentication")
		fmt.Println("Required permissions:")
		fmt.Println("  • Workers Scripts: Edit")
		fmt.Println("  • Workers KV Storage: Edit")
		fmt.Println("  • Workers R2 Storage: Edit")
		fmt.Println("  • Workers D1: Edit")
		fmt.Println("  • Account Settings: Read")
		fmt.Print("\nEnter your API token: ")

		byteToken, err := term.ReadPassword(int(syscall.Stdin))
		fmt.Println()
		if err != nil {
			return nil, fmt.Errorf("failed to read token: %w", err)
		}

		creds.Token = strings.TrimSpace(string(byteToken))
		if creds.Token == "" {
			return nil, errors.New("token cannot be empty")
		}
	}

	// Ask if they want to save it
	fmt.Print("\nSave credentials for future use? [Y/n]: ")
	response, _ := reader.ReadString('\n')
	response = strings.ToLower(strings.TrimSpace(response))

	if response == "" || response == "y" || response == "yes" {
		if err := m.SaveCredentials(&creds); err != nil {
			fmt.Printf("⚠️  Warning: Could not save credentials: %v\n", err)
		} else {
			fmt.Println("✓ Credentials saved securely")
		}
	}

	return &creds, nil
}

// PromptForAPIKey prompts the user to enter their API key (backward compatibility)
func (m *Manager) PromptForAPIKey() (string, error) {
	creds, err := m.PromptForCredentials()
	if err != nil {
		return "", err
	}
	return creds.Token, nil
}

// SaveCredentials saves the credentials to disk
func (m *Manager) SaveCredentials(creds *Credentials) error {
	// Create config directory if it doesn't exist
	if err := os.MkdirAll(m.configPath, 0700); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Write token to file with restricted permissions
	keyPath := filepath.Join(m.configPath, credsFile)
	if err := os.WriteFile(keyPath, []byte(creds.Token), 0600); err != nil {
		return fmt.Errorf("failed to write credentials: %w", err)
	}

	// Write email if present (for Global API Key)
	if creds.Email != "" {
		emailPath := filepath.Join(m.configPath, emailFile)
		if err := os.WriteFile(emailPath, []byte(creds.Email), 0600); err != nil {
			return fmt.Errorf("failed to write email: %w", err)
		}
	} else {
		// Remove email file if it exists (switching from Global API Key to Token)
		emailPath := filepath.Join(m.configPath, emailFile)
		_ = os.Remove(emailPath)
	}

	return nil
}

// SaveAPIKey saves the API key to disk (backward compatibility)
func (m *Manager) SaveAPIKey(key string) error {
	return m.SaveCredentials(&Credentials{Token: key})
}

// readStoredCredentials reads the credentials from disk
func (m *Manager) readStoredCredentials() (*Credentials, error) {
	keyPath := filepath.Join(m.configPath, credsFile)
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return nil, err
	}

	creds := &Credentials{
		Token: strings.TrimSpace(string(data)),
	}

	// Check if email file exists (for Global API Key)
	emailPath := filepath.Join(m.configPath, emailFile)
	emailData, err := os.ReadFile(emailPath)
	if err == nil {
		creds.Email = strings.TrimSpace(string(emailData))
	}

	return creds, nil
}

// readStoredKey reads the API key from disk (backward compatibility)
func (m *Manager) readStoredKey() (string, error) {
	creds, err := m.readStoredCredentials()
	if err != nil {
		return "", err
	}
	return creds.Token, nil
}

// DeleteStoredKey removes the stored credentials
func (m *Manager) DeleteStoredKey() error {
	keyPath := filepath.Join(m.configPath, credsFile)
	if err := os.Remove(keyPath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete credentials: %w", err)
	}

	// Also delete email file if it exists
	emailPath := filepath.Join(m.configPath, emailFile)
	_ = os.Remove(emailPath)

	return nil
}

// UpdateAPIKey prompts for new credentials and saves them
func (m *Manager) UpdateAPIKey() error {
	// Delete old credentials first
	_ = m.DeleteStoredKey()

	// Prompt for new credentials
	creds, err := m.PromptForCredentials()
	if err != nil {
		return err
	}

	// Save them
	return m.SaveCredentials(creds)
}
