// Composition root: wire infrastructure adapters into the application and hand it to the CLI.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/thisham/kodingin/internal/application"
	"github.com/thisham/kodingin/internal/domain/conversation"
	"github.com/thisham/kodingin/internal/domain/tool"
	"github.com/thisham/kodingin/internal/infrastructure/config"
	llmadapter "github.com/thisham/kodingin/internal/infrastructure/llm"
	"github.com/thisham/kodingin/internal/infrastructure/tools"
	"github.com/thisham/kodingin/internal/interfaces/cli"
	"github.com/thisham/kodingin/internal/version"
)

func main() {
	cfg, err := config.Load()
	dir := flag.String("C", ".", "working directory")
	model := flag.String("model", cfg.Model, "model id")
	yolo := flag.Bool("yes", false, "auto-approve writes and shell commands")
	prompt := flag.String("p", "", "run a single prompt and exit")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()
	if *showVersion {
		fmt.Println("kodingin v" + version.Current)
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "kodingin:", err)
		os.Exit(1)
	}
	root, err := filepath.Abs(*dir)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	sb := tools.Sandbox{Root: root}
	registry := tool.NewRegistry(
		tools.ReadFile{Sandbox: sb}, tools.WriteFile{Sandbox: sb}, tools.EditFile{Sandbox: sb},
		tools.ListDir{Sandbox: sb}, tools.Grep{Sandbox: sb}, tools.Shell{Sandbox: sb},
		tools.GoCheck{Sandbox: sb}, tools.GoTest{Sandbox: sb},
	)
	client := llmadapter.New(cfg.BaseURL, cfg.APIKey, *model)
	agent := application.NewAgent(client, registry, conversation.New(application.SystemPrompt(root)), cfg.MaxSteps)
	ui := cli.New(agent, *model, root, *yolo)
	ui.SetKeyInfo(cfg.MaskedKey(), cfg.APIKeySource, cfg.BaseURL)
	ui.SetKeySaver(config.SaveAPIKey)
	ui.SetSettingsSaver(func(settings map[string]string) (string, error) {
		return config.SaveSettings(settings)
	})

	if p := strings.TrimSpace(*prompt + " " + strings.Join(flag.Args(), " ")); p != "" {
		err = ui.RunOnce(p)
	} else {
		err = ui.Run()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "kodingin:", err)
		os.Exit(1)
	}
}
