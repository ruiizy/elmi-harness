package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/ruiizy/elmi-harness/config"
	"github.com/ruiizy/elmi-harness/internal/perm"
	"github.com/ruiizy/elmi-harness/provider"
	anthropicprovider "github.com/ruiizy/elmi-harness/provider/anthropic"
	openaiprovider "github.com/ruiizy/elmi-harness/provider/openai"
	"github.com/ruiizy/elmi-harness/ui"
)

const systemPrompt = `You are a coding assistant running in a terminal. You have tools:
bash, read_file, write_file, fetch. Be concise.`

const maxMessages = 100

func newProvider(choice ui.ModelChoice, ollamaBase string) provider.Provider {
	switch choice.Provider {
	case "anthropic":
		return anthropicprovider.New(choice.Model, 8192, systemPrompt)
	default:
		return openaiprovider.New(choice.Model, systemPrompt, 8192, ollamaBase+"/v1")
	}
}

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}
	if cfg.AnthropicAPIKey != "" {
		os.Setenv("ANTHROPIC_API_KEY", cfg.AnthropicAPIKey)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	permCfg := perm.New()
	llm := newProvider(ui.ModelChoice{Provider: cfg.DefaultProvider, Model: cfg.DefaultModel}, cfg.OllamaBaseURL)

	if err := RunSession(ctx, llm, permCfg, cfg.OllamaBaseURL); err != nil {
		log.Fatalf("session: %v", err)
	}
}
