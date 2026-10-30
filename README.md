# Anki card creator

A CLI tool that generates Anki flashcards for learning English words, phrases and idioms, and writes them directly into a running Anki instance via the [AnkiConnect](https://ankiweb.net/shared/info/2055492159) addon. There's no `.apkg` file to import — Anki desktop with AnkiConnect installed must be running whenever you run this tool.

Each flashcard has these fields:
* **Subject** - the word, phrase or idiom you want to learn;
* **Pronunciation** - an audio file with the subject's pronunciation;
* **Transcription**;
* **Paraphrase** - an explanation of the subject, shown on the front of the card;
* **Synonyms**;
* **Picture** - shown on both sides of the card;
* **Examples** - example sentences using the subject, with the subject's form in the sentence highlighted.

Content for each flashcard is assembled from multiple sources, run concurrently:
* [Cambridge Dictionary](https://dictionary.cambridge.org/) - tried first for paraphrase, transcription, examples and pronunciation audio (single words only; phrases fall straight through to the LLM below);
* an LLM, reached through any OpenAI-compatible API (a local [LM Studio](https://lmstudio.ai/) server, or OpenAI itself) - fills in whatever Cambridge doesn't have, and is the only source for synonyms and for rating candidate pictures;
* [DuckDuckGo Images](https://duckduckgo.com/) - searched for a candidate picture, which the LLM rates until one clears the configured minimum rating;
* Google Translate's text-to-speech endpoint - used for pronunciation audio whenever Cambridge doesn't have it.

## Features

`config.yaml`'s `llm.only-ai` flag selects one of two scenarios for the whole run:
* **with dictionary** (`only-ai: false`, default) - tries Cambridge Dictionary first for paraphrase/transcription/examples, falling back to the LLM field by field; synonyms always come from the LLM. This also transparently covers phrases, since Cambridge only supports single words.
* **only AI** (`only-ai: true`) - skips Cambridge entirely; paraphrase, transcription, examples and synonyms all come from the LLM, and pronunciation comes from text-to-speech.

Picture search (DuckDuckGo Images, rated by the LLM) can be disabled entirely via `picture.ignore: true`.

## Requirements
* [Anki](https://apps.ankiweb.net/) desktop with the [AnkiConnect](https://ankiweb.net/shared/info/2055492159) addon installed, running whenever you run this tool;
* An OpenAI-compatible LLM endpoint - either OpenAI's own API, or a local server such as [LM Studio](https://lmstudio.ai/download) with a model downloaded;
* Go 1.25+, if building from source.

## Configuration
Connection and model settings are read from `./config.yaml` (see [`config-example.yaml`](config-example.yaml)):

```yaml
queue-size: 2        # how many subjects to process concurrently
llm:
  only-ai: true       # true: skip Cambridge Dictionary, generate everything via AI
  seed: 1572738495
  base-url: http://localhost:1234/v1
  token: lm-studio
  model: google/gemma-4-12b-qat
  timeout: 2m
picture:
  min-rating: 5       # minimum LLM rating a picture needs to be accepted
  count-searches: 0   # how many DuckDuckGo results to try before giving up
  ignore: false        # true: skip picture search entirely
anki-connect:
  base-url: http://127.0.0.1:8765
  token: ""
  timeout: 30s
```

## Example file with subjects
```json
[
  {
    "deckname": "Group 1",
    "subjects": [
      "turn up",
      "speak up"
    ]
  },
  {
    "deckname": "Group 2",
    "tags": ["ideas"],
    "subjects": [
      "open to ideas",
      "stolen ideas"
    ]
  }
]
```
`tags` is optional. Re-running the tool on the same deck/subject updates the existing card instead of creating a duplicate.

## Run
Build the binary for your platform:
```bash
make build-macos-arm    # also: build-macos-intel, build-linux, build-linux-arm, build-windows
```

With Anki (and AnkiConnect) running, and `config.yaml` and `word.json` next to the binary, run it:
```bash
./anki-card-creator
```
It reads subjects from `./word.json` in the current directory and writes a flashcard into Anki for each one, creating any missing deck along the way.

## Development
```bash
go generate ./...                  # regenerate mocks (after changing any interface)
make test                          # go generate + go test -v -race -count=5 ./...
go test -run TestName ./card/fetcher/...   # run a single test
```
