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
}

// Load reads env vars, falling back to ~/.config/kodingin/env (KEY=VALUE lines).
func Load() (Config, error) {
	loadEnvFile()
	c := Config{
		BaseURL:  get("KODINGIN_BASE_URL", "https://tokenizer.id/v1"),
		APIKey:   os.Getenv("KODINGIN_API_KEY"),
		Model:    get("KODINGIN_MODEL", "claude-sonnet-5-5"),
		MaxSteps: 50,
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

func loadEnvFile() {
	home, err := os.UserHomeDir()
	if err != nil {
		return
	}
	f, err := os.Open(filepath.Join(home, ".config", "kodingin", "env"))
	if err != nil {
		return
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
}
