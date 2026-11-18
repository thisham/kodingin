package tools

import (
	"context"
	"encoding/json"
	"os/exec"
	"time"
)

func run(ctx context.Context, dir string, timeout time.Duration, name string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	return string(out), err
}

// ---- grep
type Grep struct{ Sandbox }

func (Grep) Name() string { return "grep" }
func (Grep) Description() string {
	return "Search file contents with a regex (grep -rnE). Optional glob include, e.g. *.go."
}
func (Grep) Dangerous() bool { return false }
func (Grep) Schema() json.RawMessage {
	return schema(`{"type":"object","properties":{"pattern":{"type":"string"},"include":{"type":"string"}},"required":["pattern"]}`)
}
func (t Grep) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var a struct{ Pattern, Include string }
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", err
	}
	args := []string{"-rnE", "--exclude-dir=.git", "--exclude-dir=vendor"}
	if a.Include != "" {
		args = append(args, "--include="+a.Include)
	}
	out, err := run(ctx, t.Root, 30*time.Second, "grep", append(args, "--", a.Pattern, ".")...)
	if err != nil && out == "" {
		return "no matches", nil
	}
	return out, nil
}

// ---- run_shell
type Shell struct{ Sandbox }

func (Shell) Name() string { return "run_shell" }
func (Shell) Description() string {
	return "Run a shell command (sh -c) in the working directory. 2 minute timeout."
}
func (Shell) Dangerous() bool { return true }
func (Shell) Schema() json.RawMessage {
	return schema(`{"type":"object","properties":{"command":{"type":"string"}},"required":["command"]}`)
}
func (t Shell) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var a struct{ Command string }
	if err := json.Unmarshal(raw, &a); err != nil {
		return "", err
	}
	return run(ctx, t.Root, 2*time.Minute, "sh", "-c", a.Command)
}

// ---- go_check: gofmt -l, go vet, go build
type GoCheck struct{ Sandbox }

func (GoCheck) Name() string { return "go_check" }
func (GoCheck) Description() string {
	return "Run gofmt -w, go vet ./... and go build ./... in the working directory."
}
func (GoCheck) Dangerous() bool         { return false }
func (GoCheck) Schema() json.RawMessage { return schema(`{"type":"object","properties":{}}`) }
func (t GoCheck) Run(ctx context.Context, _ json.RawMessage) (string, error) {
	var out string
	for _, c := range [][]string{{"gofmt", "-l", "-w", "."}, {"go", "vet", "./..."}, {"go", "build", "./..."}} {
		o, err := run(ctx, t.Root, 3*time.Minute, c[0], c[1:]...)
		out += "$ " + c[0] + " " + c[1] + "\n" + o
		if err != nil {
			return out, err
		}
	}
	return out + "OK", nil
}

// ---- go_test
type GoTest struct{ Sandbox }

func (GoTest) Name() string        { return "go_test" }
func (GoTest) Description() string { return "Run go test on a package pattern (default ./...)." }
func (GoTest) Dangerous() bool     { return false }
func (GoTest) Schema() json.RawMessage {
	return schema(`{"type":"object","properties":{"pattern":{"type":"string"},"run":{"type":"string","description":"-run regex"}}}`)
}
func (t GoTest) Run(ctx context.Context, raw json.RawMessage) (string, error) {
	var a struct{ Pattern, Run string }
	_ = json.Unmarshal(raw, &a)
	if a.Pattern == "" {
		a.Pattern = "./..."
	}
	args := []string{"test", a.Pattern}
	if a.Run != "" {
		args = append(args, "-run", a.Run)
	}
	return run(ctx, t.Root, 5*time.Minute, "go", args...)
}
