// Package sessionstore keeps one JSON file per session in a private directory.
package sessionstore

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/thisham/kodingin/internal/domain/session"
)

var validID = regexp.MustCompile(`^[0-9A-Za-z-]{1,64}$`)

type Dir struct{ Path string }

func New(path string) *Dir { return &Dir{Path: path} }

func (d *Dir) file(id string) (string, error) {
	if !validID.MatchString(id) {
		return "", session.ErrNotFound
	}
	return filepath.Join(d.Path, id+".json"), nil
}

// Save writes atomically (temp file + rename) with mode 0600.
func (d *Dir) Save(_ context.Context, s session.Session) error {
	path, err := d.file(s.ID)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(d.Path, 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(s)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(d.Path, ".tmp-*")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if err := tmp.Chmod(0o600); err != nil {
		tmp.Close()
		return err
	}
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func (d *Dir) Load(_ context.Context, id string) (session.Session, error) {
	path, err := d.file(id)
	if err != nil {
		return session.Session{}, err
	}
	b, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return session.Session{}, session.ErrNotFound
	}
	if err != nil {
		return session.Session{}, err
	}
	var s session.Session
	if err := json.Unmarshal(b, &s); err != nil {
		return session.Session{}, err
	}
	return s, nil
}

// List reads the newest files by modification time; unreadable files are skipped.
func (d *Dir) List(ctx context.Context, limit int) ([]session.Session, error) {
	ents, err := os.ReadDir(d.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	type f struct {
		id  string
		mod int64
	}
	var files []f
	for _, e := range ents {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".json") || strings.HasPrefix(name, ".") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			continue
		}
		files = append(files, f{strings.TrimSuffix(name, ".json"), info.ModTime().UnixNano()})
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mod > files[j].mod })
	var out []session.Session
	for _, fl := range files {
		if len(out) == limit {
			break
		}
		s, err := d.Load(ctx, fl.id)
		if err != nil || len(s.Messages) == 0 {
			continue
		}
		out = append(out, s)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Updated.After(out[j].Updated) })
	return out, nil
}
