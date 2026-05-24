package config

import (
	"os"
	"path/filepath"

	"github.com/spf13/viper"
)

type Config struct {
	UnusedThresholdDays int      `mapstructure:"unused_threshold_days"`
	UnusedMinSizeMB     int      `mapstructure:"unused_min_size_mb"`
	ExcludePaths        []string `mapstructure:"exclude_paths"`
	NoColor             bool     `mapstructure:"no_color"`
}

var Defaults = Config{
	UnusedThresholdDays: 90,
	UnusedMinSizeMB:     500,
}

func Load() (*Config, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return &Defaults, nil
	}

	v := viper.New()
	v.SetConfigName(".devcleanrc")
	v.SetConfigType("toml")
	v.AddConfigPath(home)

	v.SetDefault("unused_threshold_days", Defaults.UnusedThresholdDays)
	v.SetDefault("unused_min_size_mb", Defaults.UnusedMinSizeMB)

	if err := v.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
			cfg := Defaults
			return &cfg, nil
		}
		return &Defaults, err
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return &Defaults, err
	}

	// Always add the safety blocklist to exclusions
	cfg.ExcludePaths = append(cfg.ExcludePaths, safetyBlocklist(home)...)
	return &cfg, nil
}

func safetyBlocklist(home string) []string {
	return []string{
		filepath.Join(home, "Documents"),
		filepath.Join(home, "Desktop"),
		filepath.Join(home, "Downloads"),
		filepath.Join(home, "Pictures"),
		filepath.Join(home, ".ssh"),
		filepath.Join(home, ".gnupg"),
		filepath.Join(home, ".aws", "credentials"),
	}
}
