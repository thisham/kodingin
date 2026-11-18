package usagestore

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/thisham/kodingin/internal/domain/usage"
)

func TestFile(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "sub", "usage.jsonl")
	f := New(path)
	if recs, err := f.Since(ctx, time.Time{}); err != nil || len(recs) != 0 {
		t.Fatalf("missing file: %v %v", recs, err)
	}
	now := time.Now()
	for i, d := range []time.Duration{-72 * time.Hour, -time.Hour, 0} {
		if err := f.Append(ctx, usage.Record{Time: now.Add(d), Model: "m", Prompt: i + 1}); err != nil {
			t.Fatal(err)
		}
	}
	appendRaw(t, path, "not json\n")
	recs, err := f.Since(ctx, now.Add(-24*time.Hour))
	if err != nil || len(recs) != 2 {
		t.Fatalf("since: %d records, err %v", len(recs), err)
	}
	if st, _ := os.Stat(path); st.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", st.Mode().Perm())
	}
}

func appendRaw(t *testing.T, path, s string) {
	fh, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0)
	if err != nil {
		t.Fatal(err)
	}
	fh.WriteString(s)
	fh.Close()
}
