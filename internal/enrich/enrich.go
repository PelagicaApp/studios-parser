package enrich

import (
	"bufio"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"pelagica-studios/internal/company"
	"pelagica-studios/internal/db"
	"pelagica-studios/internal/paths"
	"pelagica-studios/internal/tmdb"
)

// LimitEnvVar controls how many not-yet-processed companies a run fetches.
// -1 (the default when unset) processes every remaining company.
const LimitEnvVar = "PRODUCTION_COMPANY_LIMIT"

// A systemic problem (bad auth, network outage) shows up as an unbroken
// streak of failures; abort instead of grinding through the rest of a
// 250k-entry list at the rate limit before anyone notices.
const maxConsecutiveFailures = 10

// refreshCycleDays controls how often an already-stored company is
// re-fetched, spreading the work evenly across daily runs so every row is
// refreshed comfortably inside TMDBs 6 month cache limit.
const refreshCycleDays = 150

func staleRefreshBatchSize(totalRows int) int {
	if totalRows <= 0 {
		return 0
	}
	return (totalRows + refreshCycleDays - 1) / refreshCycleDays
}

func processLimit() int {
	value, ok := os.LookupEnv(LimitEnvVar)
	if !ok {
		return -1
	}
	limit, err := strconv.Atoi(value)
	if err != nil {
		return -1
	}
	return limit
}

type idEntry struct {
	ID int64 `json:"id"`
}

func readIDs(idsFilePath string) ([]int64, error) {
	file, err := os.Open(idsFilePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var ids []int64
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Bytes()
		if len(line) == 0 {
			continue
		}
		var entry idEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, err
		}
		ids = append(ids, entry.ID)
	}
	return ids, scanner.Err()
}

// ProcessProductionCompanies fetches full details for every id in idsFilePath
// that isn't already in the sqlite database, then rewrites the sqlite
// database's contents out to a matching JSON file. It returns the number of
// companies newly fetched this run.
func ProcessProductionCompanies(ctx context.Context, idsFilePath string) (processed int, err error) {
	ids, err := readIDs(idsFilePath)
	if err != nil {
		return 0, err
	}

	if err := os.MkdirAll(paths.DataDir, 0o755); err != nil {
		return 0, err
	}
	dbPath := filepath.Join(paths.DataDir, "production_companies.db")
	database, err := db.OpenDatabase(dbPath)
	if err != nil {
		return 0, err
	}
	defer database.Close()

	defer func() {
		if writeErr := writeJSONExport(database); writeErr != nil && err == nil {
			err = writeErr
		}
	}()

	existingIDs, err := db.ExistingProductionCompanyIDs(database)
	if err != nil {
		return 0, err
	}

	limit := processLimit()
	client := tmdb.NewClient()
	consecutiveFailures := 0
	totalStored := len(existingIDs)
	startTime := time.Now()

	for _, id := range ids {
		if limit != -1 && processed >= limit {
			break
		}
		if _, ok := existingIDs[id]; ok {
			continue
		}

		details, fetchErr := company.Fetch(ctx, client, id)
		if fetchErr != nil {
			if errors.Is(fetchErr, context.Canceled) || errors.Is(fetchErr, context.DeadlineExceeded) {
				return processed, nil
			}
			fmt.Fprintf(os.Stderr, "failed to fetch company %d: %v\n", id, fetchErr)
			consecutiveFailures++
			if consecutiveFailures >= maxConsecutiveFailures {
				return processed, fmt.Errorf("aborting after %d consecutive failures, last error: %w", consecutiveFailures, fetchErr)
			}
			continue
		}
		consecutiveFailures = 0

		if storeErr := db.UpsertProductionCompany(database, *details); storeErr != nil {
			fmt.Fprintf(os.Stderr, "failed to store company %d: %v\n", id, storeErr)
			continue
		}

		processed++
		totalStored++
		rate := float64(processed) / time.Since(startTime).Seconds()
		fmt.Printf("added  %-40.40s  id=%8d  total=%9d/%-9d  rate=%6.2f/s\n",
			details.Name, details.ID, totalStored, len(ids), rate)
	}

	staleIDs, err := db.StaleProductionCompanyIDs(database, staleRefreshBatchSize(totalStored))
	if err != nil {
		return processed, err
	}

	removed := 0
	for _, id := range staleIDs {
		details, fetchErr := company.Fetch(ctx, client, id)
		if fetchErr != nil {
			if errors.Is(fetchErr, context.Canceled) || errors.Is(fetchErr, context.DeadlineExceeded) {
				return processed, nil
			}

			var statusErr *tmdb.StatusError
			if errors.As(fetchErr, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
				if delErr := db.DeleteProductionCompany(database, id); delErr != nil {
					fmt.Fprintf(os.Stderr, "failed to remove missing company %d: %v\n", id, delErr)
					continue
				}
				removed++
				totalStored--
				consecutiveFailures = 0
				fmt.Printf("removed no-longer-listed company id=%d\n", id)
				continue
			}

			fmt.Fprintf(os.Stderr, "failed to refresh company %d: %v\n", id, fetchErr)
			consecutiveFailures++
			if consecutiveFailures >= maxConsecutiveFailures {
				return processed, fmt.Errorf("aborting after %d consecutive failures, last error: %w", consecutiveFailures, fetchErr)
			}
			continue
		}
		consecutiveFailures = 0

		if storeErr := db.UpsertProductionCompany(database, *details); storeErr != nil {
			fmt.Fprintf(os.Stderr, "failed to store refreshed company %d: %v\n", id, storeErr)
			continue
		}
		fmt.Printf("refreshed  %-40.40s  id=%8d\n", details.Name, details.ID)
	}
	if len(staleIDs) > 0 {
		fmt.Printf("refresh pass: checked %d, removed %d\n", len(staleIDs), removed)
	}

	return processed, nil
}

func writeJSONExport(database *sql.DB) error {
	all, err := db.AllProductionCompanies(database)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}

	outputPath := filepath.Join(paths.DataDir, "production_companies.json")
	return os.WriteFile(outputPath, data, 0o644)
}
