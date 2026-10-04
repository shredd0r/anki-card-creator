package imagesearch

//go:generate mockgen -source image.go -destination mock/image_mock.go

import (
	"context"

	"github.com/shredd0r/anki-card-creator/models"
)

// Image - interface for access to make request to searching image by query
type Image interface {
	// Request - create struct where you can sort through images on page
	Request(ctx context.Context, searchQuery string) (Result, error)
}

type Result interface {
	// Get - return image from opened web page
	Get(ctx context.Context, filenameWithoutType string, numOfPicture uint) (*models.File, error)
}
