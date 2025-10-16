package config

type Config struct {
	QueueSize        int
	PictureConfig    PictureConfig
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

type PictureConfig struct {
	IgnorePictureError           bool
	NumberOfAttemptRatingPicture uint
	MinimalRating                uint
}
