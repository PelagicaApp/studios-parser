package productioncompanies

import (
	"compress/gzip"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"pelagica-studios/internal/paths"
)

const exportsBaseURL = "https://files.tmdb.org/p/exports"
const maxDaysBack = 3

func formatExportDate(date time.Time) string {
	return date.UTC().Format("01_02_2006")
}

func DownloadProductionCompanyIds() (string, error) {
	for daysBack := 0; daysBack < maxDaysBack; daysBack++ {
		dateStr := formatExportDate(time.Now().AddDate(0, 0, -daysBack))
		url := fmt.Sprintf("%s/production_company_ids_%s.json.gz", exportsBaseURL, dateStr)

		response, err := http.Get(url)
		if err != nil {
			return "", err
		}

		if response.StatusCode != http.StatusOK {
			response.Body.Close()
			continue
		}

		outputPath, err := saveUnpackedExport(response.Body, dateStr)
		response.Body.Close()
		if err != nil {
			return "", err
		}
		return outputPath, nil
	}

	return "", fmt.Errorf("no production company export found in the last %d days", maxDaysBack)
}

func saveUnpackedExport(gzipped io.Reader, dateStr string) (string, error) {
	gzipReader, err := gzip.NewReader(gzipped)
	if err != nil {
		return "", err
	}
	defer gzipReader.Close()

	if err := os.MkdirAll(paths.TempDir, 0o755); err != nil {
		return "", err
	}

	outputPath := filepath.Join(paths.TempDir, fmt.Sprintf("production_company_ids_%s.json", dateStr))
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

func DeleteProductionCompanyIdsFile(filePath string) {
	if err := os.Remove(filePath); err != nil {
		fmt.Printf("Failed to delete file %s: %v\n", filePath, err)
	}
}
