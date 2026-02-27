package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mark3labs/mcp-go/client/transport"
)

const (
	configDir       = ".config/notion-cli"
	configFile      = "config.json"
	legacyTokenFile = "token.json"
	accountsDir     = "accounts"
	defaultAccount  = "default"
)

var (
	ErrNoToken        = errors.New("no token available")
	ErrInvalidAccount = errors.New("invalid account name")

	accountNameRe = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@+-]*$`)
)

type FileTokenStore struct {
	homeDir string
	account string
	path    string
	mu      sync.RWMutex
}

func NewFileTokenStore() (*FileTokenStore, error) {
	return NewFileTokenStoreForAccount("")
}

func NewFileTokenStoreForAccount(account string) (*FileTokenStore, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	resolvedAccount, err := resolveAccountNameForHome(homeDir, account)
	if err != nil {
		return nil, err
	}

	store := &FileTokenStore{
		homeDir: homeDir,
		account: resolvedAccount,
		path:    accountTokenPath(homeDir, resolvedAccount),
	}

	if err := store.migrateLegacyDefaultIfNeeded(); err != nil {
		return nil, err
	}

	return store, nil
}

func ResolveAccountName(account string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return resolveAccountNameForHome(homeDir, account)
}

func GetActiveAccount() (string, error) {
	return ResolveAccountName("")
}

func SetActiveAccount(account string) error {
	normalized := strings.TrimSpace(account)
	if err := ValidateAccountName(normalized); err != nil {
		return err
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	cfg, err := readCLIConfig(homeDir)
	if err != nil {
		return err
	}
	cfg.ActiveAccount = normalized

	return writeCLIConfig(homeDir, cfg)
}

func ListAccounts() ([]string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	accountSet := map[string]struct{}{}

	cfg, err := readCLIConfig(homeDir)
	if err != nil {
		return nil, err
	}
	if cfg.ActiveAccount != "" {
		if err := ValidateAccountName(cfg.ActiveAccount); err != nil {
			return nil, fmt.Errorf("configured active account: %w", err)
		}
		accountSet[cfg.ActiveAccount] = struct{}{}
	}

	entries, err := os.ReadDir(accountsDirectoryPath(homeDir))
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !strings.HasSuffix(name, ".json") {
			continue
		}
		account := strings.TrimSuffix(name, ".json")
		if err := ValidateAccountName(account); err != nil {
			continue
		}
		accountSet[account] = struct{}{}
	}

	if _, err := os.Stat(legacyTokenPath(homeDir)); err == nil {
		accountSet[defaultAccount] = struct{}{}
	} else if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	accounts := make([]string, 0, len(accountSet))
	for account := range accountSet {
		accounts = append(accounts, account)
	}
	sort.Strings(accounts)
	return accounts, nil
}

func ClearAllTokens() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	if err := os.RemoveAll(accountsDirectoryPath(homeDir)); err != nil && !os.IsNotExist(err) {
		return err
	}

	if err := os.Remove(legacyTokenPath(homeDir)); err != nil && !os.IsNotExist(err) {
		return err
	}

	return nil
}

func ValidateAccountName(account string) error {
	if account == "" {
		return fmt.Errorf("%w: empty", ErrInvalidAccount)
	}
	if !accountNameRe.MatchString(account) {
		return fmt.Errorf("%w: %q (allowed: letters, numbers, ., _, -, @, +)", ErrInvalidAccount, account)
	}
	return nil
}

func (s *FileTokenStore) Account() string {
	return s.account
}

func (s *FileTokenStore) GetToken(ctx context.Context) (*transport.Token, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	stored, err := s.readStoredTokenUnlocked()
	if err != nil {
		return nil, err
	}

	return &transport.Token{
		AccessToken:  stored.AccessToken,
		TokenType:    stored.TokenType,
		RefreshToken: stored.RefreshToken,
		ExpiresAt:    stored.ExpiresAt,
	}, nil
}

func (s *FileTokenStore) SaveToken(ctx context.Context, token *transport.Token) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}

	var existing storedToken
	if current, err := s.readStoredTokenUnlocked(); err == nil {
		existing = current
	} else if err != nil && !errors.Is(err, ErrNoToken) {
		return err
	}

	stored := storedToken{
		AccessToken:  token.AccessToken,
		TokenType:    token.TokenType,
		RefreshToken: token.RefreshToken,
		ExpiresAt:    token.ExpiresAt,
		SavedAt:      time.Now(),
		ClientID:     existing.ClientID,
	}

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0600)
}

func (s *FileTokenStore) Clear() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.Remove(s.path); err != nil && !os.IsNotExist(err) {
		return err
	}

	if s.account == defaultAccount {
		if err := os.Remove(legacyTokenPath(s.homeDir)); err != nil && !os.IsNotExist(err) {
			return err
		}
	}

	return nil
}

func (s *FileTokenStore) Path() string {
	return s.path
}

type storedToken struct {
	AccessToken  string    `json:"access_token"`
	TokenType    string    `json:"token_type"`
	RefreshToken string    `json:"refresh_token,omitempty"`
	ExpiresAt    time.Time `json:"expires_at,omitempty"`
	SavedAt      time.Time `json:"saved_at,omitempty"`
	ClientID     string    `json:"client_id,omitempty"`
}

func (s *FileTokenStore) GetClientID(ctx context.Context) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	stored, err := s.readStoredTokenUnlocked()
	if err != nil {
		if errors.Is(err, ErrNoToken) {
			return "", nil
		}
		return "", err
	}

	return stored.ClientID, nil
}

func (s *FileTokenStore) SaveClientID(ctx context.Context, clientID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}

	var stored storedToken
	if current, err := s.readStoredTokenUnlocked(); err == nil {
		stored = current
	} else if err != nil && !errors.Is(err, ErrNoToken) {
		return err
	}

	stored.ClientID = clientID

	data, err := json.MarshalIndent(stored, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.path, data, 0600)
}

func (s *FileTokenStore) readStoredTokenUnlocked() (storedToken, error) {
	paths := []string{s.path}
	if s.account == defaultAccount {
		paths = append(paths, legacyTokenPath(s.homeDir))
	}

	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return storedToken{}, err
		}

		var stored storedToken
		if err := json.Unmarshal(data, &stored); err != nil {
			return storedToken{}, err
		}
		return stored, nil
	}

	return storedToken{}, ErrNoToken
}

func (s *FileTokenStore) migrateLegacyDefaultIfNeeded() error {
	if s.account != defaultAccount {
		return nil
	}

	if _, err := os.Stat(s.path); err == nil {
		return nil
	} else if err != nil && !os.IsNotExist(err) {
		return err
	}

	legacyPath := legacyTokenPath(s.homeDir)
	legacyData, err := os.ReadFile(legacyPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	if err := os.MkdirAll(filepath.Dir(s.path), 0700); err != nil {
		return err
	}

	return os.WriteFile(s.path, legacyData, 0600)
}

type cliConfig struct {
	ActiveAccount string `json:"active_account,omitempty"`
}

func resolveAccountNameForHome(homeDir, account string) (string, error) {
	normalized := strings.TrimSpace(account)
	if normalized != "" {
		if err := ValidateAccountName(normalized); err != nil {
			return "", err
		}
		return normalized, nil
	}

	cfg, err := readCLIConfig(homeDir)
	if err != nil {
		return "", err
	}

	if cfg.ActiveAccount != "" {
		if err := ValidateAccountName(cfg.ActiveAccount); err != nil {
			return "", fmt.Errorf("configured active account: %w", err)
		}
		return cfg.ActiveAccount, nil
	}

	return defaultAccount, nil
}

func readCLIConfig(homeDir string) (cliConfig, error) {
	path := cliConfigPath(homeDir)
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cliConfig{}, nil
		}
		return cliConfig{}, err
	}

	var cfg cliConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return cliConfig{}, err
	}
	return cfg, nil
}

func writeCLIConfig(homeDir string, cfg cliConfig) error {
	path := cliConfigPath(homeDir)
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}

	merged := map[string]any{}
	if existing, err := os.ReadFile(path); err == nil {
		if len(existing) > 0 {
			if err := json.Unmarshal(existing, &merged); err != nil {
				return err
			}
		}
	} else if !os.IsNotExist(err) {
		return err
	}
	merged["active_account"] = cfg.ActiveAccount

	data, err := json.MarshalIndent(merged, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0600)
}

func cliConfigPath(homeDir string) string {
	return filepath.Join(homeDir, configDir, configFile)
}

func accountsDirectoryPath(homeDir string) string {
	return filepath.Join(homeDir, configDir, accountsDir)
}

func accountTokenPath(homeDir, account string) string {
	return filepath.Join(accountsDirectoryPath(homeDir), account+".json")
}

func legacyTokenPath(homeDir string) string {
	return filepath.Join(homeDir, configDir, legacyTokenFile)
}
