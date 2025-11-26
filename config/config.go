package config

type Config struct {
	Debugging bool          `yaml:"debugging"`
	QueueSize uint          `yaml:"queue-size"`
	Gemini    GeminiConfig  `yaml:"gemini"`
	Ollama    OllamaConfig  `yaml:"ollama"`
	Picture   PictureConfig `yaml:"picture"`
}

type GeminiConfig struct {
	Token string `yaml:"token"`
}

type OllamaConfig struct {
	Host  string `yaml:"host"`
	Port  uint   `yaml:"port"`
	Model string `yaml:"model"`
}

type PictureConfig struct {
	IgnoreError   bool `yaml:"ignore"`
	CountSearches uint `yaml:"count-searches"`
	MinimalRating uint `yaml:"min-rating"`
}
