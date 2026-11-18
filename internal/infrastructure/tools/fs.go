// Package tools implements domain tool.Tool adapters over the local file system and shell.
package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// Sandbox confines paths to the working directory.
type Sandbox struct{ Root string }

func (s Sandbox) resolve(p string) (string, error) {
	if p == "" {
		p = "."
	}
	if !filepath.IsAbs(p) {
		p = filepath.Join(s.Root, p)
	}
	p = filepath.Clean(p)
	rel, err := filepath.Rel(s.Root, p)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path %q is outside the working directory", p)
	}
	return p, nil
}

func schema(s string) json.RawMessage { return json.RawMessage(s) }

// ---- read_file
type ReadFile struct{ Sandbox }

func (ReadFile) Name() string        { return "read_file" }
func (ReadFile) Description() string { return "Read a text file. Returns content with line numbers." }
func (ReadFile) Dangerous() bool     { return false }
func (ReadFile) Schema() json.RawMessage {
	return schema(`{"type":"object","properties":{"path":{"type":"string"}},"required":["path"]}`)
}
func (t ReadFile) Run(_ context.Context, raw json.RawMessage) (string, error) {
	var a struct{ Path string }
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", err
	}
	p, err := t.resolve(a.Path)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	for i, l := range strings.Split(string(b), "\n") {
		fmt.Fprintf(&sb, "%5d  %s\n", i+1, l)
	}
	return sb.String(), nil
}

// ---- write_file
type WriteFile struct{ Sandbox }

func (WriteFile) Name() string { return "write_file" }
func (WriteFile) Description() string {
	return "Create or overwrite a file with the given content. Creates parent dirs."
}
func (WriteFile) Dangerous() bool { return true }
func (WriteFile) Schema() json.RawMessage {
	return schema(`{"type":"object","properties":{"path":{"type":"string"},"content":{"type":"string"}},"required":["path","content"]}`)
}
func (t WriteFile) Run(_ context.Context, raw json.RawMessage) (string, error) {
	var a struct{ Path, Content string }
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", err
	}
	p, err := t.resolve(a.Path)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(p, []byte(a.Content), 0o644); err != nil {
		return "", err
	}
	return fmt.Sprintf("wrote %d bytes to %s", len(a.Content), a.Path), nil
}

// ---- edit_file
type EditFile struct{ Sandbox }

func (EditFile) Name() string { return "edit_file" }
func (EditFile) Description() string {
	return "Replace an exact, unique occurrence of old_string with new_string in a file."
}
func (EditFile) Dangerous() bool { return true }
func (EditFile) Schema() json.RawMessage {
	return schema(`{"type":"object","properties":{"path":{"type":"string"},"old_string":{"type":"string"},"new_string":{"type":"string"}},"required":["path","old_string","new_string"]}`)
}
func (t EditFile) Run(_ context.Context, raw json.RawMessage) (string, error) {
	var a struct {
		Path      string `json:"path"`
		OldString string `json:"old_string"`
		NewString string `json:"new_string"`
	}
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", err
	}
	p, err := t.resolve(a.Path)
	if err != nil {
		return "", err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		return "", err
	}
	switch n := strings.Count(string(b), a.OldString); {
	case a.OldString == "" || n == 0:
		return "", fmt.Errorf("old_string not found in %s", a.Path)
	case n > 1:
		return "", fmt.Errorf("old_string occurs %d times in %s; add more context", n, a.Path)
	}
	out := strings.Replace(string(b), a.OldString, a.NewString, 1)
	return "edited " + a.Path, os.WriteFile(p, []byte(out), 0o644)
}

// ---- list_dir
type ListDir struct{ Sandbox }

func (ListDir) Name() string { return "list_dir" }
func (ListDir) Description() string {
	return "List files recursively (skips .git, vendor, node_modules). Max 500 entries."
}
func (ListDir) Dangerous() bool { return false }
func (ListDir) Schema() json.RawMessage {
	return schema(`{"type":"object","properties":{"path":{"type":"string"}}}`)
}
func (t ListDir) Run(_ context.Context, raw json.RawMessage) (string, error) {
	var a struct{ Path string }
	_ = json.Unmarshal(raw, &a)
	root, err := t.resolve(a.Path)
	if err != nil {
		return "", err
	}
	var sb strings.Builder
	n := 0
	err = filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() && (d.Name() == ".git" || d.Name() == "vendor" || d.Name() == "node_modules") {
			return filepath.SkipDir
		}
		if n >= 500 {
			return filepath.SkipAll
		}
		rel, _ := filepath.Rel(t.Root, p)
		if d.IsDir() {
			rel += "/"
		}
		sb.WriteString(rel + "\n")
		n++
		return nil
	})
	return sb.String(), err
}
