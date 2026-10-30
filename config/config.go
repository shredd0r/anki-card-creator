package config

import (
	"os"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Debugging   bool              `yaml:"debugging"`
	QueueSize   uint              `yaml:"queue-size"`
	LLM         *LLMConfig        `yaml:"llm"`
	Picture     PictureConfig     `yaml:"picture"`
	AnkiConnect AnkiConnectConfig `yaml:"anki-connect"`
}

type GeminiConfig struct {
	Token string `yaml:"token"`
}

type LLMConfig struct {
	OnlyAI  bool          `yaml:"only-ai"`  // true: skip Cambridge dictionary, generate everything via AI
	Seed    int64         `yaml:"seed"`     // Seed for generating content
	BaseUrl string        `yaml:"base-url"` // Url to server with launched model based on OpenAI APIes
	Token   string        `yaml:"token"`    // Token for llm provider
	Model   string        `yaml:"model"`    // Model name
	Timeout time.Duration `yaml:"timeout"`  // Timeout for response from llm server
}

type PictureConfig struct {
	CountSearches uint8 `yaml:"count-searches"`
	MinimalRating uint8 `yaml:"min-rating"`
	Ignore        bool  `yaml:"ignore"` // true: skip picture search/rating entirely
}

type AnkiConnectConfig struct {
	BaseUrl string        `yaml:"base-url"` // AnkiConnect endpoint; defaults to http://127.0.0.1:8765 when empty
	Token   string        `yaml:"token"`    // optional AnkiConnect API key
	Timeout time.Duration `yaml:"timeout"`  // per-call budget for AnkiConnect requests
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
