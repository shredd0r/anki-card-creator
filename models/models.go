package models

type SubjectType int

const (
	SubjectTypeNone   = -1
	SubjectTypeWord   = 0
	SubjectTypePhrase = 1
)

type EnglishLevel int

const (
	EnglishLevelNone = -1
	EnglishLevelA2   = 0
	EnglishLevelB1   = 1
)

type Target struct {
	DeckName string   `json:"deckname"`
	Subjects []string `json:"subjects"`
}

type File struct {
	Content  []byte
	MIMEType string
}

type Flashcard struct {
	Subject       string
	SubjectType   SubjectType
	DeckName      string
	Pronunciation *File   // Can be nil if subject is phrase
	Transcription *string //
	Explain       string
	Picture       *File
	Examples      []string
}

type CambridgeCard struct {
	Subject       string
	Pronunciation *File   // Can be nil, if subject is phrase
	Transcription *string //
	Explains      []string
	Examples      []string
}
