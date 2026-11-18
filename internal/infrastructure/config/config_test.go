package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestSaveSettingsPreservesOtherValuesAndPermissions(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	path := filepath.Join(home, ".config", "kodingin", "env")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	original := "# existing\nKODINGIN_API_KEY=secret\nKODINGIN_MODEL=old-model\nOTHER=value\n"
	if err := os.WriteFile(path, []byte(original), 0o600); err != nil {
		t.Fatal(err)
	}

	gotPath, err := SaveSettings(map[string]string{
		"KODINGIN_BASE_URL": "https://example.test/v1",
		"KODINGIN_MODEL":    "new-model",
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != path {
		t.Fatalf("path = %q, want %q", gotPath, path)
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(got)
	for _, want := range []string{"# existing", "KODINGIN_API_KEY=secret", "KODINGIN_MODEL=new-model", "OTHER=value", "KODINGIN_BASE_URL=https://example.test/v1"} {
		if !strings.Contains(text, want) {
			t.Errorf("saved env missing %q: %s", want, text)
		}
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %o, want 600", info.Mode().Perm())
	}
}
