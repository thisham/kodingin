package config

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	BaseURL, APIKey, Model, Workdir string
	MaxSteps                        int
	AutoApprove                     bool
	// APIKeySource says where the key came from: "env" or the env file path.
	APIKeySource string
}

// MaskedKey shows only the first 4 and last 4 characters.
func (c Config) MaskedKey() string {
	k := c.APIKey
	if len(k) <= 10 {
		return strings.Repeat("*", len(k))
	}
	return k[:4] + strings.Repeat("*", 8) + k[len(k)-4:]
}

// Load reads env vars, falling back to ~/.config/kodingin/env (KEY=VALUE lines).
func Load() (Config, error) {
	src := "env KODINGIN_API_KEY"
	if os.Getenv("KODINGIN_API_KEY") == "" {
		src = loadEnvFile()
	} else {
		loadEnvFile()
	}
	c := Config{
		BaseURL:      get("KODINGIN_BASE_URL", "https://tokenizer.id/v1"),
		APIKey:       os.Getenv("KODINGIN_API_KEY"),
		Model:        get("KODINGIN_MODEL", "claude-sonnet-5-5"),
		MaxSteps:     50,
		APIKeySource: src,
	}
	if c.APIKey == "" {
		return c, errors.New("KODINGIN_API_KEY is not set (env or ~/.config/kodingin/env)")
	}
	return c, nil
}

func get(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

// loadEnvFile returns the file path it read.
func loadEnvFile() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	path := filepath.Join(home, ".config", "kodingin", "env")
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()
	s := bufio.NewScanner(f)
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok && os.Getenv(strings.TrimSpace(k)) == "" {
			os.Setenv(strings.TrimSpace(k), strings.Trim(strings.TrimSpace(v), `"'`))
		}
	}
	return path
}
