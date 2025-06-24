package config

import (
	"os"
	"path/filepath"
	"time"

	"github.com/BurntSushi/toml"
)

type Config struct {
	NapTime time.Duration
	Dirs    struct {
		Watch   string
		Output  string
		Done    string
		Problem string
	}
	Output struct {
		Crf        int
		MaxHeight  int
		MaxWidth   int
		MaxBitrate int
	}
}

func Load() (*Config, error) {
	wd, err := os.Getwd()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(wd, "config.toml")

	f, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config Config
	_, err = toml.Decode(string(f), &config)
	if err != nil {
		return nil, err
	}

	return &config, nil
}
