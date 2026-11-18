package tools

import (
	"context"
	"errors"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestCappedOutput(t *testing.T) {
	for _, size := range []int{0, 12, maxCommandOutput, maxCommandOutput + 1, maxCommandOutput * 4} {
		t.Run(strconv.Itoa(size), func(t *testing.T) {
			var out cappedOutput
			input := strings.Repeat("x", size)
			for start := 0; start < len(input); {
				end := min(start+777, len(input))
				n, err := out.Write([]byte(input[start:end]))
				if err != nil || n != end-start {
					t.Fatalf("Write = %d, %v", n, err)
				}
				start = end
			}
			if out.buf.String() != input[:min(size, maxCommandOutput)] {
				t.Fatal("retained output differs")
			}
			if out.discarded != int64(max(0, size-maxCommandOutput)) {
				t.Fatalf("discarded = %d", out.discarded)
			}
		})
	}
}

func TestRunOutput(t *testing.T) {
	for _, tt := range []struct {
		name, command, want string
		exit                int
	}{
		{"both streams", "printf stdout; printf stderr >&2", "stdoutstderr", 0},
		{"error output", "printf failure >&2; exit 7", "failure", 7},
		{"drains oversized output", "head -c 262144 /dev/zero; printf done >&2", "\n...[truncated 196612 bytes]", 0},
	} {
		t.Run(tt.name, func(t *testing.T) {
			out, err := run(context.Background(), t.TempDir(), 5*time.Second, "sh", "-c", tt.command)
			if tt.exit == 0 && err != nil {
				t.Fatal(err)
			}
			if tt.exit != 0 {
				var exit *exec.ExitError
				if !errors.As(err, &exit) || exit.ExitCode() != tt.exit {
					t.Fatalf("error = %v", err)
				}
			}
			if tt.name == "drains oversized output" {
				if len(out) != maxCommandOutput+len(tt.want) || !strings.HasSuffix(out, tt.want) {
					t.Fatalf("output length = %d", len(out))
				}
			} else if out != tt.want {
				t.Fatalf("output = %q", out)
			}
		})
	}
}
