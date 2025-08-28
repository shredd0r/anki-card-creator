package models

import "io"

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

type FlashCard struct {
	Subject       string
	SubjectType   SubjectType
	DeckName      string
	EnglishLevel  EnglishLevel
	Pronouns      io.Reader // Can be nil if subject is phrase
	Transcription *string   //
	Explain       string
	Picture       io.Reader
	Examples      []string
}

type NotesCard struct {
	Subject     string
	SubjectType SubjectType
	LessonName  string
	Pronouns    io.Reader // Can be nil, if subject is phrase
	Explain     string
}

type CambridgeCard struct {
	Subject       string
	SubjectType   SubjectType
	Pronouns      io.Reader // Can be nil, if subject is phrase
	Transcription *string   //
	Explains      []string
	Examples      []string
}

type QuizletCard struct {
	Subject     string
	SubjectType SubjectType
	Pronouns    *io.Reader
	Explain     string
	Picture     *io.Reader
}
