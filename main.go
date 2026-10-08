package main

import (
	"context"
	"log/slog"

	"github.com/mxschmitt/playwright-go"
	"github.com/shredd0r/anki-card-creator/anki"
	"github.com/shredd0r/anki-card-creator/browser"
	"github.com/shredd0r/anki-card-creator/card"
	"github.com/shredd0r/anki-card-creator/card/fetcher"
	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/downloader"
	"github.com/shredd0r/anki-card-creator/duckduckgo"
	"github.com/shredd0r/anki-card-creator/extractor"
	"github.com/shredd0r/anki-card-creator/llm"
	"github.com/shredd0r/anki-card-creator/service"
	"github.com/shredd0r/anki-card-creator/tts"
)

func main() {
	ctx := context.Background()
	slog.SetLogLoggerLevel(slog.LevelDebug)
	logger := slog.Default()
	cfg, err := config.Read("./config.yaml")
	if err != nil {
		logger.Error(err.Error())
		return
	}

	br, err := browser.LaunchFirefox(
		playwright.BrowserTypeLaunchOptions{
			Headless: playwright.Bool(false),
		},
	)
	if err != nil {
		logger.Error("failed to launch browser", slog.Any("err", err))
		return
	}
	defer br.Close()

	fileDownloader := downloader.NewFile(logger)
	imageSearchProvider := duckduckgo.NewImage(logger, br, fileDownloader)
	speech := tts.NewGoogleSpeech(logger, tts.LanguageEnglishUK)

	as, err := anki.NewService(ctx, logger, cfg.AnkiConnect)
	if err != nil {
		logger.Error("failed to initialize anki connect service", slog.Any("err", err))
		return
	}

	// llmc := llm.NewMockClient(logger)
	llmc, err := llm.NewOpenAIClient(logger, cfg.LLM)
	if err != nil {
		logger.Error("failed to create llm client", slog.Any("err", err))
		return
	}
	llmp := llm.NewProvider(llmc, 1)

	cardContent := fetcher.NewOnlyAICardContent(cfg.Picture, logger, imageSearchProvider, llmp, speech)
	cc := card.NewFlashcardCreator(logger, cardContent)
	te := extractor.NewTargetExtractor(logger)
	s := service.NewAnkiCardCreator(*cfg, logger, as, cc)
	targets, err := te.GetFromFile(ctx, "./json-files/computer_science_it.json")
	if err != nil {
		logger.Error(err.Error())
		return
	}

	err = s.Create(ctx, targets)
	if err != nil {
		logger.Error(err.Error())
		return
	}
}
