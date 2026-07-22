package exports

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"pelagica-studios/internal/entity"
	"pelagica-studios/internal/paths"
)

const exportsBaseURL = "https://files.tmdb.org/p/exports"
const maxDaysBack = 3

func formatExportDate(date time.Time) string {
	return date.UTC().Format("01_02_2006")
}

// exportName returns TMDB's daily export file prefix for a Type, e.g.
// "production_company_ids" or "tv_network_ids".
func exportName(entityType entity.Type) string {
	switch entityType {
	case entity.TypeTVNetwork:
		return "tv_network_ids"
	default:
		return "production_company_ids"
	}
}

// DownloadIDs fetches TMDB's most recent daily id export for entityType,
// unpacks it, and returns the path to the unpacked JSON lines file.
func DownloadIDs(entityType entity.Type) (string, error) {
	name := exportName(entityType)

	for daysBack := 0; daysBack < maxDaysBack; daysBack++ {
		dateStr := formatExportDate(time.Now().AddDate(0, 0, -daysBack))
		url := fmt.Sprintf("%s/%s_%s.json.gz", exportsBaseURL, name, dateStr)

		response, err := http.Get(url)
		if err != nil {
			return "", err
		}

		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			continue
		}

		outputPath, err := saveUnpackedExport(response.Body, name, dateStr)
		response.Body.Close()
		if err != nil {
			return "", err
		}
		return outputPath, nil
	}

	return "", fmt.Errorf("no %s export found in the last %d days", name, maxDaysBack)
}

func saveUnpackedExport(gzipped io.Reader, name, dateStr string) (string, error) {
	gzipReader, err := gzip.NewReader(gzipped)
	if err != nil {
		return "", err
	}
	defer gzipReader.Close()

	if err := os.MkdirAll(paths.TempDir, 0o755); err != nil {
		return "", err
	}

	outputPath := filepath.Join(paths.TempDir, fmt.Sprintf("%s_%s.json", name, dateStr))
	outputFile, err := os.Create(outputPath)
	if err != nil {
		return "", err
	}
	defer outputFile.Close()

	if _, err := io.Copy(outputFile, gzipReader); err != nil {
		return "", err
	}

	return outputPath, nil
}

func DeleteIDsFile(filePath string) {
	if err := os.Remove(filePath); err != nil {
		fmt.Printf("Failed to delete file %s: %v\n", filePath, err)
	}
}
