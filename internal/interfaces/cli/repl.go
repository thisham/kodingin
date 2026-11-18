// Package cli is the terminal delivery layer: REPL, colours, approval prompts.
package cli

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/thisham/kodingin/internal/application"
	"github.com/thisham/kodingin/internal/domain/conversation"
	"github.com/thisham/kodingin/internal/domain/llm"
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
	agent       *application.Agent
	in          *bufio.Reader
	out         io.Writer
	autoApprove bool
	model       string
	workdir     string
	tokIn       int
	tokOut      int
	keyMasked   string
	keySource   string
	baseURL     string
}

// SetKeyInfo gives the REPL what /api-key displays. The raw key never reaches the UI.
func (r *REPL) SetKeyInfo(masked, source, baseURL string) {
	r.keyMasked, r.keySource, r.baseURL = masked, source, baseURL
}

func New(a *application.Agent, model, workdir string, autoApprove bool) *REPL {
	return &REPL{agent: a, in: bufio.NewReader(os.Stdin), out: os.Stdout, autoApprove: autoApprove, model: model, workdir: workdir}
}

func (r *REPL) banner() {
	fmt.Fprintf(r.out, "%s%s ┌─ kodingin ─────────────────────────────%s\n", bold, mag, reset)
	fmt.Fprintf(r.out, "%s%s │%s model   %s\n", bold, mag, reset, r.model)
	fmt.Fprintf(r.out, "%s%s │%s workdir %s\n", bold, mag, reset, r.workdir)
	fmt.Fprintf(r.out, "%s%s └─%s %s/help /model /api-key /clear /yolo /exit · Ctrl-C cancels a turn%s\n\n", bold, mag, reset, dim, reset)
}

// RunOnce executes a single prompt non-interactively.
func (r *REPL) RunOnce(prompt string) error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
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
		line = strings.TrimSpace(line)
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
			fmt.Fprintf(r.out, "%sconversation cleared%s\n", dim, reset)
			continue
		case "/api-key":
			r.apiKeyCmd()
			continue
		case "/yolo":
			r.autoApprove = !r.autoApprove
			fmt.Fprintf(r.out, "%sauto-approve: %v%s\n", yellow, r.autoApprove, reset)
			continue
		case "/help":
			fmt.Fprintf(r.out, "%s/api-key  check the API key works\n/model [id]  show/list models, or switch to <id>\n/clear  reset conversation\n/yolo   toggle auto-approve for writes and shell\n/exit   quit\nEnd a line with \\ to continue on the next line.%s\n", dim, reset)
			continue
		}
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
	fmt.Fprintf(r.out, "%s── tokens in %d · out %d%s\n\n", dim, r.tokIn, r.tokOut, reset)
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
		if len(l) > 120 {
			l = l[:120] + "…"
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
	fmt.Fprintf(r.out, "  %sallow %s? [y]es / [n]o / [a]lways:%s ", yellow, c.Name, reset)
	ans, _ := r.in.ReadString('\n')
	switch strings.ToLower(strings.TrimSpace(ans)) {
	case "y", "yes", "":
		return true
	case "a", "always":
		r.autoApprove = true
		return true
	}
	return false
}

func (r *REPL) OnUsage(u llm.Usage) { r.tokIn += u.PromptTokens; r.tokOut += u.CompletionTokens }

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
			if len(s) > 80 {
				s = s[:80] + "…"
			}
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
