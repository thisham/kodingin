// Package historystore keeps prompt history as JSON lines in a private local file.
package historystore

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"

	"github.com/thisham/kodingin/internal/domain/history"
)

type File struct {
	Path string
	mu   sync.Mutex
}

func New(path string) *File { return &File{Path: path} }

// Append adds one line; the file is created with mode 0600 and only appended to.
func (f *File) Append(_ context.Context, e history.Entry) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(e)
	if err != nil {
		return err
	}
	fh, err := os.OpenFile(f.Path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := fh.Write(append(b, '\n')); err != nil {
		fh.Close()
		return err
	}
	return fh.Close()
}

// Recent returns the last n entries, oldest first. Malformed lines are skipped.
func (f *File) Recent(_ context.Context, n int) ([]history.Entry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	fh, err := os.Open(f.Path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer fh.Close()
	var ring []history.Entry
	s := bufio.NewScanner(fh)
	s.Buffer(make([]byte, 64*1024), 8*1024*1024)
	for s.Scan() {
		var e history.Entry
		if json.Unmarshal(s.Bytes(), &e) != nil {
			continue
		}
		ring = append(ring, e)
		if len(ring) > n {
			ring = ring[1:]
		}
	}
	return ring, s.Err()
}
