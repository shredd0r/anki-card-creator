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

type File struct {
	Filename string
	Content  []byte
	MIMEType string
}

// Example is one example sentence plus the exact subject form (if known)
// that appears within it - e.g. for subject "go", SubjectForm might be
// "went". SubjectForm is empty when the source doesn't report it (e.g.
// Cambridge Dictionary examples), in which case formatting falls back to a
// best-effort guess based on the subject's own spelling.
type Example struct {
	Sentence    string
	SubjectForm string
}

type Flashcard struct {
	Subject       string
	SubjectType   SubjectType
	DeckName      string
	Pronunciation *File   // Can be nil if subject is phrase
	Transcription *string //
	Paraphrase    string
	Synonyms      []string
	Picture       *File
	Examples      []Example
	Tags          []string
}

type CardContent struct {
	Paraphrase    string
	Transcription *string
	Pronunciation *File
	Examples      []Example
	Synonyms      []string
}
