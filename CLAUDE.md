# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go CLI tool that generates Anki `.apkg` decks for learning English words/phrases. For each subject it assembles a flashcard from multiple sources, running them concurrently:
- **Cambridge Dictionary** (scraped via headless browser) — paraphrase, transcription, pronunciation audio, examples (words only)
- **LLM** (OpenAI-compatible API, e.g. local LM Studio) — synonyms, examples fallback, generated paraphrase for phrases, and picture rating
- **DuckDuckGo Images** (scraped via headless browser) — candidate pictures, rated by the LLM until one clears `picture.min-rating`

Output is written as a single `.apkg` package (via `genanki-go`) using a bundled Mustache-style card template (`anki/const.go`), based on [Maple Anki Template](https://github.com/liuzikai/Maple-Anki-Template).

## Commands

```bash
go generate ./...             # regenerate all mockgen mocks (run after changing any interface)
go test -v -race -count=5 ./...   # full test suite (as run by `make test`); note count=5 — tests re-run 5x to catch flakiness/races
go test -run TestName ./card/fetcher/...   # run a single test
make build-macos-arm          # build for current platform (also: build-macos-intel, build-linux, build-linux-arm, build-windows)
```

There is no lint target configured. Version is injected at build time via `-ldflags` into `internal/version.Version` (see `Makefile`); it defaults to `"development"` in unbuilt/test binaries.

The app is run as `./anki-card-creator <file-or-directory>.json`, reading connection/model settings from `./config.yaml` (see `config-example.yaml`). Note: `main.go` currently hardcodes the input path to `./test.json` rather than reading `os.Args`.

## Architecture

**Pipeline**: `extractor` (reads target JSON) → `service.AnkiCardCreator` (fan-out orchestration) → `card.Creator` (per-subject assembly) → `card/fetcher.CardContentFactory` (per-subject-type content) → `anki.Service` (accumulates decks, writes `.apkg`).

- **`extractor`**: `Target` reads one JSON file or recursively walks a directory of `*.json` files into `[]TargetInfo{DeckName, Tags, Subjects}`. `Cambridge` scrapes dictionary.cambridge.org via Playwright for word-only content (transcription, definitions, examples, pronunciation audio).
- **`service.AnkiCardCreator`**: top-level orchestrator. Fans out one goroutine per subject (bounded by `config.queue-size`, default 10) into a bounded channel of `*models.Flashcard`; a separate goroutine drains that channel into `anki.Service`. Any single subject error cancels the whole run via a shared `context.WithCancel`.
- **`card.Creator`**: for one subject, classifies it as word vs. phrase (`utils.GetSubjectType` — anything with a space or hyphen is a phrase) and runs three fetches concurrently (card content, picture, pronunciation), cancelling siblings on first error.
- **`card/fetcher.CardContentFactory`**: returns a cached `CardContent` implementation per `models.SubjectType`:
  - `wordCardContent` — combines Cambridge (`extractor.Cambridge`) + LLM (`llm.Provider`) results in parallel via `errgroup`; falls back to LLM-generated examples if Cambridge has none; generates pronunciation audio via `htgo-tts` if Cambridge didn't supply it; searches/rates DuckDuckGo Images (`duckduckgo.Image`) up to `picture.count-searches` attempts, keeping the first picture whose LLM rating ≥ `picture.min-rating`.
  - `phraseCardContent` — LLM-only; explicitly skips picture and pronunciation.
- **`llm`**: `Provider` wraps a `Client` and JSON-(un)marshals prompts/responses (`GeneratedCardContent`, `RatedPictureContent`); prompt/schema text lives in `llm/system.go` / `llm/schema.go`. `implClient` (constructed via `NewOpenAIClient`) is the only `Client` implementation, built on [`langchaingo`](https://github.com/tmc/langchaingo)'s `llms/openai` backend (talks to any OpenAI-compatible chat completions API, e.g. local LM Studio) — it keeps three `llms.Model` instances (card content, picture rating, health check) since langchaingo only accepts a structured-output `ResponseFormat` at client-construction time, not per call. `cfg.Token`/`cfg.BaseUrl`/`cfg.Model` from `config.yaml`'s `llm` section are wired straight into the client. Images are sent as `data:` URL `llms.ImageURLContent` parts for `RatePicture` — `llms.BinaryContent` is not usable against a plain OpenAI-compatible endpoint since langchaingo serializes it as `{"type":"binary",...}`, which the OpenAI chat completions wire format doesn't understand.
- **`duckduckgo`**: `Image` drives a headless browser to DuckDuckGo Images, resolves each thumbnail to its original image url, and downloads the full-size image via `downloader.File`.
- **`downloader`**: plain HTTP GET with a custom User-Agent (needed for sources like Wikipedia); derives the file extension from the response `Content-Type`.
- **`anki`**: owns the in-memory `map[deckname]*genanki.Deck`, assigns deterministic note/deck IDs by hashing name strings (FNV-1a seeded RNG) so re-runs produce stable IDs, and renders the fixed field/template/CSS set defined in `anki/const.go` into the final package.
- **`browser`**: thin wrapper launching a Playwright-controlled Firefox instance, shared by `extractor.Cambridge` and `duckduckgo.Image`.

## Conventions

- Interfaces are typically named for their package concept (e.g. `card.Creator`, `llm.Provider`, `anki.Service`) with an unexported `impl*` struct implementing them, constructed via `New*` functions — this is what enables mocking.
- Every file defining an interface has a `//go:generate mockgen -source <file> -destination mock/<file>_mock.go` directive; mocks live in a `mock/` subpackage next to the source. Run `go generate ./...` after adding/changing interface methods, not by hand-editing the mocks.
- Concurrent fan-out/fan-in with per-branch error cancellation (via a shared cancellable `context.Context`) is the recurring pattern for combining multiple slow I/O sources (Cambridge, LLM, DuckDuckGo Images) — follow that pattern rather than sequential calls when adding new content sources.
- `models` holds shared plain data types (`Flashcard`, `CardContent`, `File`, `SubjectType`) used across package boundaries — avoid introducing parallel per-package structs for the same concepts.
