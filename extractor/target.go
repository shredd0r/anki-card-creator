package extractor

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
)

type TargetInfo struct {
	DeckName string    `json:"deckname"`
	Tags     *[]string `json:"tags,omitempty"`
	Subjects []string  `json:"subjects"`
}

type Target interface {
	GetFromFile(ctx context.Context, pathToFile string) (*[]TargetInfo, error)
	GetFromDir(ctx context.Context, pathToDir string) (*[]TargetInfo, error)
}

type implTargetExtractor struct {
	logger *slog.Logger
}

func NewTargetExtractor(logger *slog.Logger) Target {
	return &implTargetExtractor{
		logger: logger.WithGroup("target-extractor"),
	}
}

func (e *implTargetExtractor) GetFromFile(ctx context.Context, pathToFile string) (*[]TargetInfo, error) {
	bytesOfReadedFile, err := os.ReadFile(pathToFile)
	if err != nil {
		e.logger.Error("failed read file with targets")
		return nil, err
	}

	var listOfTargets []TargetInfo
	err = json.Unmarshal(bytesOfReadedFile, &listOfTargets)
	if err != nil {
		e.logger.Error("failed unmarshal file bytes to json")
		return nil, err
	}

	return &listOfTargets, nil
}

func (e *implTargetExtractor) GetFromDir(ctx context.Context, pathToDir string) (*[]TargetInfo, error) {
	listOfTargets := []TargetInfo{}

	innerFiles, err := os.ReadDir(pathToDir)
	if err != nil {
		e.logger.Error("failed read directory")
		return nil, err
	}
	for _, innerFile := range innerFiles {
		if innerFile.IsDir() {
			targets, err := e.GetFromDir(ctx, fmt.Sprintf("%s/%s", pathToDir, innerFile.Name()))
			if err != nil {
				return nil, err
			}
			listOfTargets = append(listOfTargets, *targets...)
		} else {
			if strings.Contains(innerFile.Name(), ".json") {
				targets, err := e.GetFromFile(ctx, fmt.Sprintf("%s/%s", pathToDir, innerFile.Name()))
				if err != nil {
					return nil, err
				}
				listOfTargets = append(listOfTargets, *targets...)
			}
		}
	}
	return &listOfTargets, nil
}
