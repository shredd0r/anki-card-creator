# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## What this is

A Go CLI tool that generates Anki flashcards for learning English words/phrases and writes them directly into a running Anki instance via the **AnkiConnect** addon (`http://127.0.0.1:8765` by default) — there is no `.apkg` file output; Anki desktop with AnkiConnect installed is a hard runtime dependency for every run.

For each subject it assembles a flashcard from multiple sources, running them concurrently. Content assembly runs in one of two scenarios for the whole run, selected by `config.yaml`'s `llm.only-ai` flag:
- **`with dictionary`** (`llm.only-ai: false`, default): tries **Cambridge Dictionary** first for paraphrase/transcription/examples; whatever Cambridge doesn't have falls back to the **LLM**, field by field. Synonyms always come from the LLM (Cambridge has no synonym data). Pronunciation prefers Cambridge's audio, falling back to **TTS**. This also transparently covers phrase-shaped subjects, since Cambridge rejects anything but a single word outright — that just becomes "Cambridge failed, use AI for everything."
- **`only ai`** (`llm.only-ai: true`): paraphrase/transcription/examples/synonyms all from the LLM, pronunciation via TTS, no Cambridge involved at all.

Both scenarios search **DuckDuckGo Images** for a candidate picture (unless `picture.ignore: true`), rated by the LLM until one clears `picture.min-rating`.

The Anki note type (fields/Mustache templates/CSS in `anki/const.go`, based on [Maple Anki Template](https://github.com/liuzikai/Maple-Anki-Template)) is auto-created/self-healed in the user's Anki collection on startup via AnkiConnect rather than bundled into a package file.

## Commands

```bash
go generate ./...             # regenerate all mockgen mocks (run after changing any interface)
go test -v -race -count=5 ./...   # full test suite (as run by `make test`); note count=5 — tests re-run 5x to catch flakiness/races
go test -run TestName ./card/fetcher/...   # run a single test
make build-macos-arm          # build for current platform (also: build-macos-intel, build-linux, build-linux-arm, build-windows)
```

There is no lint target configured (the `lint:` Makefile target is currently empty). Version is injected at build time via `-ldflags` into `internal/version.Version` (see `Makefile`); it defaults to `"development"` in unbuilt/test binaries.

The app reads connection/model settings from `./config.yaml` (see `config-example.yaml`). Note: `main.go` currently hardcodes the input target path to `./word.json` rather than reading `os.Args`.

## Architecture

**Pipeline**: `extractor` (reads target JSON) → `service.AnkiCardCreator` (fan-out orchestration) → `card.Creator` (per-subject assembly, single scenario for the whole run) → `anki.Service` (writes to Anki via AnkiConnect).

- **`extractor`**: `Target` reads one JSON file or recursively walks a directory of `*.json` files into `[]TargetInfo{DeckName, Tags, Subjects}`. `Cambridge` scrapes dictionary.cambridge.org via Playwright for word-only content (transcription, definitions, examples, pronunciation audio) — `GetCard` is resilient per-field: if one volume fails to scrape, it's left nil/empty rather than failing the whole call, so callers can fall back to AI for just that field. Only "subject not found" / non-word subject / navigation failure remain hard errors.
- **`service.AnkiCardCreator`**: top-level orchestrator. Fans out one goroutine per subject (bounded by `config.queue-size`, default 10) into a bounded channel of `*models.Flashcard`; a separate goroutine drains that channel into `anki.Service`. The producer closes the flashcard channel once every subject is done, which is the "all done" signal — `context` cancellation is reserved for actual errors (either a subject failing to build, or an `anki.Service.AddFlashcard` call failing), which still fast-cancels everything else in flight.
- **`card.Creator`**: for one subject, runs three fetches concurrently against the single configured `fetcher.CardContent` (card content, picture, pronunciation), cancelling siblings on first error. `utils.GetSubjectType` (space/hyphen heuristic) is only used to tag the resulting `Flashcard.SubjectType` — it no longer selects which content-fetching logic runs.
- **`card/fetcher`**: `CardContent` has two implementations, chosen once in `main.go` based on `config.LLMConfig.OnlyAI`:
  - `withDictionaryCardContent` — fetches Cambridge (`extractor.Cambridge`) and the LLM (`llm.Provider`) concurrently via independent goroutines (not `errgroup`, so a Cambridge error can't cancel the in-flight LLM call); merges per-field, preferring Cambridge when present and falling back to the LLM's result otherwise.
  - `onlyAICardContent` — LLM-only for every text field.
  - Both embed `mediaFetcher`, which implements `GetPicture` (searches/rates DuckDuckGo Images up to `picture.count-searches` attempts, keeping the first whose LLM rating ≥ `picture.min-rating`; skipped entirely if `picture.ignore: true`) and `GetPronunciation` (TTS via `tts.Speech`) identically for both scenarios — this embedding-for-shared-behavior pattern is the way to add logic common to multiple `CardContent` implementations without duplicating it.
- **`llm`**: `Provider` wraps a `Client` and JSON-(un)marshals prompts/responses (`GeneratedCardContent`, `RatedPictureContent`); prompt/schema text lives in `llm/system.go` / `llm/schema.go`. `Client` has two implementations: `implClient` (via `NewClient`, in `client.go`) wraps three already-built `llms.Model` instances (card content, picture rating, health check — three, not one, because [`langchaingo`](https://github.com/tmc/langchaingo) only accepts a structured-output `ResponseFormat` at model-construction time, not per call) and does the actual `GenerateContent` calls; `NewOpenAIClient` (in `openai.go`) is the OpenAI-compatible-specific constructor that builds those three models via `langchaingo`'s `llms/openai` backend from `cfg.Token`/`cfg.BaseUrl`/`cfg.Model`/`cfg.Seed`, then hands them to `NewClient`. `NewMockClient` returns canned, subject-aware content with no real LLM server involved, useful for local runs/tests — `main.go` currently uses it by default (the real `NewOpenAIClient` call is present but commented out). Images are sent as `data:` URL `llms.ImageURLContent` parts for `RatePicture`.
- **`duckduckgo`**: `Image` drives a headless browser to DuckDuckGo Images, resolves each thumbnail to its original image url, and downloads the full-size image via `downloader.File`. Its CSS selectors are a best-effort guess at DuckDuckGo's markup, not verified against a live page — check these first if picture search stops finding results.
- **`tts`**: `Speech` interface (`Create(ctx, text) (io.Reader, error)`); `NewGoogleSpeech` is a cloud implementation (a direct HTTP call to Google Translate's TTS endpoint, no local library). Deliberately just an interface + one implementation so a future local synthesizer (e.g. espeak-ng, see `FEATURE.md`) can be swapped in without touching callers.
- **`downloader`**: plain HTTP GET with a custom User-Agent (needed for sources like Wikipedia); derives the file extension from the response `Content-Type`.
- **`anki`**: `Service.AddFlashcard` talks to a running AnkiConnect addon (`config.AnkiConnectConfig`, default `http://127.0.0.1:8765`) over plain HTTP+JSON (`ankiconnect_client.go` — stdlib only, no SDK; built with `Transport.DisableKeepAlives: true` since AnkiConnect's server doesn't reliably support persistent connections). On construction it ensures the Anki note type matches `anki/const.go`'s fields/templates/CSS (`note_type.go`, auto-creating or self-healing it — so manual Anki-side edits to the template get overwritten on the next run). Each `AddFlashcard` call ensures the deck exists, uploads any media, then finds-and-updates or adds the note by `Subject` match within the deck, so re-running the tool on the same deck/subject updates the existing card rather than duplicating it. There is no more `.apkg`/package concept.
- **`browser`**: thin wrapper launching a Playwright-controlled Firefox instance, shared by `extractor.Cambridge` and `duckduckgo.Image`.

## Conventions

- Interfaces are typically named for their package concept (e.g. `card.Creator`, `llm.Provider`, `anki.Service`) with an unexported `impl*` struct implementing them, constructed via `New*` functions — this is what enables mocking.
- Every file defining an interface has a `//go:generate mockgen -source <file> -destination mock/<file>_mock.go` directive; mocks live in a `mock/` subpackage next to the source. Run `go generate ./...` after adding/changing interface methods, not by hand-editing the mocks.
- Concurrent fan-out/fan-in with per-branch error cancellation (via a shared cancellable `context.Context`) is the recurring pattern for combining multiple slow I/O sources (Cambridge, LLM, DuckDuckGo Images) — follow that pattern rather than sequential calls when adding new content sources. When one branch's failure should *not* cancel the others (e.g. Cambridge failing shouldn't cancel an in-flight LLM call needed for fallback), use independent goroutines with their own result/error variables instead of `errgroup.WithContext`, which cancels all branches on the first error.
- Shared behavior across multiple implementations of the same interface (e.g. `mediaFetcher`'s `GetPicture`/`GetPronunciation`, used by both `card/fetcher.CardContent` implementations) goes in an unexported struct the implementations embed, rather than being duplicated or pulled into the caller.
- `models` holds shared plain data types (`Flashcard`, `CardContent`, `File`, `SubjectType`) used across package boundaries — avoid introducing parallel per-package structs for the same concepts.
- Global run-wide behavior switches (which scenario/implementation to use for the whole run) belong in `config.yaml`, not in per-subject heuristics — `config.LLMConfig.OnlyAI` and `config.PictureConfig.Ignore` are both wired this way.
