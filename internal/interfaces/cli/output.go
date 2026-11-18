package cli

import (
	"io"
	"os"
	"strings"
)

// terminalOutput keeps redirected output and NO_COLOR sessions free of styling.
func terminalOutput(f *os.File) io.Writer {
	info, err := f.Stat()
	_, noColor := os.LookupEnv("NO_COLOR")
	if err != nil || info.Mode()&os.ModeCharDevice == 0 || noColor || os.Getenv("TERM") == "dumb" {
		return plainOutput{f}
	}
	return f
}

type plainOutput struct{ io.Writer }

func (w plainOutput) Write(p []byte) (int, error) {
	text := string(p)
	for _, style := range []string{reset, bold, dim, red, green, yellow, cyan, mag} {
		text = strings.ReplaceAll(text, style, "")
	}
	n, err := io.WriteString(w.Writer, text)
	if err != nil {
		return 0, err
	}
	if n != len(text) {
		return 0, io.ErrShortWrite
	}
	return len(p), nil
}

func truncate(s string, limit int) string {
	runes := []rune(s)
	if len(runes) <= limit {
		return s
	}
	return string(runes[:limit]) + "…"
}
