## Feature: TUI for app

### Context

The app today is a one-shot CLI (`main.go`): it reads `./word.json`, launches a visible Playwright Firefox, builds the full dependency graph inline, and runs the pipeline once to completion, exiting. There's no way to pick a deck interactively, generate a single word ad hoc, tweak `config.yaml` without hand-editing it, or verify the LLM server is reachable before a long run. The goal is a TUI with two frames: **Settings** (edit `config.yaml`, with a "Test Connection" button for the LLM server) and **Main** (generate from a file or a typed word, choose a deck, start generation with live progress).

Along the way, this also fixes a real bug: `main.go` currently *always* constructs `fetcher.NewOnlyAICardContent`, ignoring `cfg.LLM.OnlyAI` — so Cambridge Dictionary is never actually consulted today even when `only-ai: false`. Centralizing dependency-graph construction (needed anyway so both the legacy path and the TUI's "Start" button build the same graph) is the natural place to restore the documented branching.

Decisions already confirmed: **Bubble Tea + Bubbles + Lipgloss** (plus `huh` for the Settings form — justified below) for the TUI library; **Settings covers the full `config.yaml`** with a Save action that writes the file; **deck picker is a dropdown of real AnkiConnect deck names** (but still allows typing a new one); **progress is structured** (counter + per-subject status), fed by a custom `slog.Handler` rather than changes to service/card business logic.

### Verified facts

- `llm.NewOpenAIClient(logger *slog.Logger, cfg *config.LLMConfig) (Client, error)` — `llm/openai.go:19`. Builds 3 `langchaingo/openai` models; errors are bounded (bad base-url etc.), good for a Test Connection call.
- `extractor.NewCambridge(logger *slog.Logger, browser playwright.Browser, fileDownloader downloader.File) Cambridge` — `extractor/cambridge.go:56`.
- `models.Flashcard`/`models.File` etc. unchanged; progress UI only needs subject strings, which `service.AnkiCardCreator`'s own logs already carry.
- AnkiConnect's `deckNames` action (no params, returns `[]string`) fits the existing `anki.Client.Invoke(ctx, action, params, result)` convention already used for `createDeck`/`findNotes`/`addNote`.
- Logger group names are **sibling** groups off one root logger (`anki-card-creator`, `flashcard-creator`, `with-dictionary-card-content`, `only-ai-card-content`, `anki-service`, `openAI`, etc.) — each constructor calls `.WithGroup(...)` once on the same root `*slog.Logger` passed in from `main.go`. This matters for the custom handler's group-matching.
- `config.yaml`/`config-example.yaml` both have `debuggin:` (typo) instead of `debugging:` — currently silently ignored by `yaml.Unmarshal`, so `cfg.Debugging` is always `false`. `config.Write` naturally emits the correct key going forward; harmless side-fix.
- Repo convention: flat top-level packages, `New*` constructors returning an exported interface backed by an unexported `impl*` struct, mocks via `go:generate mockgen` in a sibling `mock/` package. No `cmd/` directory exists.
- No `bubbletea`/`bubbles`/`lipgloss`/`huh` in `go.sum` — all net-new dependencies.

### Why `huh` for Settings specifically

`huh.Form` binds fields directly to pointers of plain Go types and keeps them live-updated as the user types, with no submit step needed to read values back out. That's exactly what "Test Connection must work from currently-typed values, not just saved ones" needs. It embeds as a `tea.Model` (`form.Update`/`form.View`), composing cleanly into the app's own Bubble Tea model. Hand-rolling ~11 fields across 4 groups with raw `bubbles/textinput`s would be meaningfully more code for no benefit. Everywhere else (menu, file path, subjects/tags entry, deck picker, progress list, raw log) plain `bubbles` components (`textinput`, `textarea`, `list`, `viewport`, `spinner`) are enough.

### New package: `pipeline/`

Centralizes the object-graph construction both the legacy path and the TUI's "Start" action need, fixing the `OnlyAI` branching bug in the process.

- **`pipeline/build.go`**: `func Build(ctx context.Context, cfg config.Config, logger *slog.Logger, headless bool) (*service.AnkiCardCreator, func() error, error)`
  - `browser.LaunchFirefox(playwright.BrowserTypeLaunchOptions{Headless: playwright.Bool(headless)})` → `downloader.NewFile` → `duckduckgo.NewImage` → `tts.NewGoogleSpeech` → `anki.NewService(ctx, logger, cfg.AnkiConnect)` (already validates AnkiConnect connectivity via `ensureNoteType` — no separate Anki "test connection" needed anywhere) → `llm.NewOpenAIClient(logger, cfg.LLM)` → `llm.NewProvider(llmc, 1)`.
  - Branch on `cfg.LLM.OnlyAI`: `true` → `fetcher.NewOnlyAICardContent(...)`; `false` → build `extractor.NewCambridge(logger, br, fileDownloader)` then `fetcher.NewWithDictionaryCardContent(...)`.
  - `card.NewFlashcardCreator(logger, cardContent)` → `service.NewAnkiCardCreator(cfg, logger, ankiService, cardCreator)`.
  - Returned cleanup closes the browser; called on any construction error too (mirrors today's `defer br.Close()`, centralized).

### Config package additions

- `config/config.go`: add `func Write(pathToFile string, cfg *Config) error` (`yaml.Marshal` + `os.WriteFile(..., 0644)`), symmetric with `Read`.
- `config/config_test.go` (new): round-trip test — `Write` a populated `Config`, `Read` it back, assert deep-equal (deref the `LLM *LLMConfig` pointer).

### Anki package addition

- `anki/service.go`: add `ListDecks(ctx context.Context) ([]string, error)` to the `Service` interface; implement via `s.withTimeout` + `s.client.Invoke(ctx, "deckNames", nil, &names)`, matching the existing `ensureDeck`/`storeMedia` pattern.
- Run `go generate ./...` to regenerate `anki/mock/service_mock.go` (do not hand-edit).
- `anki/service_test.go`: add a `ListDecks` case using the existing `newTestService(t)` + mocked `Client.Invoke` pattern.

### New package: `tui/`

- **`app.go`** — `func Run(cfg *config.Config, cfgPath string, logger *slog.Logger) error`. Root `App` model: current screen enum (menu/settings/file-mode/word-mode/generate), shared `*config.Config`, `cfgPath`, sub-models per screen, and `runCtx`/`runCancel` for an in-flight generation. Global keys: `ctrl+c` quits when idle; while generating, `esc` (and `ctrl+c` as a safety net) cancels the run via `runCancel()` instead of killing the app outright, so `cleanup()` still runs. Help bar reads "esc: cancel · q: quit after" during a run.
- **`styles.go`** — shared `lipgloss.Style`s (title, help bar, pending/running/done/failed colors) reused across screens.
- **`menu.go`** — 4-item menu: Generate from file / Generate word(s) / Settings / Quit.
- **`settings.go`** — `huh.Form` bound to a working-copy struct (not the live config, so Esc discards unsaved edits). Groups: General (`Debugging`, `QueueSize`), LLM (`OnlyAI`, `Seed`, `BaseUrl`, `Token`, `Model`, `Timeout` as duration string), Picture (`CountSearches`, `MinimalRating`, `Ignore`), AnkiConnect (`BaseUrl`, `Token`, `Timeout`). `ctrl+s` → `config.Write(cfgPath, &cfg)`, on success copies into `app.cfg` + shows "Saved". `ctrl+t` → builds a throwaway `config.LLMConfig` from current form values (bounded by the form's `Timeout` or a 10s default), fires a `tea.Cmd` doing `llm.NewOpenAIClient` + `HealthCheck`, non-blocking, spinner while in flight, green/red result line after.
- **`filemode.go`** — `textinput` defaulting to `./word.json`; `Enter` → `tea.Cmd` calling `extractor.NewTargetExtractor(logger).GetFromFile`; error shown inline (stay on screen); success → generate screen's confirm step.
- **`wordmode.go`** — deck picker: `bubbles/list` with filtering, items from `anki.Service.ListDecks` (fetched via `tea.Cmd` on screen entry using a short-lived `anki.Service` built from current `cfg.AnkiConnect`); a synthetic `+ Create "<filter text>"` item lets typing an unseen deck name commit it directly (on `ListDecks` failure, degrade to free-text entry rather than blocking). Subjects: `textarea`, split on newlines/commas, trimmed, empties dropped. Tags: `textinput`, comma-separated, `nil` when empty (matches `Tags *[]string` optional semantics). Submit builds `[]extractor.TargetInfo{{DeckName, Tags, Subjects}}` directly in memory — no extractor call needed for this mode.
- **`generate.go`** — confirm → running → finished sub-states.
  - confirm: summary + Enter to start / Esc to go back, before any browser/LLM/API calls fire.
  - running: `runCtx/runCancel` created; `tea.Cmd` runs `pipeline.Build(runCtx, *app.cfg, progressLogger, true /* headless */)` → `svc.Create(runCtx, &targets)` → always `cleanup()` → returns `runFinishedMsg{Err}`. Never blocks Update.
  - State: `subjectsOrder`, `subjectStatus map[string]status{pending|running|done|failed}`, `subjectErr`, `total`, `current`, plus a bounded (~2000 line) `viewport` ring buffer for raw log lines.
  - Messages (from the custom handler via `program.Send`): `subjectsDetectedMsg`, `subjectStartMsg`, `subjectDoneMsg`, `subjectFailedMsg`, `rawLogMsg` (every record, matched or not — Debug-level sub-step detail included), `runFinishedMsg` (authoritative completion/failure signal from the `tea.Cmd` itself; distinguishes user-cancel via `errors.Is(err, context.Canceled)` from a real failure).
  - finished: any key → back to menu.
- **`loghandler.go`** — custom `slog.Handler`:
  - A shared `sink{program *tea.Program}` so every `WithGroup`/`WithAttrs` clone (one per package constructor) funnels into the one live program. Construction order: build the handler with `sink.program` unset → `logger := slog.New(handler)` → build `App`/`tea.Program` with that logger already wired in → set `handler.sink.program = p` → `p.Run()`. `Program.Send` is safe from any goroutine once `*Program` exists.
  - `Enabled` always true at Debug+ (matches today's `slog.SetLogLoggerLevel(slog.LevelDebug)`); display-level filtering happens in the view.
  - `Handle`: always sends a formatted `rawLogMsg` for every record. If the handle chain's first group is `"anki-card-creator"`, additionally pattern-match the six exact message strings service/anki_card_creator.go emits (`"detected subjects"`, `"creating flashcard"`, `"creating flashcard is done"`, `"failed to create flashcard"`, `"creating flashcards is done"`, `"received error during create flashcard"`) against carried+record attrs (`subject`, `count`, `current`, `total`, `err`) to emit the structured messages above. The last of these six isn't given its own message type — the wrapping `tea.Cmd` already surfaces that exact error as `runFinishedMsg.Err`, avoiding a second, possibly-racy source of truth.
- **`messages.go`** — all `tea.Msg` struct definitions in one place: `switchScreenMsg`, `targetsLoadedMsg`, `decksLoadedMsg`, `testConnectionResultMsg`, `settingsSavedMsg`, `subjectsDetectedMsg`, `subjectStartMsg`, `subjectDoneMsg`, `subjectFailedMsg`, `runFinishedMsg`, `rawLogMsg`.

### `main.go` changes

- Add `flag.Bool("tui", false, "launch the interactive terminal UI")`.
- No flag (default): keep today's zero-config one-shot behavior for scripting/cron compatibility — still reads `./word.json`, runs once, exits — but replace the inline, permanently-`OnlyAI` wiring with `pipeline.Build(ctx, *cfg, logger, false /* headless, matches today's visible-Firefox behavior */)`. This both shrinks `main.go` and fixes the branching bug for this path too.
- `-tui`: call `tui.Run(cfg, "./config.yaml", logger)` instead.
- `./word.json` stays hardcoded for the legacy path (out of scope — the TUI's file-mode screen is the actual fix for that limitation).

### Dependency additions

`go get github.com/charmbracelet/bubbletea@latest github.com/charmbracelet/bubbles@latest github.com/charmbracelet/lipgloss@latest github.com/charmbracelet/huh@latest`

### Critical files

- `main.go` — flag branch, replace inline wiring with `pipeline.Build`
- `config/config.go` — add `Write`
- `anki/service.go` — add `ListDecks`
- `service/anki_card_creator.go` — log message strings the custom handler matches against (read-only reference, not modified)
- `card/fetcher/content.go` — constructor signatures `pipeline.Build` branches between

### Verification

1. `go generate ./...` then `go build ./...` — confirms mock regen and new packages compile.
2. `go test -v -race -count=5 ./...` stays green; new `config/config_test.go` and `ListDecks` test pass.
3. Settings: `-tui` → Settings → confirm fields pre-populate from `config.yaml`; edit a field, `ctrl+s`, diff the file on disk (including the `debugging:` key now spelled correctly); restart, confirm persistence.
4. Test Connection: point `BaseUrl` at an unreachable address, `ctrl+t`, confirm the UI stays responsive and fails within the bounded timeout using *typed* (unsaved) values; repeat against a real reachable endpoint for the success path.
5. From-file mode against a real local Anki + AnkiConnect: confirm live per-subject status + counter, and that cards actually land in Anki.
6. Word mode: deck picker lists real `deckNames`; typing a new name shows the "+ Create" option and works; multiple newline/comma-separated subjects land in the right deck.
7. Cancel mid-run: `Esc` partway through a multi-subject run; confirm prompt stop, a distinct "cancelled" (vs "failed") terminal state, and no partial note left for the in-flight subject.
8. Raw log pane: confirm Debug-level lines from non-`anki-card-creator` groups still appear even though they don't drive the structured counter.
9. Headless check: `-tui` never pops a visible Firefox window; no-flag run still does (unchanged).
10. Regression/bugfix check: no-flag run with `only-ai: false` — confirm (via debug logs) Cambridge is now actually being hit, where before it never was regardless of this setting.

## Feature: local tts generation for pronunciation