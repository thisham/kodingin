// Package usagestore keeps usage records as JSON lines in a private local file.
package usagestore

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/thisham/kodingin/internal/domain/usage"
)

type File struct {
	Path string
	mu   sync.Mutex
}

func New(path string) *File { return &File{Path: path} }

// Append adds one line. The file is created with mode 0600 and only ever appended to.
func (f *File) Append(_ context.Context, r usage.Record) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := os.MkdirAll(filepath.Dir(f.Path), 0o700); err != nil {
		return err
	}
	b, err := json.Marshal(r)
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

// Since reads records at or after t. Malformed lines are skipped, not fatal.
func (f *File) Since(_ context.Context, t time.Time) ([]usage.Record, error) {
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
	var out []usage.Record
	s := bufio.NewScanner(fh)
	s.Buffer(make([]byte, 64*1024), 1024*1024)
	for s.Scan() {
		var r usage.Record
		if json.Unmarshal(s.Bytes(), &r) == nil && !r.Time.Before(t) {
			out = append(out, r)
		}
	}
	return out, s.Err()
}
