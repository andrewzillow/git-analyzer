package auth

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type TokenCache struct {
	GitHubToken string `json:"github_token"`
	GitLabToken string `json:"gitlab_token"`
}

func getCachePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %v", err)
	}
	return filepath.Join(homeDir, ".repo-analyzer-tokens.json"), nil
}

func LoadTokens() (*TokenCache, error) {
	cachePath, err := getCachePath()
	if err != nil {
		return nil, err
	}

	// Check if cache file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		return &TokenCache{}, nil
	}

	// Read cache file
	data, err := os.ReadFile(cachePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read token cache: %v", err)
	}

	var cache TokenCache
	if err := json.Unmarshal(data, &cache); err != nil {
		return nil, fmt.Errorf("failed to parse token cache: %v", err)
	}

	return &cache, nil
}

func SaveTokens(cache *TokenCache) error {
	cachePath, err := getCachePath()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cache, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal token cache: %v", err)
	}

	if err := os.WriteFile(cachePath, data, 0600); err != nil {
		return fmt.Errorf("failed to write token cache: %v", err)
	}

	return nil
}

func GetCachedToken(provider string) (string, error) {
	cache, err := LoadTokens()
	if err != nil {
		return "", err
	}

	switch provider {
	case "github":
		return cache.GitHubToken, nil
	case "gitlab":
		return cache.GitLabToken, nil
	default:
		return "", fmt.Errorf("unsupported provider: %s", provider)
	}
}

func SaveToken(provider, token string) error {
	cache, err := LoadTokens()
	if err != nil {
		return err
	}

	switch provider {
	case "github":
		cache.GitHubToken = token
	case "gitlab":
		cache.GitLabToken = token
	default:
		return fmt.Errorf("unsupported provider: %s", provider)
	}

	return SaveTokens(cache)
}
