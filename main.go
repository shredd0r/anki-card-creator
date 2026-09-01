package main

import (
	"context"
	"log/slog"

	"github.com/shredd0r/anki-card-creator/anki"
	"github.com/shredd0r/anki-card-creator/card"
	"github.com/shredd0r/anki-card-creator/card/fetcher"
	"github.com/shredd0r/anki-card-creator/config"
	"github.com/shredd0r/anki-card-creator/extractor"
	"github.com/shredd0r/anki-card-creator/llm"
	"github.com/shredd0r/anki-card-creator/service"
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

	as := anki.NewService(logger)
	llmc := llm.NewOpenAIClient(logger, cfg.LLM)
	llmp := llm.NewProvider(llmc)
	ccf := fetcher.NewCardContentFactory(cfg.Picture, logger, nil, nil, llmp)
	cc := card.NewFlashcardCreator(cfg.Picture, logger, llmp, nil, ccf)
	te := extractor.NewTargetExtractor(logger)
	s := service.NewAnkiCardCreator(*cfg, logger, as, cc)
	targets, err := te.GetFromFile(ctx, "./test.json")
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
