package config

import (
	"errors"
	"fmt"

	"github.com/spf13/viper"
)

type Config struct {
	AnthropicAPIKey string
	OllamaBaseURL   string
	DefaultModel    string
	DefaultProvider string // "anthropic" | "ollama"
}

func Load() (*Config, error) {
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv() // real env vars override .env

	viper.SetDefault("OLLAMA_BASE_URL", "http://localhost:11434")
	viper.SetDefault("DEFAULT_MODEL", "qwen3.5:9b")
	viper.SetDefault("DEFAULT_PROVIDER", "ollama")

	// .env is optional — only fail on real parse errors
	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return nil, fmt.Errorf("config: %w", err)
		}
	}

	cfg := &Config{
		AnthropicAPIKey: viper.GetString("ANTHROPIC_API_KEY"),
		OllamaBaseURL:   viper.GetString("OLLAMA_BASE_URL"),
		DefaultModel:    viper.GetString("DEFAULT_MODEL"),
		DefaultProvider: viper.GetString("DEFAULT_PROVIDER"),
	}

	if cfg.DefaultProvider == "anthropic" && cfg.AnthropicAPIKey == "" {
		return nil, fmt.Errorf("ANTHROPIC_API_KEY required when DEFAULT_PROVIDER=anthropic")
	}

	return cfg, nil
}
