// Package cli is the terminal delivery layer: REPL, colours, approval prompts.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"time"

	"github.com/thisham/kodingin/internal/application"
	"github.com/thisham/kodingin/internal/domain/conversation"
	"github.com/thisham/kodingin/internal/domain/llm"
	"github.com/thisham/kodingin/internal/version"
)

const (
	reset  = "\033[0m"
	bold   = "\033[1m"
	dim    = "\033[2m"
	red    = "\033[31m"
	green  = "\033[32m"
	yellow = "\033[33m"
	cyan   = "\033[36m"
	mag    = "\033[35m"
)

type REPL struct {
	agent           *application.Agent
	in              *bufio.Reader
	out             io.Writer
	autoApprove     bool
	model           string
	workdir         string
	tokIn           int
	tokOut          int
	keyMasked       string
	keySource       string
	baseURL         string
	saveKey         func(key string) (string, error)
	turnIn, turnOut int
	turnStarted     time.Time
}

// SetKeySaver injects how /api-key-set persists a key (infrastructure concern).
func (r *REPL) SetKeySaver(f func(string) (string, error)) { r.saveKey = f }

// SetKeyInfo gives the REPL what /api-key displays. The raw key never reaches the UI.
func (r *REPL) SetKeyInfo(masked, source, baseURL string) {
	r.keyMasked, r.keySource, r.baseURL = masked, source, baseURL
}

func New(a *application.Agent, model, workdir string, autoApprove bool) *REPL {
	return &REPL{agent: a, in: bufio.NewReader(os.Stdin), out: terminalOutput(os.Stdout), autoApprove: autoApprove, model: model, workdir: workdir}
}

func (r *REPL) banner() {
	fmt.Fprintf(r.out, "\n%s%skodingin%s %sv%s%s\n", bold, mag, reset, dim, version.Current, reset)
	fmt.Fprintf(r.out, "  %sGo coding agent · explore, edit, verify%s\n\n", dim, reset)
	r.status()
	fmt.Fprintf(r.out, "\n  %s/help for commands · trailing \\ for multiline · Ctrl-C cancels a turn%s\n\n", dim, reset)
}

func (r *REPL) status() {
	mode := "ask before writes and shell"
	if r.autoApprove {
		mode = "auto-approve writes and shell"
	}
	fmt.Fprintf(r.out, "  Model      %s\n  Workspace  %s\n  Approval   %s\n  Session    %d input / %d output tokens\n", r.model, r.workdir, mode, r.tokIn, r.tokOut)
}

func (r *REPL) help() {
	fmt.Fprintf(r.out, "\n%sConversation%s\n  /clear             Reset conversation and token counts\n  /status            Show model, workspace, approvals and usage\n  /version           Show application version\n  /exit              Quit (also /quit)\n\n%sConfiguration%s\n  /model [id]        List models or switch model\n  /api-key           Show masked key and check access\n  /api-key-set       Save a key using hidden input\n  /yolo              Toggle approval prompts\n\n%sInput%s\n  End a line with \\ to continue on the next line.\n  Ctrl-C cancels the current turn. Ctrl-D exits.\n\n", bold, reset, bold, reset, bold, reset)
}

func (r *REPL) beginTurn() {
	r.turnIn, r.turnOut = 0, 0
	r.turnStarted = time.Now()
}

// RunOnce executes a single prompt non-interactively.
func (r *REPL) RunOnce(prompt string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	r.beginTurn()
	err := r.agent.Ask(ctx, prompt, r)
	r.footer()
	return err
}

func (r *REPL) Run() error {
	r.banner()
	for {
		fmt.Fprintf(r.out, "%s%s❯%s ", bold, green, reset)
		line, err := r.readInput()
		if err == io.EOF {
			fmt.Fprintln(r.out)
			return nil
		}
		if err != nil {
			return fmt.Errorf("read input: %w", err)
		}
		line = strings.TrimSpace(line)
		if line == "/api-key-set" || strings.HasPrefix(line, "/api-key-set ") {
			r.apiKeySetCmd(strings.TrimSpace(strings.TrimPrefix(line, "/api-key-set")))
			continue
		}
		if line == "/model" || strings.HasPrefix(line, "/model ") {
			r.modelCmd(strings.TrimSpace(strings.TrimPrefix(line, "/model")))
			continue
		}
		switch line {
		case "":
			continue
		case "/exit", "/quit":
			return nil
		case "/clear":
			r.agent.Reset()
			r.tokIn, r.tokOut = 0, 0
			fmt.Fprintf(r.out, "%sconversation cleared%s\n", dim, reset)
			continue
		case "/api-key":
			r.apiKeyCmd()
			continue
		case "/yolo":
			r.autoApprove = !r.autoApprove
			fmt.Fprintf(r.out, "%sauto-approve: %v%s\n", yellow, r.autoApprove, reset)
			continue
		case "/status":
			r.status()
			continue
		case "/version":
			fmt.Fprintln(r.out, "kodingin v"+version.Current)
			continue
		case "/help":
			r.help()
			continue
		}
		if strings.HasPrefix(line, "/") {
			fmt.Fprintf(r.out, "%sUnknown command %q. Use /help to see commands.%s\n", yellow, strings.Fields(line)[0], reset)
			continue
		}
		r.beginTurn()
		ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
		fmt.Fprintf(r.out, "%sthinking…%s\n", dim, reset)
		if err := r.agent.Ask(ctx, line, r); err != nil {
			fmt.Fprintf(r.out, "%s✗ %v%s\n", red, err, reset)
		}
		stop()
		r.footer()
	}
}

// readInput supports multi-line input with trailing backslash.
func (r *REPL) readInput() (string, error) {
	var sb strings.Builder
	for {
		l, err := r.in.ReadString('\n')
		if err != nil && l == "" {
			return sb.String(), err
		}
		l = strings.TrimRight(l, "\r\n")
		if strings.HasSuffix(l, "\\") {
			sb.WriteString(strings.TrimSuffix(l, "\\") + "\n")
			fmt.Fprintf(r.out, "%s…%s ", dim, reset)
			continue
		}
		sb.WriteString(l)
		return sb.String(), nil
	}
}

func (r *REPL) footer() {
	fmt.Fprintf(r.out, "%s── %s · tokens in %d / out %d · session %d / %d%s\n\n", dim, time.Since(r.turnStarted).Round(time.Millisecond), r.turnIn, r.turnOut, r.tokIn, r.tokOut, reset)
}

// ---- application.Observer

func (r *REPL) OnAssistantText(text string) { fmt.Fprintf(r.out, "\n%s\n\n", text) }

func (r *REPL) OnToolCall(c conversation.ToolCall) {
	fmt.Fprintf(r.out, "%s⏺ %s%s %s%s%s\n", cyan, c.Name, reset, dim, summarize(c.Arguments), reset)
}

func (r *REPL) OnToolResult(c conversation.ToolCall, out string, err error) {
	if err != nil {
		fmt.Fprintf(r.out, "  %s⎿ %v%s\n", red, err, reset)
	}
	lines := strings.Split(strings.TrimRight(out, "\n"), "\n")
	if out == "" {
		return
	}
	show := lines
	if len(show) > 6 {
		show = show[:6]
	}
	for _, l := range show {
		if len([]rune(l)) > 120 {
			l = truncate(l, 120)
		}
		fmt.Fprintf(r.out, "  %s⎿ %s%s\n", dim, l, reset)
	}
	if len(lines) > 6 {
		fmt.Fprintf(r.out, "  %s  … %d more lines%s\n", dim, len(lines)-6, reset)
	}
}

func (r *REPL) Approve(c conversation.ToolCall) bool {
	if r.autoApprove {
		return true
	}
	r.preview(c)
	for {
		fmt.Fprintf(r.out, "  %sAllow %s? [y]es / [N]o / [a]lways:%s ", yellow, c.Name, reset)
		ans, err := r.in.ReadString('\n')
		if err != nil {
			return false
		}
		switch strings.ToLower(strings.TrimSpace(ans)) {
		case "y", "yes":
			return true
		case "", "n", "no":
			return false
		case "a", "always":
			r.autoApprove = true
			fmt.Fprintln(r.out, "  Auto-approve enabled for this session. Use /yolo to disable.")
			return true
		default:
			fmt.Fprintln(r.out, "  Enter y, n, or a.")
		}
	}
}

func (r *REPL) OnUsage(u llm.Usage) {
	r.tokIn += u.PromptTokens
	r.tokOut += u.CompletionTokens
	r.turnIn += u.PromptTokens
	r.turnOut += u.CompletionTokens
}

func (r *REPL) preview(c conversation.ToolCall) {
	var a map[string]any
	if json.Unmarshal([]byte(c.Arguments), &a) != nil {
		return
	}
	switch c.Name {
	case "run_shell":
		fmt.Fprintf(r.out, "  %s$ %v%s\n", yellow, a["command"], reset)
	case "edit_file":
		for _, l := range strings.Split(fmt.Sprint(a["old_string"]), "\n") {
			fmt.Fprintf(r.out, "  %s- %s%s\n", red, l, reset)
		}
		for _, l := range strings.Split(fmt.Sprint(a["new_string"]), "\n") {
			fmt.Fprintf(r.out, "  %s+ %s%s\n", green, l, reset)
		}
	case "write_file":
		lines := strings.Split(fmt.Sprint(a["content"]), "\n")
		for i, l := range lines {
			if i == 15 {
				fmt.Fprintf(r.out, "  %s  … %d more lines%s\n", dim, len(lines)-15, reset)
				break
			}
			fmt.Fprintf(r.out, "  %s+ %s%s\n", green, l, reset)
		}
	}
}

func summarize(args string) string {
	var a map[string]any
	if json.Unmarshal([]byte(args), &a) != nil {
		return ""
	}
	for _, k := range []string{"path", "command", "pattern"} {
		if v, ok := a[k]; ok {
			s := fmt.Sprint(v)
			s = truncate(strings.Join(strings.Fields(s), " "), 80)
			return s
		}
	}
	return ""
}

// modelCmd: "/model" lists available models (current marked); "/model <id>" switches.
func (r *REPL) modelCmd(arg string) {
	sel, ok := r.agent.Models()
	if !ok {
		fmt.Fprintf(r.out, "%smodel switching not supported by this client%s\n", red, reset)
		return
	}
	if arg != "" {
		sel.SetModel(arg)
		r.model = arg
		fmt.Fprintf(r.out, "%smodel → %s%s\n", yellow, arg, reset)
		return
	}
	fmt.Fprintf(r.out, "%scurrent: %s%s\n", bold, sel.Model(), reset)
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ids, err := sel.ListModels(ctx)
	if err != nil {
		fmt.Fprintf(r.out, "%s✗ %v%s\n", red, err, reset)
		return
	}
	for _, id := range ids {
		mark := "  "
		if id == sel.Model() {
			mark = green + "● " + reset
		}
		fmt.Fprintf(r.out, "%s%s\n", mark, id)
	}
	fmt.Fprintf(r.out, "%sswitch with /model <id>%s\n", dim, reset)
}

// apiKeyCmd shows the masked key and verifies it with an authenticated GET /models.
func (r *REPL) apiKeyCmd() {
	fmt.Fprintf(r.out, "  key      %s\n  source   %s\n  endpoint %s\n", r.keyMasked, r.keySource, r.baseURL)
	sel, ok := r.agent.Models()
	if !ok {
		fmt.Fprintf(r.out, "%s  cannot verify: client has no model listing%s\n", yellow, reset)
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	ids, err := sel.ListModels(ctx)
	if err != nil {
		fmt.Fprintf(r.out, "  %s✗ not working: %v%s\n", red, err, reset)
		return
	}
	fmt.Fprintf(r.out, "  %s✓ key accepted (%d models available)%s\n", green, len(ids), reset)
}

// apiKeySetCmd saves a key to the env file, applies it to this session, then verifies it.
func (r *REPL) apiKeySetCmd(key string) {
	if r.saveKey == nil {
		fmt.Fprintf(r.out, "%ssaving keys is not available%s\n", red, reset)
		return
	}
	if key == "" {
		fmt.Fprint(r.out, "  paste API key (hidden): ")
		key = strings.TrimSpace(r.readHidden())
		fmt.Fprintln(r.out)
	}
	if key == "" || strings.ContainsAny(key, " \t\r\n") {
		fmt.Fprintf(r.out, "  %s✗ empty or invalid key, nothing saved%s\n", red, reset)
		return
	}
	path, err := r.saveKey(key)
	if err != nil {
		fmt.Fprintf(r.out, "  %s✗ save failed: %v%s\n", red, err, reset)
		return
	}
	r.agent.SetAPIKey(key)
	r.keySource = path
	if len(key) <= 10 {
		r.keyMasked = strings.Repeat("*", len(key))
	} else {
		r.keyMasked = key[:4] + strings.Repeat("*", 8) + key[len(key)-4:]
	}
	fmt.Fprintf(r.out, "  %s✓ saved to %s (mode 600)%s\n", green, path, reset)
	if os.Getenv("KODINGIN_API_KEY") != "" && os.Getenv("KODINGIN_API_KEY") != key {
		fmt.Fprintf(r.out, "  %snote: env KODINGIN_API_KEY is set and wins on next start; unset it to use the saved key%s\n", yellow, reset)
	}
	r.apiKeyCmd()
}

// readHidden turns off terminal echo while reading one line (no-op echo if not a TTY).
func (r *REPL) readHidden() string {
	off := exec.Command("stty", "-echo")
	off.Stdin = os.Stdin
	hidden := off.Run() == nil
	line, _ := r.in.ReadString('\n')
	if hidden {
		on := exec.Command("stty", "echo")
		on.Stdin = os.Stdin
		_ = on.Run()
	}
	return line
}
