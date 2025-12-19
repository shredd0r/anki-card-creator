package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Debugging bool          `yaml:"debugging"`
	QueueSize uint          `yaml:"queue-size"`
	Output    string        `yaml:"output"`
	OnlyAI    bool          `yaml:"only-ai"`
	Gemini    *GeminiConfig `yaml:"gemini"`
	Ollama    *OllamaConfig `yaml:"ollama"`
	Picture   PictureConfig `yaml:"picture"`
}

type GeminiConfig struct {
	Token string `yaml:"token"`
}

type OllamaConfig struct {
	Host    string        `yaml:"host"`
	Port    uint16        `yaml:"port"`
	Model   string        `yaml:"model"`
	Timeout time.Duration `yaml:"timeout"`
}

type PictureConfig struct {
	IgnoreError   bool  `yaml:"ignore"`
	CountSearches uint8 `yaml:"count-searches"`
	MinimalRating uint8 `yaml:"min-rating"`
}

func Read(pathToFile string) (*Config, error) {
	cfg := Config{}
	configFile, err := os.ReadFile(pathToFile)
	if err != nil {
		return nil, err
	}

	if err := yaml.Unmarshal(configFile, &cfg); err != nil {
		return nil, err
	}

	return &cfg, nil
}
