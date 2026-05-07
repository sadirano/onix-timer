package config

import (
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Timer TimerConfig `toml:"timer"`
}

type TimerConfig struct {
	RawSeconds bool         `toml:"raw_seconds"`
	Notify     bool         `toml:"notify"`
	FZF        FZFSubConfig `toml:"fzf"`
}

type FZFSubConfig struct {
	Prompt string `toml:"prompt"`
	Layout string `toml:"layout"`
}

func defaultConfig() Config {
	return Config{
		Timer: TimerConfig{
			RawSeconds: false,
			Notify:     true,
			FZF: FZFSubConfig{
				Prompt: "timer> ",
				Layout: "default",
			},
		},
	}
}

func LoadConfig(onixHome string) Config {
	cfg := defaultConfig()
	if onixHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return cfg
		}
		onixHome = filepath.Join(home, ".onix")
	}
	p := filepath.Join(timerDir(onixHome), "config.toml")
	// Graceful fallback: missing or malformed config → use defaults
	if _, err := toml.DecodeFile(p, &cfg); err != nil {
		return defaultConfig()
	}
	return applyDefaults(cfg)
}

func applyDefaults(cfg Config) Config {
	if cfg.Timer.FZF.Prompt == "" {
		cfg.Timer.FZF.Prompt = "timer> "
	}
	return cfg
}

