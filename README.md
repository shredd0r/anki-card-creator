# Anki card creator

This tool generates package with flashcard for improving your English skill. After generate you can import .apkg file to your anki. For template is used [Maple Template X](https://github.com/liuzikai/Maple-Anki-Template) with small changes.
 

Flashcard has this fields:
* Subject - word, phrase, idiom which you want learn;
* Pronunciation - sound file with pronunciation word;
* Transcription;
* Synonyms;
* Description - describe subject in front side flashcard;
* Picture - describe subject in both sides flashcard;
* Examples - examples how you can use this subject;

Information for flashcards is taken from: 
* [Cambridge dictionary](https://dictionary.cambridge.org/), main source, because this volumes already approved by humans;
* Your local llm ran on [Lm-Studio server](https://lmstudio.ai/), used for generate volumes if it not found in dictionary, and rating picture for flashcard;
* Google image, for find pictures for flashcard

## Features
For some fields used LLM. This creator support ollama apies, you can run local LLM by ollama and use it for generating flashcards.

## Requirements
* OpenAI API token key or [LM Studio server](https://lmstudio.ai/download) with downloaded some model

## Example file with subjects
```bash
// example.json
[
    {
    "deckname": "Group 1",
    "subjects": [
      "turn up",
      "speak up",
    ]
  },
  {
    "deckname": "Group 2",
    "tags": ["ideas"],
    "subjects": [
      "open to ideas",
      "stolen ideas",
    ]
  }
]
```

## Run
### Format running command
You can put into run command one json file or directory with files:
```
.
├── direction-and-location.json
└── lessons/
    ├── lesson-1.json
    └── lesson-2.json
```
```
    anki-card-creator directory/file-with-subjects --opts...
```


# TODO
* after adding flashcard to anki some decks need to restore using button "check database", I think its bug on creator