package db

import (
	"database/sql"
	"time"

	"pelagica-studios/internal/entity"
)

const companiesTableDDL = `
	CREATE TABLE IF NOT EXISTS companies (
		id INTEGER NOT NULL,
		type TEXT NOT NULL,
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
		fetched_at INTEGER NOT NULL DEFAULT 0,
		PRIMARY KEY (id, type)
	)
`

func ensureCompaniesSchema(database *sql.DB) error {
	if err := migrateLegacyProductionCompaniesTable(database); err != nil {
		return err
	}

	_, err := database.Exec(companiesTableDDL)
	return err
}

// migrateLegacyProductionCompaniesTable upgrades legacy database
func migrateLegacyProductionCompaniesTable(database *sql.DB) error {
	hasLegacy, err := tableExists(database, "production_companies")
	if err != nil {
		return err
	}
	if !hasLegacy {
		return nil
	}
	hasCurrent, err := tableExists(database, "companies")
	if err != nil {
		return err
	}
	if hasCurrent {
		return nil
	}

	if err := ensureColumn(database, "production_companies", "fetched_at", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}

	tx, err := database.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(companiesTableDDL); err != nil {
		return err
	}

	_, err = tx.Exec(`
		INSERT INTO companies (
			id, type, name, headquarters, homepage, description, origin_country,
			logo_file_path, logo_aspect_ratio, logo_height, logo_id, logo_file_type,
			logo_width, logo_vote_count, logo_vote_average, fetched_at
		)
		SELECT id, ?, name, headquarters, homepage, description, origin_country,
			logo_file_path, logo_aspect_ratio, logo_height, logo_id, logo_file_type,
			logo_width, logo_vote_count, logo_vote_average, fetched_at
		FROM production_companies
	`, string(entity.TypeProductionCompany))
	if err != nil {
		return err
	}

	if _, err := tx.Exec(`DROP TABLE production_companies`); err != nil {
		return err
	}

	return tx.Commit()
}

func tableExists(database *sql.DB, name string) (bool, error) {
	var found string
	err := database.QueryRow(`SELECT name FROM sqlite_master WHERE type = 'table' AND name = ?`, name).Scan(&found)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return true, nil
}

// ensureColumn adds column to table if it isn't already present.
func ensureColumn(database *sql.DB, table, column, definition string) error {
	rows, err := database.Query(`PRAGMA table_info(` + table + `)`)
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
		if name == column {
			return rows.Err()
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = database.Exec(`ALTER TABLE ` + table + ` ADD COLUMN ` + column + ` ` + definition)
	return err
}

func ExistingIDs(database *sql.DB, entityType entity.Type) (map[int64]struct{}, error) {
	rows, err := database.Query(`SELECT id FROM companies WHERE type = ?`, string(entityType))
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

func UpsertCompany(database *sql.DB, details entity.Details) error {
	_, err := database.Exec(`
		INSERT OR REPLACE INTO companies (
			id, type, name, headquarters, homepage, description, origin_country,
			logo_file_path, logo_aspect_ratio, logo_height, logo_id, logo_file_type,
			logo_width, logo_vote_count, logo_vote_average, fetched_at
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		details.ID, string(details.Type), details.Name, details.Headquarters, details.Homepage, details.Description, details.OriginCountry,
		details.LogoFilePath, details.LogoAspectRatio, details.LogoHeight, details.LogoID, details.LogoFileType,
		details.LogoWidth, details.LogoVoteCount, details.LogoVoteAverage, time.Now().Unix(),
	)
	return err
}

// CompanyKey identifies a row in the companies table. TMDB company ids and
// network ids can collide, so id alone is not enough to look up a row.
type CompanyKey struct {
	ID   int64
	Type entity.Type
}

// StaleCompanyKeys returns up to limit keys ordered by least recently
// fetched, across both entity types, for the rolling refresh pass in enrich.
func StaleCompanyKeys(database *sql.DB, limit int) ([]CompanyKey, error) {
	if limit <= 0 {
		return nil, nil
	}

	rows, err := database.Query(`SELECT id, type FROM companies ORDER BY fetched_at ASC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []CompanyKey
	for rows.Next() {
		var key CompanyKey
		var entityType string
		if err := rows.Scan(&key.ID, &entityType); err != nil {
			return nil, err
		}
		key.Type = entity.Type(entityType)
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

func DeleteCompany(database *sql.DB, key CompanyKey) error {
	_, err := database.Exec(`DELETE FROM companies WHERE id = ? AND type = ?`, key.ID, string(key.Type))
	return err
}

func AllCompanies(database *sql.DB) ([]entity.Details, error) {
	rows, err := database.Query(`
		SELECT id, type, name, headquarters, homepage, description, origin_country,
		       logo_file_path, logo_aspect_ratio, logo_height, logo_id, logo_file_type,
		       logo_width, logo_vote_count, logo_vote_average
		FROM companies
		ORDER BY type, id
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []entity.Details
	for rows.Next() {
		var d entity.Details
		var entityType string
		if err := rows.Scan(
			&d.ID, &entityType, &d.Name, &d.Headquarters, &d.Homepage, &d.Description, &d.OriginCountry,
			&d.LogoFilePath, &d.LogoAspectRatio, &d.LogoHeight, &d.LogoID, &d.LogoFileType,
			&d.LogoWidth, &d.LogoVoteCount, &d.LogoVoteAverage,
		); err != nil {
			return nil, err
		}
		d.Type = entity.Type(entityType)
		results = append(results, d)
	}
	return results, rows.Err()
}
