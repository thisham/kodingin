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
	// PortalCookie is the tokenizer.id portal session (Cookie header) used by /usage-credit.
	PortalCookie string
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
		PortalCookie: os.Getenv("KODINGIN_PORTAL_COOKIE"),
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

// EnvFilePath is where SaveAPIKey writes.
func EnvFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "kodingin", "env"), nil
}

// SaveAPIKey writes KODINGIN_API_KEY into the env file (mode 0600), keeping other lines.
func SaveAPIKey(key string) (string, error) {
	return SaveSettings(map[string]string{"KODINGIN_API_KEY": key})
}

// SaveSettings updates env-file values, preserving unrelated entries.
func SaveSettings(settings map[string]string) (string, error) {
	path, err := EnvFilePath()
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return "", err
	}
	values := make(map[string]string, len(settings))
	for key, value := range settings {
		values[key] = value
	}
	var lines []string
	if b, err := os.ReadFile(path); err == nil {
		for _, line := range strings.Split(strings.TrimRight(string(b), "\n"), "\n") {
			key, _, ok := strings.Cut(line, "=")
			key = strings.TrimSpace(key)
			if value, update := values[key]; ok && update {
				lines = append(lines, key+"="+value)
				delete(values, key)
				continue
			}
			if line != "" {
				lines = append(lines, line)
			}
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return "", err
	}
	for key, value := range values {
		lines = append(lines, key+"="+value)
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, []byte(strings.Join(lines, "\n")+"\n"), 0o600); err != nil {
		return "", err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return "", err
	}
	return path, os.Chmod(path, 0o600)
}

// UsageFilePath is where per-call usage history is appended.
func UsageFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "kodingin", "usage.jsonl"), nil
}

// HistoryFilePath is where sent prompts are appended.
func HistoryFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "kodingin", "history.jsonl"), nil
}
