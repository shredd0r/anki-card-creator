package downloader

import (
	"bytes"
	"fmt"
	"log/slog"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

const (
	url_to_jpg_file                     = "https://images.squarespace-cdn.com/content/v1/534ad50ae4b04a5110f5ae72/f0cd330d-46de-4f2e-864a-31e2741cf6ae/Salle+Ovale%2C+BNF+Richelieu%2C+Henri+Labrouste%2C+1936.jpg"
	url_to_png_file                     = "https://png.pngtree.com/png-clipart/20220215/ourmid/pngtree-blue-sky-and-white-clouds-png-image_4391818.png"
	url_to_svg_file                     = "https://upload.wikimedia.org/wikipedia/commons/c/ca/Subaru_logo_%28transparent%29.svg"
	url_to_wikipedia_file               = "https://upload.wikimedia.org/wikipedia/commons/thumb/b/b3/Wikipedia-logo-v2-en.svg/1045px-Wikipedia-logo-v2-en.svg.png"
	format_path_to_directory_with_files = "./../assets/%s"
)

type testCase struct {
	Name             string
	UrlToFile        string
	ExpectedFileName string
	ExpectedFileType string
}

func TestPositiveDownloadFiles(t *testing.T) {
	testCases := []testCase{
		{
			Name:             "Downloaded file was jpg",
			UrlToFile:        url_to_jpg_file,
			ExpectedFileName: "jpg-file.jpg",
			ExpectedFileType: "image/jpeg",
		},
		{
			Name:             "Downloaded file was png",
			UrlToFile:        url_to_png_file,
			ExpectedFileName: "png-file.png",
			ExpectedFileType: "image/png",
		},
		{
			Name:             "Downlaoded file was svg",
			UrlToFile:        url_to_svg_file,
			ExpectedFileName: "svg-file.svg",
			ExpectedFileType: "image/svg+xml",
		},
		{
			Name:             "Download from file from Wikipedia",
			UrlToFile:        url_to_wikipedia_file,
			ExpectedFileName: "wikipedia-file.png",
			ExpectedFileType: "image/png",
		},
	}

	logger := slog.Default()
	fileDownloader := NewFile(logger)

	for _, tc := range testCases {
		t.Run(tc.ExpectedFileName, func(t *testing.T) { positiveDownloadTestCase(t, &tc, fileDownloader) })

	}
}

func positiveDownloadTestCase(t *testing.T, tc *testCase, fileDownloader File) {
	expectedFileBytes, err := os.ReadFile(fmt.Sprintf(format_path_to_directory_with_files, tc.ExpectedFileName))
	assert.NoError(t, err)
	assert.NotNil(t, expectedFileBytes)

	downloadedFile, err := fileDownloader.Download(t.Context(), tc.UrlToFile)
	assert.NoError(t, err)
	assert.NotNil(t, downloadedFile)

	actualFileBytes := downloadedFile.Content

	assert.Equal(t, tc.ExpectedFileType, downloadedFile.MIMEType, "type of files not equal")
	assert.Equalf(t, len(expectedFileBytes), len(actualFileBytes), "size of expected and actual files not equal")
	assert.Truef(t, bytes.Equal(expectedFileBytes, actualFileBytes), "expected file not equal with actual file")
}
