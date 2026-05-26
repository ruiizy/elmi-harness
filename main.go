package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/ruiizy/elmi-harness/config"
	"github.com/ruiizy/elmi-harness/internal/agent"
	"github.com/ruiizy/elmi-harness/internal/api"
	"github.com/ruiizy/elmi-harness/internal/perm"
	"github.com/ruiizy/elmi-harness/internal/tools"
	"github.com/ruiizy/elmi-harness/provider"
	anthropicprovider "github.com/ruiizy/elmi-harness/provider/anthropic"
	openaiprovider "github.com/ruiizy/elmi-harness/provider/openai"
	"github.com/ruiizy/elmi-harness/ui"
)

const systemPrompt = `You are a coding assistant running in a terminal. You have three tools:
bash, read_file, write_file. Be concise.`

// maxMessages caps conversation history to avoid unbounded memory and token growth.
const maxMessages = 100

func newProvider(choice ui.ModelChoice, ollamaBase string) provider.Provider {
	switch choice.Provider {
	case "anthropic":
		return anthropicprovider.New(choice.Model, 8192, systemPrompt)
	default: // "ollama"
		return openaiprovider.New(choice.Model, systemPrompt, 8192, ollamaBase+"/v1")
	}
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	// Anthropic SDK reads ANTHROPIC_API_KEY from the real environment.
	// If it came from .env via viper, expose it so the SDK can see it.
	if cfg.AnthropicAPIKey != "" {
		os.Setenv("ANTHROPIC_API_KEY", cfg.AnthropicAPIKey)
	}

	ui.PrintBanner()

	// ctx is cancelled on SIGINT/SIGTERM, which interrupts in-flight API calls.
	ctx, cancelSignal := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancelSignal()

	permCfg := perm.New()
	var messages []api.Message

	llm := newProvider(ui.ModelChoice{Provider: cfg.DefaultProvider, Model: cfg.DefaultModel}, cfg.OllamaBaseURL)

	var history []string
	var quit bool

	registry := map[string]command{}
	registry["help"] = command{
		description: "show available commands",
		run:         func(args string) { cmdHelp(registry, args) },
	}
	registry["model"] = command{
		description: "switch the active model",
		run: func(_ string) {
			if choice := ui.RunModelPicker(cfg.OllamaBaseURL, llm.Model()); choice != nil {
				llm = newProvider(*choice, cfg.OllamaBaseURL)
				messages = nil
				fmt.Printf("switched to %s (%s)\n", choice.Model, choice.Provider)
			}
		},
	}
	registry["mode"] = command{
		description: "set tool permission mode",
		usage:       "always-ask | always-allow | ask-once | allow-list <tools...>",
		run: func(args string) {
			if args == "" {
				fmt.Printf("current mode: %s\n", permCfg.Mode)
				cmdHelp(registry, "")
				return
			}
			parts := strings.Fields(args)
			switch parts[0] {
			case "always-ask":
				permCfg.SetMode(perm.ModeAlwaysAsk)
			case "always-allow":
				permCfg.SetMode(perm.ModeAlwaysAllow)
			case "ask-once":
				permCfg.SetMode(perm.ModeAskOnce)
				permCfg.ResetAskOnce()
			case "allow-list":
				if len(parts) < 2 {
					fmt.Println("usage: /mode allow-list <tool1> [tool2...]")
					return
				}
				permCfg.SetMode(perm.ModeAllowList)
				permCfg.SetAllowList(parts[1:])
			default:
				fmt.Printf("unknown mode: %s\n", parts[0])
				return
			}
			fmt.Printf("mode set: %s\n", permCfg.Mode)
		},
	}
	registry["clear"] = command{
		description: "clear conversation history",
		run:         func(_ string) { messages = nil; fmt.Println("history cleared") },
	}
	registry["exit"] = command{
		description: "exit the harness",
		run:         func(_ string) { quit = true },
	}

	for {
		prompt := fmt.Sprintf("[%s | mode:%s] > ", llm.Model(), permCfg.Mode)
		input, ctrlC := ui.ReadLine(prompt, history)
		if ctrlC {
			return
		}
		if input == "" {
			continue
		}
		history = append(history, input)

		if runCommand(registry, input) {
			if quit {
				return
			}
			continue
		}

		messages = append(messages, api.Message{
			Role:    api.RoleUser,
			Content: []api.Block{{Type: api.BlockText, Text: input}},
		})

		var textStarted bool
		spinner := ui.StartSpinner("thinking...")
		updated, err := agent.Run(ctx, llm, messages, tools.Definitions(), agent.Hooks{
			OnText: func(chunk string) {
				if !textStarted {
					spinner.Stop() // clear spinner before first chunk
					textStarted = true
				}
				fmt.Print(chunk)
			},
			OnToolUse: func(name, rawInput string) agent.ToolResult {
				if textStarted {
					fmt.Println() // newline after streamed text
					textStarted = false
				}
				spinner.Stop()
				run, override := permCfg.Check(name, rawInput, ui.RunConfirmTUI)
				spinner = ui.StartSpinner("thinking...")
				switch {
				case run:
					out, isErr := tools.Execute(name, rawInput)
					return agent.ToolResult{Content: out, IsError: isErr}
				case override != "":
					return agent.ToolResult{Content: override, IsError: true}
				default:
					return agent.ToolResult{Content: "user denied permission to run this tool", IsError: true}
				}
			},
		})
		spinner.Stop()
		if textStarted {
			fmt.Println() // newline after final streamed text
		}

		// Signal received during API call — exit cleanly.
		if ctx.Err() != nil {
			fmt.Println()
			return
		}
		if err != nil {
			fmt.Printf("error: %v\n", err)
		}

		messages = updated
		if len(messages) > maxMessages {
			messages = messages[len(messages)-maxMessages:]
		}
	}
}
