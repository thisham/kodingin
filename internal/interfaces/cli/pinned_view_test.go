package cli

import "testing"

func TestVisualInputLines(t *testing.T) {
	for _, tt := range []struct {
		name        string
		value       string
		width, want int
	}{
		{"empty", "", 10, 1},
		{"exact width", "1234567890", 10, 1},
		{"wraps", "12345678901", 10, 2},
		{"explicit lines", "one\ntwo", 10, 2},
		{"unicode", "界界界", 4, 2},
		{"tabs", "123\t4", 4, 2},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := visualInputLines(tt.value, tt.width); got != tt.want {
				t.Fatalf("visualInputLines(%q, %d) = %d, want %d", tt.value, tt.width, got, tt.want)
			}
		})
	}
}
