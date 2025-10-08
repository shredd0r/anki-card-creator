package extractors

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/shredd0r/anki-card-creator/models"
)

type TargetExtractor interface {
	GetTargetsFromFile(ctx context.Context, pathToFile string) (*[]models.Target, error)
	GetTargetsFromDir(ctx context.Context, pathToDir string) (*[]models.Target, error)
}

type implTargetExtractor struct {
	logger *slog.Logger
}

func NewTargetExtractor(logger *slog.Logger) TargetExtractor {
	return &implTargetExtractor{
		logger: logger.WithGroup("target-extractor"),
	}
}

func (e *implTargetExtractor) GetTargetsFromFile(ctx context.Context, pathToFile string) (*[]models.Target, error) {
	bytesOfReadedFile, err := os.ReadFile(pathToFile)
	if err != nil {
		e.logger.Error("failed read file with targets")
		return nil, err
	}

	var listOfTargets []models.Target
	err = json.Unmarshal(bytesOfReadedFile, &listOfTargets)
	if err != nil {
		e.logger.Error("failed unmarshal file bytes to json")
		return nil, err
	}

	return &listOfTargets, nil
}

func (e *implTargetExtractor) GetTargetsFromDir(ctx context.Context, pathToDir string) (*[]models.Target, error) {
	listOfTargets := []models.Target{}

	innerFiles, err := os.ReadDir(pathToDir)
	if err != nil {
		e.logger.Error("failed read directory")
		return nil, err
	}
	for _, innerFile := range innerFiles {
		if innerFile.IsDir() {
			targets, err := e.GetTargetsFromDir(ctx, fmt.Sprintf("%s/%s", pathToDir, innerFile.Name()))
			if err != nil {
				return nil, err
			}
			listOfTargets = append(listOfTargets, *targets...)
		} else {
			if strings.Contains(innerFile.Name(), ".json") {
				targets, err := e.GetTargetsFromFile(ctx, fmt.Sprintf("%s/%s", pathToDir, innerFile.Name()))
				if err != nil {
					return nil, err
				}
				listOfTargets = append(listOfTargets, *targets...)
			}
		}
	}
	return &listOfTargets, nil
}
