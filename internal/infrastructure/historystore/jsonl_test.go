package historystore

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/thisham/kodingin/internal/domain/history"
)

func TestRecent(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "h", "history.jsonl")
	f := New(path)
	if got, err := f.Recent(ctx, 5); err != nil || len(got) != 0 {
		t.Fatalf("missing file: %v %v", got, err)
	}
	for i := range 5 {
		if err := f.Append(ctx, history.Entry{Prompt: fmt.Sprintf("p%d", i)}); err != nil {
			t.Fatal(err)
		}
	}
	for _, tt := range []struct {
		n           int
		first, last string
		count       int
	}{{3, "p2", "p4", 3}, {10, "p0", "p4", 5}, {1, "p4", "p4", 1}} {
		got, err := f.Recent(ctx, tt.n)
		if err != nil || len(got) != tt.count || got[0].Prompt != tt.first || got[len(got)-1].Prompt != tt.last {
			t.Fatalf("n=%d: %+v %v", tt.n, got, err)
		}
	}
	if st, _ := os.Stat(path); st.Mode().Perm() != 0o600 {
		t.Fatalf("mode = %v", st.Mode().Perm())
	}
}
