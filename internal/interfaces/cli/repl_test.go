package cli

import (
	"bufio"
	"bytes"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/thisham/kodingin/internal/domain/conversation"
)

func TestApprove(t *testing.T) {
	for _, tt := range []struct {
		name, input  string
		want, always bool
	}{
		{"yes", "y\n", true, false},
		{"default denies", "\n", false, false},
		{"no", "no\n", false, false},
		{"always", "a\n", true, true},
		{"invalid retries", "maybe\ny\n", true, false},
		{"eof denies", "", false, false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			r := &REPL{in: bufio.NewReader(strings.NewReader(tt.input)), out: &bytes.Buffer{}}
			if got := r.Approve(conversation.ToolCall{Name: "write_file"}); got != tt.want {
				t.Fatalf("Approve = %v, want %v", got, tt.want)
			}
			if r.autoApprove != tt.always {
				t.Fatalf("autoApprove = %v, want %v", r.autoApprove, tt.always)
			}
		})
	}
}

func TestPlainOutput(t *testing.T) {
	var b bytes.Buffer
	text := bold + cyan + "kodingin 日本語" + reset + "\n"
	n, err := (plainOutput{&b}).Write([]byte(text))
	if err != nil || n != len(text) {
		t.Fatalf("Write = %d, %v", n, err)
	}
	if got := b.String(); got != "kodingin 日本語\n" {
		t.Fatalf("output = %q", got)
	}
}

func TestTruncate(t *testing.T) {
	for _, tt := range []struct {
		input string
		limit int
		want  string
	}{
		{"abc", 3, "abc"}, {"日本語abc", 3, "日本語…"}, {"", 1, ""},
	} {
		t.Run(tt.input, func(t *testing.T) {
			got := truncate(tt.input, tt.limit)
			if got != tt.want || !utf8.ValidString(got) {
				t.Fatalf("truncate = %q, want %q", got, tt.want)
			}
		})
	}
}
