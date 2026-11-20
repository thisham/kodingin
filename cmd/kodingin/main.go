// Composition root: wire infrastructure adapters into the application and hand it to the CLI.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/thisham/kodingin/internal/application"
	"github.com/thisham/kodingin/internal/domain/conversation"
	"github.com/thisham/kodingin/internal/domain/tool"
	"github.com/thisham/kodingin/internal/infrastructure/config"
	"github.com/thisham/kodingin/internal/infrastructure/historystore"
	llmadapter "github.com/thisham/kodingin/internal/infrastructure/llm"
	"github.com/thisham/kodingin/internal/infrastructure/portal"
	"github.com/thisham/kodingin/internal/infrastructure/sessionstore"
	"github.com/thisham/kodingin/internal/infrastructure/tools"
	"github.com/thisham/kodingin/internal/infrastructure/usagestore"
	"github.com/thisham/kodingin/internal/interfaces/cli"
	"github.com/thisham/kodingin/internal/version"
)

func main() {
	cfg, err := config.Load()
	dir := flag.String("C", ".", "working directory")
	model := flag.String("model", cfg.Model, "model id")
	yolo := flag.Bool("yes", false, "auto-approve writes and shell commands")
	prompt := flag.String("p", "", "run a single prompt and exit")
	cont := flag.Bool("continue", false, "resume the most recent session")
	harness := flag.Bool("harness", false, "run a prompt as an autonomous JSON harness")
	timeout := flag.Duration("timeout", 10*time.Minute, "harness execution timeout")
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
	if path, err := config.UsageFilePath(); err == nil {
		ui.SetUsageStore(usagestore.New(path), path)
	}
	if path, err := config.HistoryFilePath(); err == nil {
		ui.SetHistoryStore(historystore.New(path), path)
	}
	if dir, err := config.SessionsDir(); err == nil {
		ui.SetSessionStore(sessionstore.New(dir))
	}
	if *cont {
		ui.ResumeLatest(context.Background())
	}
	ui.SetCreditSource(portal.New(cfg.BaseURL, cfg.PortalCookie))
	ui.SetSettingsSaver(func(settings map[string]string) (string, error) {
		return config.SaveSettings(settings)
	})

	p := strings.TrimSpace(*prompt + " " + strings.Join(flag.Args(), " "))
	if *harness {
		if p == "" {
			fmt.Fprintln(os.Stderr, "kodingin: -harness requires a prompt via -p or positional arguments")
			os.Exit(2)
		}
		ctx, cancel := context.WithTimeout(context.Background(), *timeout)
		defer cancel()
		report := application.RunHarness(ctx, agent, p)
		data, marshalErr := application.MarshalHarnessReport(report)
		if marshalErr != nil {
			err = marshalErr
		} else {
			fmt.Println(string(data))
			if !report.Success {
				err = fmt.Errorf("harness failed: %s", report.Error)
			}
		}
	} else if p != "" {
		err = ui.RunOnce(p)
	} else {
		err = ui.Run()
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "kodingin:", err)
		os.Exit(1)
	}
}
