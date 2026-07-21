package db

import (
	"database/sql"
	"time"

	"pelagica-studios/internal/company"
)

func ensureProductionCompaniesSchema(database *sql.DB) error {
	_, err := database.Exec(`
		CREATE TABLE IF NOT EXISTS production_companies (
			id INTEGER PRIMARY KEY,
			name TEXT NOT NULL,
			headquarters TEXT,
			homepage TEXT,
			description TEXT,
			origin_country TEXT,
			logo_file_path TEXT,
			logo_aspect_ratio REAL,
			logo_height INTEGER,
			logo_id TEXT,
			logo_file_type TEXT,
			logo_width INTEGER,
			logo_vote_count INTEGER,
			logo_vote_average REAL,
			fetched_at INTEGER NOT NULL DEFAULT 0
		)
	`)
	if err != nil {
		return err
	}
	return ensureFetchedAtColumn(database)
}

// ensureFetchedAtColumn adds fetched_at to databases created before staleness
// tracking existed. Backfilled rows default to 0, which sorts first for
// refresh, so they naturally get a real timestamp the next time they're due.
func ensureFetchedAtColumn(database *sql.DB) error {
	rows, err := database.Query(`PRAGMA table_info(production_companies)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid, notNull, pk int
		var name, colType string
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &colType, &notNull, &dflt, &pk); err != nil {
			return err
		}
		if name == "fetched_at" {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = database.Exec(`ALTER TABLE production_companies ADD COLUMN fetched_at INTEGER NOT NULL DEFAULT 0`)
	return err
}

func ExistingProductionCompanyIDs(database *sql.DB) (map[int64]struct{}, error) {
	rows, err := database.Query(`SELECT id FROM production_companies`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	ids := make(map[int64]struct{})
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids[id] = struct{}{}
	}
	return ids, rows.Err()
}

func UpsertProductionCompany(database *sql.DB, details company.Details) error {
	_, err := database.Exec(`
		INSERT OR REPLACE INTO production_companies (
			id, name, headquarters, homepage, description, origin_country,
			logo_file_path, logo_aspect_ratio, logo_height, logo_id, logo_file_type,
			logo_width, logo_vote_count, logo_vote_average, fetched_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		details.ID, details.Name, details.Headquarters, details.Homepage, details.Description, details.OriginCountry,
		details.LogoFilePath, details.LogoAspectRatio, details.LogoHeight, details.LogoID, details.LogoFileType,
		details.LogoWidth, details.LogoVoteCount, details.LogoVoteAverage, time.Now().Unix(),
	)
	return err
}

// StaleProductionCompanyIDs returns up to limit ids ordered by least
// recently fetched, for the rolling refresh pass in enrich.
func StaleProductionCompanyIDs(database *sql.DB, limit int) ([]int64, error) {
	if limit <= 0 {
		return nil, nil
	}

	rows, err := database.Query(`SELECT id FROM production_companies ORDER BY fetched_at ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func DeleteProductionCompany(database *sql.DB, id int64) error {
	_, err := database.Exec(`DELETE FROM production_companies WHERE id = ?`, id)
	return err
}

func AllProductionCompanies(database *sql.DB) ([]company.Details, error) {
	rows, err := database.Query(`
		SELECT id, name, headquarters, homepage, description, origin_country,
		       logo_file_path, logo_aspect_ratio, logo_height, logo_id, logo_file_type,
		       logo_width, logo_vote_count, logo_vote_average
		FROM production_companies
		ORDER BY id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []company.Details
	for rows.Next() {
		var d company.Details
		if err := rows.Scan(
			&d.ID, &d.Name, &d.Headquarters, &d.Homepage, &d.Description, &d.OriginCountry,
			&d.LogoFilePath, &d.LogoAspectRatio, &d.LogoHeight, &d.LogoID, &d.LogoFileType,
			&d.LogoWidth, &d.LogoVoteCount, &d.LogoVoteAverage,
		); err != nil {
			return nil, err
		}
		results = append(results, d)
	}
	return results, rows.Err()
}
