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

	"pelagica-studios/internal/db"
	"pelagica-studios/internal/entity"
	"pelagica-studios/internal/paths"
	"pelagica-studios/internal/tmdb"
)

// LimitEnvVar controls how many not-yet-processed entities a run fetches,
// combined across all types. -1 (the default when unset) processes every
// remaining entity.
const LimitEnvVar = "COMPANY_LIMIT"

// A systemic problem (bad auth, network outage) shows up as an unbroken
// streak of failures; abort instead of grinding through the rest of a
// quarter-million-entry list at the rate limit before anyone notices.
const maxConsecutiveFailures = 10

// refreshCycleDays controls how often an already-stored entity is
// re-fetched, spreading the work evenly across daily runs so every row is
// refreshed comfortably inside TMDB's 6 month cache limit.
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

// Process fetches full details for every id in idsFiles that isn't already
// in the sqlite database, refreshes a rolling batch of the least recently
// fetched existing rows across all types, and removes rows TMDB no longer
// returns data for. idsFiles maps each entity type to the path of its daily
// id export. It returns the number of entities newly fetched this run.
func Process(ctx context.Context, idsFiles map[entity.Type]string) (processed int, err error) {
	if err := os.MkdirAll(paths.DataDir, 0o755); err != nil {
		return 0, err
	}
	dbPath := filepath.Join(paths.DataDir, "companies.db")
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

	limit := processLimit()
	client := tmdb.NewClient()
	consecutiveFailures := 0
	totalStored := 0
	startTime := time.Now()

	for _, entityType := range []entity.Type{entity.TypeProductionCompany, entity.TypeTVNetwork} {
		idsFilePath, ok := idsFiles[entityType]
		if !ok {
			continue
		}

		ids, err := readIDs(idsFilePath)
		if err != nil {
			return processed, err
		}

		existingIDs, err := db.ExistingIDs(database, entityType)
		if err != nil {
			return processed, err
		}
		totalStored += len(existingIDs)

		for _, id := range ids {
			if limit != -1 && processed >= limit {
				break
			}
			if _, ok := existingIDs[id]; ok {
				continue
			}

			details, fetchErr := entity.Fetch(ctx, client, entityType, id)
			if fetchErr != nil {
				if errors.Is(fetchErr, context.Canceled) || errors.Is(fetchErr, context.DeadlineExceeded) {
					return processed, nil
				}
				fmt.Fprintf(os.Stderr, "failed to fetch %s %d: %v\n", entityType, id, fetchErr)
				consecutiveFailures++
				if consecutiveFailures >= maxConsecutiveFailures {
					return processed, fmt.Errorf("aborting after %d consecutive failures, last error: %w", consecutiveFailures, fetchErr)
				}
				continue
			}
			consecutiveFailures = 0

			if storeErr := db.UpsertCompany(database, *details); storeErr != nil {
				fmt.Fprintf(os.Stderr, "failed to store %s %d: %v\n", entityType, id, storeErr)
				continue
			}

			processed++
			totalStored++
			rate := float64(processed) / time.Since(startTime).Seconds()
			fmt.Printf("added  %-40.40s  type=%-19s  id=%8d  total=%9d/%-9d  rate=%6.2f/s\n",
				details.Name, entityType, details.ID, totalStored, len(ids), rate)
		}
	}

	staleKeys, err := db.StaleCompanyKeys(database, staleRefreshBatchSize(totalStored))
	if err != nil {
		return processed, err
	}

	removed := 0
	for _, key := range staleKeys {
		details, fetchErr := entity.Fetch(ctx, client, key.Type, key.ID)
		if fetchErr != nil {
			if errors.Is(fetchErr, context.Canceled) || errors.Is(fetchErr, context.DeadlineExceeded) {
				return processed, nil
			}

			var statusErr *tmdb.StatusError
			if errors.As(fetchErr, &statusErr) && statusErr.StatusCode == http.StatusNotFound {
				if delErr := db.DeleteCompany(database, key); delErr != nil {
					fmt.Fprintf(os.Stderr, "failed to remove missing %s %d: %v\n", key.Type, key.ID, delErr)
					continue
				}
				removed++
				totalStored--
				consecutiveFailures = 0
				fmt.Printf("removed no-longer-listed %s id=%d\n", key.Type, key.ID)
				continue
			}

			fmt.Fprintf(os.Stderr, "failed to refresh %s %d: %v\n", key.Type, key.ID, fetchErr)
			consecutiveFailures++
			if consecutiveFailures >= maxConsecutiveFailures {
				return processed, fmt.Errorf("aborting after %d consecutive failures, last error: %w", consecutiveFailures, fetchErr)
			}
			continue
		}
		consecutiveFailures = 0

		if storeErr := db.UpsertCompany(database, *details); storeErr != nil {
			fmt.Fprintf(os.Stderr, "failed to store refreshed %s %d: %v\n", key.Type, key.ID, storeErr)
			continue
		}
		fmt.Printf("refreshed  %-40.40s  type=%-19s  id=%8d\n", details.Name, key.Type, key.ID)
	}
	if len(staleKeys) > 0 {
		fmt.Printf("refresh pass: checked %d, removed %d\n", len(staleKeys), removed)
	}

	return processed, nil
}

func writeJSONExport(database *sql.DB) error {
	all, err := db.AllCompanies(database)
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(all, "", "  ")
	if err != nil {
		return err
	}

	outputPath := filepath.Join(paths.DataDir, "companies.json")
	return os.WriteFile(outputPath, data, 0o644)
}
