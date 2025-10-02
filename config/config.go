package config

type Config struct {
	RatingConfig     RatingConfig
	NotesConfig      NotesConfig
	GeminiConfig     GeminiConfig
	PlayWrightConfig PlayWrightConfig
}

type PlayWrightConfig struct {
	ShowBrowser bool
}

type NotesConfig struct {
	Token    string
	TokenRaw string
}

type GeminiConfig struct {
	Token string
}

type RatingConfig struct {
	NumberOfAttemptRatingPicture uint
	MinimalRating                uint
}
