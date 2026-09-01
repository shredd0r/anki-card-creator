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
	LLM       *LLMConfig    `yaml:"llm"`
	Picture   PictureConfig `yaml:"picture"`
}

type GeminiConfig struct {
	Token string `yaml:"token"`
}

type LLMConfig struct {
	Seed    int64         `yaml:"seed"`     // Seed for generating content
	BaseUrl string        `yaml:"base-url"` // Url to server with launched model based on OpenAI APIes
	Token   string        `yaml:"token"`    // Token for llm provider
	Model   string        `yaml:"model"`    // Model name
	Timeout time.Duration `yaml:"timeout"`  // Timeout for response from llm server
}

type PictureConfig struct {
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
